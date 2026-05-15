package json

import (
	"bytes"
	"encoding/base64"
	stdjson "encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/olimci/roundtrip/internal/list"
	"github.com/olimci/roundtrip/internal/util/reflectutil"
)

func Marshal(v any) ([]byte, error) {
	var b bytes.Buffer
	err := NewEncoder(&b).Encode(v)
	return b.Bytes(), err
}

func MarshalMeta(m *Meta) ([]byte, error) {
	var b bytes.Buffer
	err := NewEncoder(&b).EncodeMeta(m)
	return b.Bytes(), err
}

func NewEncoder(w io.Writer) *Encoder {
	return &Encoder{w: w}
}

type Encoder struct {
	w      io.Writer
	Indent string
	depth  int
	tokens *list.List[token]
}

func (e *Encoder) EncodeMeta(m *Meta) error {
	_, err := e.w.Write(m.SST.Root.Bytes())
	return err
}

func (e *Encoder) Encode(v any) error {
	n, err := e.encode(v)
	if err != nil {
		return err
	}
	_, err = e.w.Write(n.Bytes())
	return err
}

func (e *Encoder) encode(v any) (*node, error) {
	e.tokens = list.New[token]()
	return e.value(reflect.ValueOf(v), 0)
}

func (e *Encoder) value(v reflect.Value, depth int) (*node, error) {
	if !v.IsValid() {
		return e.scalar(NodeTypeNull, TokenIdentifier, "null"), nil
	}

	if reflectutil.Nilable(v.Kind()) && v.IsNil() {
		return e.scalar(NodeTypeNull, TokenIdentifier, "null"), nil
	}

	if m, ok := marshaler(v); ok {
		b, err := m.MarshalJSON()
		if err != nil {
			return nil, err
		}
		d := NewDecoder(bytes.NewReader(b))
		meta, err := d.DecodeMeta()
		if err != nil {
			return nil, err
		}
		if meta.SST.Tokens.Tail.Value.Type == TokenEOF {
			meta.SST.Tokens.Remove(meta.SST.Tokens.Tail)
		}
		e.tokens.PushBackList(meta.SST.Tokens)
		return meta.SST.Root, nil
	}
	if m, ok := textMarshaler(v); ok {
		b, err := m.MarshalText()
		if err != nil {
			return nil, err
		}
		return e.scalar(NodeTypeString, TokenString, quoteString(string(b))), nil
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return e.value(v.Elem(), depth)
	case reflect.Bool:
		return e.scalar(NodeTypeBool, TokenIdentifier, strconv.FormatBool(v.Bool())), nil
	case reflect.String:
		return e.scalar(NodeTypeString, TokenString, quoteString(v.String())), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return e.scalar(NodeTypeNumber, TokenNumber, strconv.FormatInt(v.Int(), 10)), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return e.scalar(NodeTypeNumber, TokenNumber, strconv.FormatUint(v.Uint(), 10)), nil
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, fmt.Errorf("cannot encode %v", f)
		}
		b, err := stdjson.Marshal(v.Interface())
		if err != nil {
			return nil, err
		}
		return e.scalar(NodeTypeNumber, TokenNumber, string(b)), nil
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return e.scalar(NodeTypeString, TokenString, quoteString(base64.StdEncoding.EncodeToString(v.Bytes()))), nil
		}
		return e.array(v, depth)
	case reflect.Array:
		return e.array(v, depth)
	case reflect.Map:
		return e.mapValue(v, depth)
	case reflect.Struct:
		return e.structValue(v, depth)
	default:
		return nil, fmt.Errorf("cannot encode %v", v.Type())
	}
}

func (e *Encoder) scalar(nodeType NodeType, tokenType TokenType, literal string) *node {
	elem := e.token(tokenType, literal)
	return &node{Type: nodeType, Start: elem, End: elem}
}

func (e *Encoder) array(v reflect.Value, depth int) (*node, error) {
	start := e.token(TokenLeftBracket, "[")
	n := &node{Type: NodeTypeArray, Start: start}
	for i := range v.Len() {
		if i > 0 {
			e.token(TokenComma, ",")
		}
		e.newline(depth + 1)
		child, err := e.value(v.Index(i), depth+1)
		if err != nil {
			return nil, err
		}
		n.Children = append(n.Children, child)
	}
	if v.Len() > 0 {
		e.newline(depth)
	}
	n.End = e.token(TokenRightBracket, "]")
	return n, nil
}

func (e *Encoder) mapValue(v reflect.Value, depth int) (*node, error) {
	if !mapKeyTypeSupported(v.Type().Key()) {
		return nil, fmt.Errorf("cannot encode %v", v.Type())
	}

	keys := make([]string, 0, v.Len())
	keyValues := map[string]reflect.Value{}
	for _, key := range v.MapKeys() {
		s, ok := mapKeyString(key)
		if !ok {
			return nil, fmt.Errorf("cannot encode %v", v.Type())
		}
		keys = append(keys, s)
		keyValues[s] = key
	}
	sort.Strings(keys)

	start := e.token(TokenLeftBrace, "{")
	n := &node{Type: NodeTypeObject, Start: start}
	for i, key := range keys {
		if i > 0 {
			e.token(TokenComma, ",")
		}
		e.newline(depth + 1)
		keyNode := e.scalar(NodeTypeString, TokenString, quoteString(key))
		e.token(TokenColon, ":")
		e.fieldSpace()
		valueNode, err := e.value(v.MapIndex(keyValues[key]), depth+1)
		if err != nil {
			return nil, err
		}
		appendObjectField(n, keyNode, valueNode)
	}
	if v.Len() > 0 {
		e.newline(depth)
	}
	n.End = e.token(TokenRightBrace, "}")
	return n, nil
}

func (e *Encoder) structValue(v reflect.Value, depth int) (*node, error) {
	fields := encodedStructFields(v)
	start := e.token(TokenLeftBrace, "{")
	n := &node{Type: NodeTypeObject, Start: start}
	for i, field := range fields {
		if i > 0 {
			e.token(TokenComma, ",")
		}
		e.newline(depth + 1)
		keyNode := e.scalar(NodeTypeString, TokenString, quoteString(field.Name))
		e.token(TokenColon, ":")
		e.fieldSpace()
		fieldValue := field.Value
		if field.Quoted && quoteValue(fieldValue) {
			encoded, err := encodedValueString(fieldValue)
			if err != nil {
				return nil, err
			}
			fieldValue = reflect.ValueOf(encoded)
		}
		valueNode, err := e.value(fieldValue, depth+1)
		if err != nil {
			return nil, err
		}
		appendObjectField(n, keyNode, valueNode)
	}
	if len(fields) > 0 {
		e.newline(depth)
	}
	n.End = e.token(TokenRightBrace, "}")
	return n, nil
}

func encodedValueString(v reflect.Value) (string, error) {
	enc := &Encoder{}
	n, err := enc.encode(v.Interface())
	if err != nil {
		return "", err
	}
	return string(n.Bytes()), nil
}

func quoteString(s string) string {
	b, _ := stdjson.Marshal(s)
	return string(b)
}

func (e *Encoder) newline(depth int) {
	if e.Indent == "" {
		return
	}
	e.token(TokenNewline, "\n")
	e.token(TokenWhitespace, strings.Repeat(e.Indent, e.depth+depth))
}

func (e *Encoder) fieldSpace() {
	if e.Indent != "" {
		e.token(TokenWhitespace, " ")
	}
}

func (e *Encoder) token(typ TokenType, literal string) *list.Elem[token] {
	return e.tokens.PushBack(token{Type: typ, Literal: literal})
}

func encode(v any, indent string, depth int) (*node, *list.List[token], error) {
	e := &Encoder{Indent: indent, depth: depth}
	n, err := e.encode(v)
	if err != nil {
		return nil, nil, err
	}
	return n, e.tokens, nil
}

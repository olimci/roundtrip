package json

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"
	"math"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

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

func NewJSONCEncoder(w io.Writer) *Encoder {
	return NewEncoder(w)
}

func NewJSON5Encoder(w io.Writer) *Encoder {
	e := NewEncoder(w)
	e.SetAllowIdentifierKeys(true)
	e.SetAllowJSON5Numbers(true)
	return e
}

type Encoder struct {
	w                   io.Writer
	Indent              string
	Prefix              string
	escapeHTMLDisabled  bool
	AllowIdentifierKeys bool
	AllowJSON5Numbers   bool
	depth               int
	tokens              *list.List[token]
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

func (e *Encoder) SetIndent(prefix, indent string) {
	e.Prefix = prefix
	e.Indent = indent
}

func (e *Encoder) SetEscapeHTML(on bool) {
	e.escapeHTMLDisabled = !on
}

func (e *Encoder) SetAllowIdentifierKeys(on bool) {
	e.AllowIdentifierKeys = on
}

func (e *Encoder) SetAllowJSON5Numbers(on bool) {
	e.AllowJSON5Numbers = on
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

	if v.Kind() == reflect.Pointer && v.Type().Elem() == numberType && e.AllowJSON5Numbers {
		return e.value(v.Elem(), depth)
	}

	if v.Type() == numberType && e.AllowJSON5Numbers {
		n := v.Interface().(Number)
		if !validJSON5Number(string(n)) {
			return nil, fmt.Errorf("json: invalid number literal %q", n)
		}
		return e.scalar(NodeTypeNumber, TokenNumber, string(n)), nil
	}

	if m, ok := marshaler(v); ok {
		b, err := m.MarshalJSON()
		if err != nil {
			return nil, err
		}
		if !e.escapeHTMLDisabled {
			var escaped bytes.Buffer
			HTMLEscape(&escaped, b)
			b = escaped.Bytes()
		}
		d := NewDecoder(bytes.NewReader(b))
		if e.AllowJSON5Numbers || e.AllowIdentifierKeys {
			d = NewJSON5Decoder(bytes.NewReader(b))
		}
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
		return e.scalar(NodeTypeString, TokenString, e.quoteString(string(b))), nil
	}

	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		return e.value(v.Elem(), depth)
	case reflect.Bool:
		return e.scalar(NodeTypeBool, TokenIdentifier, strconv.FormatBool(v.Bool())), nil
	case reflect.String:
		return e.scalar(NodeTypeString, TokenString, e.quoteString(v.String())), nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return e.scalar(NodeTypeNumber, TokenNumber, strconv.FormatInt(v.Int(), 10)), nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return e.scalar(NodeTypeNumber, TokenNumber, strconv.FormatUint(v.Uint(), 10)), nil
	case reflect.Float32, reflect.Float64:
		f := v.Float()
		if math.IsInf(f, 0) || math.IsNaN(f) {
			return nil, fmt.Errorf("cannot encode %v", f)
		}
		return e.scalar(NodeTypeNumber, TokenNumber, formatFloat(f, int(v.Type().Bits()))), nil
	case reflect.Slice:
		if v.Type().Elem().Kind() == reflect.Uint8 {
			return e.scalar(NodeTypeString, TokenString, e.quoteString(base64.StdEncoding.EncodeToString(v.Bytes()))), nil
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
		keyNode := e.objectKey(key)
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
		keyNode := e.objectKey(field.Name)
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

func (e *Encoder) objectKey(s string) *node {
	if e.AllowIdentifierKeys && isJSON5Identifier(s) {
		return e.scalar(NodeTypeString, TokenIdentifier, s)
	}
	return e.scalar(NodeTypeString, TokenString, e.quoteString(s))
}

func encodedValueString(v reflect.Value) (string, error) {
	enc := &Encoder{}
	n, err := enc.encode(v.Interface())
	if err != nil {
		return "", err
	}
	return string(n.Bytes()), nil
}

func (e *Encoder) quoteString(s string) string {
	return string(appendQuotedString(nil, s, !e.escapeHTMLDisabled))
}

func quoteString(s string) string {
	return string(appendQuotedString(nil, s, true))
}

func appendQuotedString(dst []byte, src string, escapeHTML bool) []byte {
	const hex = "0123456789abcdef"

	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(src); {
		if b := src[i]; b < utf8.RuneSelf {
			if jsonStringByteSafe(b, escapeHTML) {
				i++
				continue
			}
			dst = append(dst, src[start:i]...)
			switch b {
			case '\\', '"':
				dst = append(dst, '\\', b)
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			default:
				dst = append(dst, '\\', 'u', '0', '0', hex[b>>4], hex[b&0xf])
			}
			i++
			start = i
			continue
		}

		r, size := utf8.DecodeRuneInString(src[i:])
		if r == utf8.RuneError && size == 1 {
			dst = append(dst, src[start:i]...)
			dst = append(dst, `\ufffd`...)
			i += size
			start = i
			continue
		}
		if r == '\u2028' || r == '\u2029' {
			dst = append(dst, src[start:i]...)
			dst = append(dst, '\\', 'u', '2', '0', '2', hex[r&0xf])
			i += size
			start = i
			continue
		}
		i += size
	}
	dst = append(dst, src[start:]...)
	dst = append(dst, '"')
	return dst
}

func jsonStringByteSafe(b byte, escapeHTML bool) bool {
	if b < 0x20 || b == '\\' || b == '"' {
		return false
	}
	if escapeHTML && (b == '<' || b == '>' || b == '&') {
		return false
	}
	return true
}

func formatFloat(f float64, bits int) string {
	abs := math.Abs(f)
	fmt := byte('f')
	if abs != 0 {
		if bits == 64 && (abs < 1e-6 || abs >= 1e21) || bits == 32 && (float32(abs) < 1e-6 || float32(abs) >= 1e21) {
			fmt = 'e'
		}
	}
	b := strconv.AppendFloat(nil, f, fmt, -1, bits)
	if fmt == 'e' {
		n := len(b)
		if n >= 4 && b[n-4] == 'e' && b[n-3] == '-' && b[n-2] == '0' {
			b[n-2] = b[n-1]
			b = b[:n-1]
		}
	}
	return string(b)
}

func (e *Encoder) newline(depth int) {
	if e.Indent == "" {
		return
	}
	e.token(TokenNewline, "\n")
	e.token(TokenWhitespace, e.Prefix+strings.Repeat(e.Indent, e.depth+depth))
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

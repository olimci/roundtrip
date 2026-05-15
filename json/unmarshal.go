package json

import (
	"encoding"
	"encoding/base64"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"github.com/olimci/roundtrip/internal/util/reflectutil"
)

type Unmarshaler interface {
	UnmarshalJSON([]byte) error
}

var unmarshalerType = reflect.TypeFor[Unmarshaler]()
var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

type decodeOptions struct {
	useNumber       bool
	disallowUnknown bool
}

func decodeInto(m *Meta, n *node, v any, opts decodeOptions) error {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() {
		return InvalidUnmarshalError{}
	}
	if rv.Kind() != reflect.Pointer || rv.IsNil() {
		return InvalidUnmarshalError{Type: rv.Type()}
	}
	return decodeValue(m, n, rv.Elem(), opts)
}

func decodeValue(m *Meta, n *node, v reflect.Value, opts decodeOptions) error {
	if !v.CanSet() {
		return fmt.Errorf("cannot set %s", v.Type())
	}

	if u, ok := unmarshaler(v); ok {
		return u.UnmarshalJSON(Node{meta: m, node: n}.Bytes())
	}
	if n.Type == NodeTypeString {
		if u, ok := textUnmarshaler(v); ok {
			s, err := decodeString(m, n)
			if err != nil {
				return err
			}
			return u.UnmarshalText([]byte(s))
		}
	}

	if n.Type == NodeTypeNull {
		if reflectutil.Nilable(v.Kind()) {
			v.SetZero()
		}
		return nil
	}
	if v.Type() == numberType {
		if n.Type != NodeTypeNumber {
			return typeError(n, v.Type())
		}
		v.SetString(n.Start.Value.Literal)
		return nil
	}

	switch v.Kind() {
	case reflect.Pointer:
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		return decodeValue(m, n, v.Elem(), opts)
	case reflect.Interface:
		value, err := decodeAny(m, n, opts)
		if err != nil {
			return err
		}
		v.Set(reflect.ValueOf(value))
		return nil
	case reflect.Bool:
		value, err := strconv.ParseBool(n.Start.Value.Literal)
		if err != nil {
			return err
		}
		v.SetBool(value)
		return nil
	case reflect.String:
		if n.Type != NodeTypeString {
			return typeError(n, v.Type())
		}
		value, err := strconv.Unquote(n.Start.Value.Literal)
		if err != nil {
			return err
		}
		v.SetString(value)
		return nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if n.Type != NodeTypeNumber {
			return typeError(n, v.Type())
		}
		value, err := strconv.ParseInt(n.Start.Value.Literal, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetInt(value)
		return nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		if n.Type != NodeTypeNumber {
			return typeError(n, v.Type())
		}
		value, err := strconv.ParseUint(n.Start.Value.Literal, 10, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetUint(value)
		return nil
	case reflect.Float32, reflect.Float64:
		if n.Type != NodeTypeNumber {
			return typeError(n, v.Type())
		}
		value, err := strconv.ParseFloat(n.Start.Value.Literal, v.Type().Bits())
		if err != nil {
			return err
		}
		v.SetFloat(value)
		return nil
	case reflect.Slice:
		return decodeSlice(m, n, v, opts)
	case reflect.Array:
		return decodeArray(m, n, v, opts)
	case reflect.Map:
		return decodeMap(m, n, v, opts)
	case reflect.Struct:
		return decodeStruct(m, n, v, opts)
	default:
		return fmt.Errorf("cannot decode %v into %v", n.Type, v.Type())
	}
}

func decodeAny(m *Meta, n *node, opts decodeOptions) (any, error) {
	switch n.Type {
	case NodeTypeObject:
		value := make(map[string]any, objectFieldCount(n))
		for keyNode, valueNode := range objectFields(n) {
			key, err := decodeString(m, keyNode)
			if err != nil {
				return nil, err
			}
			child, err := decodeAny(m, valueNode, opts)
			if err != nil {
				return nil, err
			}
			value[key] = child
		}
		return value, nil
	case NodeTypeArray:
		value := make([]any, len(n.Children))
		for i, child := range n.Children {
			item, err := decodeAny(m, child, opts)
			if err != nil {
				return nil, err
			}
			value[i] = item
		}
		return value, nil
	case NodeTypeString:
		return decodeString(m, n)
	case NodeTypeNumber:
		if opts.useNumber {
			return Number(n.Start.Value.Literal), nil
		}
		return strconv.ParseFloat(n.Start.Value.Literal, 64)
	case NodeTypeBool:
		return strconv.ParseBool(n.Start.Value.Literal)
	case NodeTypeNull:
		return nil, nil
	default:
		return nil, fmt.Errorf("cannot decode %v", n.Type)
	}
}

func decodeSlice(m *Meta, n *node, v reflect.Value, opts decodeOptions) error {
	if v.Type().Elem().Kind() == reflect.Uint8 && n.Type == NodeTypeString {
		s, err := decodeString(m, n)
		if err != nil {
			return err
		}
		b, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return err
		}
		v.SetBytes(b)
		return nil
	}
	if n.Type != NodeTypeArray {
		return fmt.Errorf("cannot decode %v into %v", n.Type, v.Type())
	}

	value := reflect.MakeSlice(v.Type(), len(n.Children), len(n.Children))
	for i, child := range n.Children {
		if err := decodeValue(m, child, value.Index(i), opts); err != nil {
			return err
		}
	}
	v.Set(value)
	return nil
}

func decodeArray(m *Meta, n *node, v reflect.Value, opts decodeOptions) error {
	if n.Type != NodeTypeArray {
		return fmt.Errorf("cannot decode %v into %v", n.Type, v.Type())
	}

	for i := range v.Len() {
		v.Index(i).SetZero()
	}
	for i, child := range n.Children[:min(len(n.Children), v.Len())] {
		if err := decodeValue(m, child, v.Index(i), opts); err != nil {
			return err
		}
	}
	return nil
}

func decodeMap(m *Meta, n *node, v reflect.Value, opts decodeOptions) error {
	if n.Type != NodeTypeObject {
		return fmt.Errorf("cannot decode %v into %v", n.Type, v.Type())
	}
	if !mapKeyTypeSupported(v.Type().Key()) {
		return fmt.Errorf("cannot decode object into %v", v.Type())
	}
	if v.IsNil() {
		v.Set(reflect.MakeMapWithSize(v.Type(), objectFieldCount(n)))
	}

	for keyNode, valueNode := range objectFields(n) {
		key, err := decodeString(m, keyNode)
		if err != nil {
			return err
		}

		mapKey, err := decodeMapKey(key, v.Type().Key())
		if err != nil {
			return err
		}

		value := reflect.New(v.Type().Elem()).Elem()
		if err := decodeValue(m, valueNode, value, opts); err != nil {
			return err
		}
		v.SetMapIndex(mapKey, value)
	}
	return nil
}

func decodeStruct(m *Meta, n *node, v reflect.Value, opts decodeOptions) error {
	if n.Type != NodeTypeObject {
		return fmt.Errorf("cannot decode %v into %v", n.Type, v.Type())
	}

	fields := structFieldIndexes(v.Type())
	for keyNode, valueNode := range objectFields(n) {
		key, err := decodeString(m, keyNode)
		if err != nil {
			return err
		}

		field, ok := fieldIndex(fields, key)
		if !ok {
			if opts.disallowUnknown {
				return fmt.Errorf("json: unknown field %q", key)
			}
			continue
		}
		valueMeta := m
		target := v.Field(field.Index)
		if field.Options.Quoted {
			decoded, err := quotedNode(m, valueNode)
			if err != nil {
				return err
			}
			valueNode = decoded.SST.Root
			valueMeta = decoded
		}
		if err := decodeValue(valueMeta, valueNode, target, opts); err != nil {
			return err
		}
	}
	return nil
}

func unmarshaler(v reflect.Value) (Unmarshaler, bool) {
	if v.Kind() == reflect.Pointer && v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	if v.CanInterface() && v.Type().Implements(unmarshalerType) {
		return v.Interface().(Unmarshaler), true
	}
	if v.CanAddr() && reflect.PointerTo(v.Type()).Implements(unmarshalerType) {
		return v.Addr().Interface().(Unmarshaler), true
	}
	return nil, false
}

func textUnmarshaler(v reflect.Value) (encoding.TextUnmarshaler, bool) {
	if v.Kind() == reflect.Pointer && v.IsNil() {
		v.Set(reflect.New(v.Type().Elem()))
	}
	if v.CanInterface() && v.Type().Implements(textUnmarshalerType) {
		return v.Interface().(encoding.TextUnmarshaler), true
	}
	if v.CanAddr() && reflect.PointerTo(v.Type()).Implements(textUnmarshalerType) {
		return v.Addr().Interface().(encoding.TextUnmarshaler), true
	}
	return nil, false
}

func quotedNode(m *Meta, n *node) (*Meta, error) {
	s, err := decodeString(m, n)
	if err != nil {
		return nil, err
	}
	return NewDecoder(strings.NewReader(s)).DecodeMeta()
}

func decodeMapKey(s string, typ reflect.Type) (reflect.Value, error) {
	v := reflect.New(typ).Elem()
	if u, ok := textUnmarshaler(v); ok {
		if err := u.UnmarshalText([]byte(s)); err != nil {
			return reflect.Value{}, err
		}
		return v, nil
	}
	switch typ.Kind() {
	case reflect.String:
		v.SetString(s)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		i, err := strconv.ParseInt(s, 10, typ.Bits())
		if err != nil {
			return reflect.Value{}, err
		}
		v.SetInt(i)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		u, err := strconv.ParseUint(s, 10, typ.Bits())
		if err != nil {
			return reflect.Value{}, err
		}
		v.SetUint(u)
	default:
		return reflect.Value{}, fmt.Errorf("cannot decode object key into %v", typ)
	}
	return v, nil
}

func typeError(n *node, typ reflect.Type) error {
	return &UnmarshalTypeError{Value: strings.ToLower(n.Type.String()), Type: typ, Offset: int64(n.Start.Value.Position.Offset)}
}

func decodeString(m *Meta, n *node) (string, error) {
	if n.Type != NodeTypeString {
		return "", fmt.Errorf("cannot decode %v into string", n.Type)
	}
	return strconv.Unquote(n.Start.Value.Literal)
}

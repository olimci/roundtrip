package json

import (
	"encoding"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

var marshalerType = reflect.TypeFor[Marshaler]()
var textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()

type structField struct {
	Name      string
	Value     reflect.Value
	OmitEmpty bool
	OmitZero  bool
	Quoted    bool
}

type fieldOptions struct {
	OmitEmpty bool
	OmitZero  bool
	Quoted    bool
}

type structFieldIndex struct {
	Index   int
	Options fieldOptions
}

func encodedStructFields(v reflect.Value) []structField {
	fields := []structField{}
	t := v.Type()
	for i := range t.NumField() {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name, opts, ok := parseFieldTag(f)
		if !ok {
			continue
		}
		value := v.Field(i)
		if (opts.OmitEmpty && isEmptyValue(value)) || (opts.OmitZero && value.IsZero()) {
			continue
		}
		fields = append(fields, structField{Name: name, Value: value, OmitEmpty: opts.OmitEmpty, OmitZero: opts.OmitZero, Quoted: opts.Quoted})
	}
	return fields
}

func marshaler(v reflect.Value) (Marshaler, bool) {
	if v.CanInterface() && v.Type().Implements(marshalerType) {
		return v.Interface().(Marshaler), true
	}
	if v.CanAddr() && reflect.PointerTo(v.Type()).Implements(marshalerType) {
		return v.Addr().Interface().(Marshaler), true
	}
	return nil, false
}

func textMarshaler(v reflect.Value) (encoding.TextMarshaler, bool) {
	if v.CanInterface() && v.Type().Implements(textMarshalerType) {
		return v.Interface().(encoding.TextMarshaler), true
	}
	if v.CanAddr() && reflect.PointerTo(v.Type()).Implements(textMarshalerType) {
		return v.Addr().Interface().(encoding.TextMarshaler), true
	}
	return nil, false
}

func parseFieldTag(f reflect.StructField) (string, fieldOptions, bool) {
	name := f.Name
	opts := fieldOptions{}
	if tag := f.Tag.Get("json"); tag != "" {
		tagName, tagOpts, _ := strings.Cut(tag, ",")
		if tagName == "-" {
			return "", opts, false
		}
		if tagName != "" {
			name = tagName
		}
		for opt := range strings.SplitSeq(tagOpts, ",") {
			switch opt {
			case "omitempty":
				opts.OmitEmpty = true
			case "omitzero":
				opts.OmitZero = true
			case "string":
				opts.Quoted = true
			}
		}
	}
	return name, opts, true
}

func isEmptyValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.Array:
		return v.Len() == 0
	case reflect.Map, reflect.Slice, reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Pointer:
		return v.IsNil()
	}
	return false
}

func mapKeyString(v reflect.Value) (string, bool) {
	if m, ok := textMarshaler(v); ok {
		b, err := m.MarshalText()
		return string(b), err == nil
	}
	switch v.Kind() {
	case reflect.String:
		return v.String(), true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(v.Int(), 10), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return strconv.FormatUint(v.Uint(), 10), true
	}
	return "", false
}

func mapKeyTypeSupported(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return true
	}
	return t.Implements(textMarshalerType) || reflect.PointerTo(t).Implements(textMarshalerType)
}

func structFieldIndexes(t reflect.Type) map[string]structFieldIndex {
	fields := map[string]structFieldIndex{}
	for i := range t.NumField() {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}
		name, opts, ok := parseFieldTag(f)
		if !ok {
			continue
		}
		fields[name] = structFieldIndex{Index: i, Options: opts}
	}
	return fields
}

func fieldIndex(fields map[string]structFieldIndex, key string) (structFieldIndex, bool) {
	if index, ok := fields[key]; ok {
		return index, true
	}
	for name, index := range fields {
		if strings.EqualFold(name, key) {
			return index, true
		}
	}
	return structFieldIndex{}, false
}

func quoteValue(v reflect.Value) bool {
	return slices.Contains([]reflect.Kind{
		reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.String,
	}, v.Kind())
}

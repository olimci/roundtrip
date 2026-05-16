package json

import (
	"cmp"
	"encoding"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
)

type Marshaler interface {
	MarshalJSON() ([]byte, error)
}

var marshalerType = reflect.TypeFor[Marshaler]()
var textMarshalerType = reflect.TypeFor[encoding.TextMarshaler]()

type structField struct {
	Name   string
	Value  reflect.Value
	Quoted bool
}

type fieldOptions struct {
	OmitEmpty bool
	OmitZero  bool
	Quoted    bool
}

type structFieldIndex struct {
	Name    string
	Index   []int
	Options fieldOptions
	Tagged  bool
	Type    reflect.Type
	IsZero  func(reflect.Value) bool
}

func encodedStructFields(v reflect.Value) []structField {
	fields := []structField{}
	for _, f := range structFieldIndexes(v.Type()) {
		value, ok := fieldByIndex(v, f.Index, false)
		if !ok {
			continue
		}
		if (f.Options.OmitEmpty && isEmptyValue(value)) || (f.Options.OmitZero && isZeroValue(value, f.IsZero)) {
			continue
		}
		fields = append(fields, structField{Name: f.Name, Value: value, Quoted: f.Options.Quoted})
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

func parseFieldInfo(parent structFieldIndex, f reflect.StructField, i int) (structFieldIndex, bool, bool) {
	if f.Anonymous {
		t := f.Type
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		if !f.IsExported() && t.Kind() != reflect.Struct {
			return structFieldIndex{}, false, false
		}
	} else if !f.IsExported() {
		return structFieldIndex{}, false, false
	}

	tag := f.Tag.Get("json")
	if tag == "-" {
		return structFieldIndex{}, false, false
	}

	name, opts, tagged := strings.Cut(tag, ",")
	if !isValidTag(name) {
		name = ""
	}
	options := fieldOptions{}
	for opt := range strings.SplitSeq(opts, ",") {
		switch opt {
		case "omitempty":
			options.OmitEmpty = true
		case "omitzero":
			options.OmitZero = true
		case "string":
			options.Quoted = true
		}
	}
	tagged = tagged || name != ""

	index := slices.Clone(parent.Index)
	index = append(index, i)

	ft := f.Type
	if ft.Name() == "" && ft.Kind() == reflect.Pointer {
		ft = ft.Elem()
	}
	options.Quoted = options.Quoted && quoteType(ft)

	if name != "" || !f.Anonymous || ft.Kind() != reflect.Struct {
		if name == "" {
			name = f.Name
			tagged = false
		}
		field := structFieldIndex{
			Name:    name,
			Index:   index,
			Options: options,
			Tagged:  tagged,
			Type:    ft,
		}
		if options.OmitZero {
			field.IsZero = zeroFunc(f.Type)
		}
		return field, true, false
	}

	return structFieldIndex{Name: ft.Name(), Index: index, Type: ft}, false, true
}

func isValidTag(s string) bool {
	if s == "" {
		return false
	}
	for _, c := range s {
		if strings.ContainsRune("!#$%&()*+-./:;<=>?@[]^_{|}~ ", c) || unicode.IsLetter(c) || unicode.IsDigit(c) {
			continue
		}
		return false
	}
	return true
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

type isZeroer interface {
	IsZero() bool
}

var isZeroerType = reflect.TypeFor[isZeroer]()

func zeroFunc(t reflect.Type) func(reflect.Value) bool {
	switch {
	case t.Kind() == reflect.Interface && t.Implements(isZeroerType):
		return func(v reflect.Value) bool {
			return v.IsNil() || (v.Elem().Kind() == reflect.Pointer && v.Elem().IsNil()) || v.Interface().(isZeroer).IsZero()
		}
	case t.Kind() == reflect.Pointer && t.Implements(isZeroerType):
		return func(v reflect.Value) bool {
			return v.IsNil() || v.Interface().(isZeroer).IsZero()
		}
	case t.Implements(isZeroerType):
		return func(v reflect.Value) bool {
			return v.Interface().(isZeroer).IsZero()
		}
	case reflect.PointerTo(t).Implements(isZeroerType):
		return func(v reflect.Value) bool {
			if !v.CanAddr() {
				v2 := reflect.New(v.Type()).Elem()
				v2.Set(v)
				v = v2
			}
			return v.Addr().Interface().(isZeroer).IsZero()
		}
	}
	return nil
}

func isZeroValue(v reflect.Value, isZero func(reflect.Value) bool) bool {
	if isZero != nil {
		return isZero(v)
	}
	return v.IsZero()
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

var structFieldCache sync.Map

func structFieldIndexes(t reflect.Type) []structFieldIndex {
	if fields, ok := structFieldCache.Load(t); ok {
		return fields.([]structFieldIndex)
	}

	current := []structFieldIndex{}
	next := []structFieldIndex{{Type: t}}
	var count, nextCount map[reflect.Type]int
	visited := map[reflect.Type]bool{}
	fields := []structFieldIndex{}

	for len(next) > 0 {
		current, next = next, current[:0]
		count, nextCount = nextCount, map[reflect.Type]int{}

		for _, parent := range current {
			if visited[parent.Type] {
				continue
			}
			visited[parent.Type] = true

			for i := range parent.Type.NumField() {
				field, include, explore := parseFieldInfo(parent, parent.Type.Field(i), i)
				if include {
					fields = append(fields, field)
					if count[parent.Type] > 1 {
						fields = append(fields, field)
					}
				}
				if explore {
					nextCount[field.Type]++
					if nextCount[field.Type] == 1 {
						next = append(next, field)
					}
				}
			}
		}
	}

	slices.SortFunc(fields, func(a, b structFieldIndex) int {
		if c := strings.Compare(a.Name, b.Name); c != 0 {
			return c
		}
		if c := cmp.Compare(len(a.Index), len(b.Index)); c != 0 {
			return c
		}
		if a.Tagged != b.Tagged {
			if a.Tagged {
				return -1
			}
			return 1
		}
		return slices.Compare(a.Index, b.Index)
	})

	out := fields[:0]
	for advance, i := 0, 0; i < len(fields); i += advance {
		name := fields[i].Name
		for advance = 1; i+advance < len(fields) && fields[i+advance].Name == name; advance++ {
		}
		if advance == 1 {
			out = append(out, fields[i])
			continue
		}
		if len(fields[i].Index) != len(fields[i+1].Index) || fields[i].Tagged != fields[i+1].Tagged {
			out = append(out, fields[i])
		}
	}
	fields = out
	slices.SortFunc(fields, func(a, b structFieldIndex) int {
		return slices.Compare(a.Index, b.Index)
	})
	actual, _ := structFieldCache.LoadOrStore(t, fields)
	return actual.([]structFieldIndex)
}

func fieldIndex(fields []structFieldIndex, key string) (structFieldIndex, bool) {
	for _, field := range fields {
		if field.Name == key {
			return field, true
		}
	}
	for _, field := range fields {
		if strings.EqualFold(field.Name, key) {
			return field, true
		}
	}
	return structFieldIndex{}, false
}

func fieldByIndex(v reflect.Value, index []int, allocate bool) (reflect.Value, bool) {
	for i, x := range index {
		if v.Kind() == reflect.Pointer {
			if v.IsNil() {
				if !allocate {
					return reflect.Value{}, false
				}
				if !v.CanSet() {
					return reflect.Value{}, false
				}
				v.Set(reflect.New(v.Type().Elem()))
			}
			v = v.Elem()
		}
		v = v.Field(x)
		if i < len(index)-1 && v.Kind() == reflect.Pointer && v.IsNil() && !allocate {
			return reflect.Value{}, false
		}
	}
	return v, true
}

func quoteType(t reflect.Type) bool {
	return slices.Contains([]reflect.Kind{
		reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64,
		reflect.String,
	}, t.Kind())
}

func quoteValue(v reflect.Value) bool {
	return quoteType(v.Type())
}

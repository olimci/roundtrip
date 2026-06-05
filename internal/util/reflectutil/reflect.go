package reflectutil

import "reflect"

// ReflectField pairs a selected field name with its value.
type ReflectField struct {
	Name  string
	Value reflect.Value
}

// Nilable reports whether values of kind k can be nil.
func Nilable(k reflect.Kind) bool {
	switch k {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}

// Indirect unwraps pointers and interfaces until it reaches a concrete value or
// nil.
func Indirect(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return v
		}
		v = v.Elem()
	}
	return v
}

// StructFieldIndexes returns indexes for exported fields accepted by name.
//
// t must be a struct type and name must be non-nil.
func StructFieldIndexes(t reflect.Type, name func(reflect.StructField) (string, bool)) map[string]int {
	fields := map[string]int{}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}

		fieldName, ok := name(f)
		if !ok {
			continue
		}
		fields[fieldName] = i
	}
	return fields
}

// StructFields returns exported fields accepted by name.
//
// v must be a struct value and name must be non-nil.
func StructFields(v reflect.Value, name func(reflect.StructField) (string, bool)) []ReflectField {
	fields := []ReflectField{}
	t := v.Type()
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.PkgPath != "" {
			continue
		}

		fieldName, ok := name(f)
		if !ok {
			continue
		}
		fields = append(fields, ReflectField{Name: fieldName, Value: v.Field(i)})
	}
	return fields
}

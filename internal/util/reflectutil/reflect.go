package reflectutil

import "reflect"

// TODO: tests...

type ReflectField struct {
	Name  string
	Value reflect.Value
}

func Nilable(k reflect.Kind) bool {
	switch k {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}

func Indirect(v reflect.Value) reflect.Value {
	for v.IsValid() && (v.Kind() == reflect.Pointer || v.Kind() == reflect.Interface) {
		if v.IsNil() {
			return v
		}
		v = v.Elem()
	}
	return v
}

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

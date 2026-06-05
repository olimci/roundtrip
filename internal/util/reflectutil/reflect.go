package reflectutil

import "reflect"

// Nilable reports whether values of kind k can be nil.
func Nilable(k reflect.Kind) bool {
	switch k {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return true
	default:
		return false
	}
}

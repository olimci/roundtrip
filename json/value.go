package json

import (
	"fmt"
	"reflect"
	"strconv"
)

// RawMessage is a raw encoded JSON value.
//
// MarshalJSON emits nil RawMessage values as null. UnmarshalJSON requires a
// non-nil *RawMessage receiver and replaces the receiver's bytes with a copy of
// data.
type RawMessage []byte

// MarshalJSON returns m as one encoded JSON value.
func (m RawMessage) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return m, nil
}

// UnmarshalJSON replaces m with a copy of data.
//
// m must be non-nil.
func (m *RawMessage) UnmarshalJSON(data []byte) error {
	*m = append((*m)[:0], data...)
	return nil
}

// Number stores a JSON number literal without converting it to float64.
type Number string

var numberType = reflect.TypeFor[Number]()

// String returns the number literal.
func (n Number) String() string {
	return string(n)
}

// MarshalJSON returns n as one JSON number.
func (n Number) MarshalJSON() ([]byte, error) {
	if !validNumber(string(n)) {
		return nil, fmt.Errorf("json: invalid number literal %q", n)
	}
	return []byte(n), nil
}

// Float64 converts n to a float64.
func (n Number) Float64() (float64, error) {
	return strconv.ParseFloat(string(n), 64)
}

// Int64 converts n to an int64.
func (n Number) Int64() (int64, error) {
	return strconv.ParseInt(string(n), 10, 64)
}

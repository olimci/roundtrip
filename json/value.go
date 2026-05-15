package json

import (
	stdjson "encoding/json"
	"fmt"
	"reflect"
)

type RawMessage []byte

func (m RawMessage) MarshalJSON() ([]byte, error) {
	if m == nil {
		return []byte("null"), nil
	}
	return m, nil
}

func (m *RawMessage) UnmarshalJSON(data []byte) error {
	*m = append((*m)[:0], data...)
	return nil
}

type Number string

var numberType = reflect.TypeFor[Number]()

func (n Number) String() string {
	return string(n)
}

func (n Number) MarshalJSON() ([]byte, error) {
	if !validNumber(string(n)) {
		return nil, fmt.Errorf("json: invalid number literal %q", n)
	}
	return []byte(n), nil
}

func (n Number) Float64() (float64, error) {
	return stdjson.Number(n).Float64()
}

func (n Number) Int64() (int64, error) {
	return stdjson.Number(n).Int64()
}

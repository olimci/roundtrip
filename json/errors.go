package json

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	ErrUnexpectedEOF   = errors.New("unexpected EOF")
	ErrUnexpectedToken = errors.New("unexpected token")
)

type ParseError struct {
	Err   error
	Token token
}

func (e ParseError) Error() string {
	if e.Token.Type == TokenEOF {
		return fmt.Sprintf("%v at %d:%d", e.Err, e.Token.Position.Line, e.Token.Position.Column)
	}
	return fmt.Sprintf("%v %s at %d:%d", e.Err, e.Token.Type, e.Token.Position.Line, e.Token.Position.Column)
}

func (e ParseError) Unwrap() error {
	return e.Err
}

type SyntaxError struct {
	msg    string
	Offset int64
}

func (e *SyntaxError) Error() string {
	return e.msg
}

type UnmarshalTypeError struct {
	Value  string
	Type   reflect.Type
	Offset int64
	Struct string
	Field  string
}

func (e *UnmarshalTypeError) Error() string {
	if e.Struct != "" || e.Field != "" {
		return fmt.Sprintf("json: cannot unmarshal %s into Go struct field %s.%s of type %s", e.Value, e.Struct, e.Field, e.Type)
	}
	return fmt.Sprintf("json: cannot unmarshal %s into Go value of type %s", e.Value, e.Type)
}

type InvalidUnmarshalError struct {
	Type reflect.Type
}

func (e InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "json: Unmarshal(nil)"
	}
	if e.Type.Kind() != reflect.Pointer {
		return "json: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "json: Unmarshal(nil " + e.Type.String() + ")"
}

type UnsupportedTypeError struct {
	Type reflect.Type
}

func (e *UnsupportedTypeError) Error() string {
	return "json: unsupported type: " + e.Type.String()
}

type UnsupportedValueError struct {
	Value reflect.Value
	Str   string
}

func (e *UnsupportedValueError) Error() string {
	return "json: unsupported value: " + e.Str
}

type MarshalerError struct {
	Type reflect.Type
	Err  error
}

func (e *MarshalerError) Error() string {
	return "json: error calling MarshalJSON for type " + e.Type.String() + ": " + e.Err.Error()
}

func (e *MarshalerError) Unwrap() error {
	return e.Err
}

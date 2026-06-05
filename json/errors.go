package json

import (
	"errors"
	"fmt"
	"reflect"
)

var (
	// ErrUnexpectedEOF reports input that ended before a complete value was parsed.
	ErrUnexpectedEOF = errors.New("unexpected EOF")
	// ErrUnexpectedToken reports a token that is not valid at the current parse position.
	ErrUnexpectedToken = errors.New("unexpected token")
	// ErrInvalidString reports a malformed string literal.
	ErrInvalidString = errors.New("invalid string")
	// ErrInvalidNumber reports a malformed number literal.
	ErrInvalidNumber = errors.New("invalid number")
	// ErrInvalidSpace reports whitespace that is not valid for the active syntax options.
	ErrInvalidSpace = errors.New("invalid whitespace")
	// ErrDuplicateObjectKey reports an object key repeated in the same object.
	ErrDuplicateObjectKey = errors.New("duplicate object key")
)

// ParseError describes a lexer or parser error at a concrete input token.
type ParseError struct {
	Err   error
	Token token
}

// Error returns the formatted parse error.
func (e ParseError) Error() string {
	if e.Token.Type == TokenEOF {
		return fmt.Sprintf("%v at %d:%d", e.Err, e.Token.Position.Line, e.Token.Position.Column)
	}
	return fmt.Sprintf("%v %s at %d:%d", e.Err, e.Token.Type, e.Token.Position.Line, e.Token.Position.Column)
}

// Unwrap returns the underlying parse error sentinel.
func (e ParseError) Unwrap() error {
	return e.Err
}

// SyntaxError describes invalid JSON syntax.
type SyntaxError struct {
	msg    string
	Offset int64
}

// Error returns the formatted syntax error.
func (e *SyntaxError) Error() string {
	return e.msg
}

// UnmarshalTypeError describes a JSON value that cannot be assigned to the
// requested Go type.
type UnmarshalTypeError struct {
	Value  string
	Type   reflect.Type
	Offset int64
	Struct string
	Field  string
}

// Error returns the formatted unmarshal type error.
func (e *UnmarshalTypeError) Error() string {
	if e.Struct != "" || e.Field != "" {
		return fmt.Sprintf("json: cannot unmarshal %s into Go struct field %s.%s of type %s", e.Value, e.Struct, e.Field, e.Type)
	}
	return fmt.Sprintf("json: cannot unmarshal %s into Go value of type %s", e.Value, e.Type)
}

// InvalidUnmarshalError describes an invalid target passed to Unmarshal,
// Decoder.Decode, or Node.Decode.
//
// Those APIs require v to be a non-nil pointer to the value being populated.
type InvalidUnmarshalError struct {
	Type reflect.Type
}

// Error returns the formatted invalid unmarshal target error.
func (e InvalidUnmarshalError) Error() string {
	if e.Type == nil {
		return "json: Unmarshal(nil)"
	}
	if e.Type.Kind() != reflect.Pointer {
		return "json: Unmarshal(non-pointer " + e.Type.String() + ")"
	}
	return "json: Unmarshal(nil " + e.Type.String() + ")"
}

// UnsupportedTypeError describes a Go type that cannot be encoded as JSON.
type UnsupportedTypeError struct {
	Type reflect.Type
}

// Error returns the formatted unsupported type error.
func (e *UnsupportedTypeError) Error() string {
	return "json: unsupported type: " + e.Type.String()
}

// UnsupportedValueError describes a Go value that cannot be encoded as JSON.
type UnsupportedValueError struct {
	Value reflect.Value
	Str   string
}

// Error returns the formatted unsupported value error.
func (e *UnsupportedValueError) Error() string {
	return "json: unsupported value: " + e.Str
}

// MarshalerError wraps an error returned by a Marshaler implementation.
type MarshalerError struct {
	Type       reflect.Type
	Err        error
	SourceFunc string
}

// Error returns the formatted marshaler error.
func (e *MarshalerError) Error() string {
	sourceFunc := e.SourceFunc
	if sourceFunc == "" {
		sourceFunc = "MarshalJSON"
	}
	return "json: error calling " + sourceFunc + " for type " + e.Type.String() + ": " + e.Err.Error()
}

// Unwrap returns the error reported by the Marshaler.
func (e *MarshalerError) Unwrap() error {
	return e.Err
}

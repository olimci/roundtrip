package sst

// Enum is the integer constraint used by token and node enum types.
//
// Anchor token values must be -1.
type Enum interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

// IsAnchor reports whether t is an anchor token.
func IsAnchor[TT Enum](t Token[TT]) bool {
	return t.Type == TT(-1)
}

package sst

// Anchor tokens should always be -1
type Enum interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64
}

func IsAnchor[TT Enum](t Token[TT]) bool {
	return t.Type == TT(-1)
}

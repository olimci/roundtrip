package sst

import (
	"fmt"

	"github.com/olimci/roundtrip/internal/cursor"
)

// Token is one lexical token with its source position.
type Token[TT comparable] struct {
	Type    TT
	Literal string

	Position cursor.Position
}

// String returns a debug representation of t.
func (t Token[TT]) String() string {
	return fmt.Sprint(t.Type) + "(" + t.Literal + ")"
}

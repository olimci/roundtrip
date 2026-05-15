package sst

import (
	"fmt"

	"github.com/olimci/roundtrip/internal/cursor"
)

type Token[TT comparable] struct {
	Type    TT
	Literal string

	Position cursor.Position
}

func (t Token[TT]) String() string {
	return fmt.Sprint(t.Type) + "(" + t.Literal + ")"
}

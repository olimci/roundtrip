package sst

import (
	"testing"

	"github.com/olimci/roundtrip/internal/list"
)

type testTokenType int8
type testNodeType int8

const (
	testTokenAnchor testTokenType = -1
	testTokenValue  testTokenType = iota
)

func TestNodeBytesStripsAnchorTokens(t *testing.T) {
	tokens := list.FromSlice([]Token[testTokenType]{
		{Type: testTokenAnchor, Literal: "before"},
		{Type: testTokenValue, Literal: "a"},
		{Type: testTokenAnchor, Literal: "middle"},
		{Type: testTokenValue, Literal: "b"},
		{Type: testTokenAnchor, Literal: "after"},
	})

	n := &Node[testTokenType, testNodeType]{
		Start: tokens.Head,
		End:   tokens.Tail,
	}

	if got := string(n.Bytes()); got != "ab" {
		t.Fatalf("Bytes() = %q, want %q", got, "ab")
	}
}

func TestNodeTokensStripsAnchorTokensAtBounds(t *testing.T) {
	tokens := list.FromSlice([]Token[testTokenType]{
		{Type: testTokenAnchor, Literal: "start"},
		{Type: testTokenValue, Literal: "value"},
		{Type: testTokenAnchor, Literal: "end"},
	})

	n := &Node[testTokenType, testNodeType]{
		Start: tokens.Head,
		End:   tokens.Tail,
	}

	var got []Token[testTokenType]
	for tok := range n.Tokens() {
		got = append(got, tok)
	}
	if len(got) != 1 || got[0].Literal != "value" {
		t.Fatalf("Tokens() = %#v, want only value token", got)
	}
}

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

func TestNodeCloneCopiesTokenElementsAndChildren(t *testing.T) {
	tokens := list.FromSlice([]Token[testTokenType]{
		{Type: testTokenAnchor, Literal: "start"},
		{Type: testTokenValue, Literal: "["},
		{Type: testTokenValue, Literal: "1"},
		{Type: testTokenValue, Literal: "]"},
		{Type: testTokenAnchor, Literal: "end"},
	})
	child := &Node[testTokenType, testNodeType]{
		Start: tokens.Head.Next.Next,
		End:   tokens.Head.Next.Next,
	}
	root := &Node[testTokenType, testNodeType]{
		Start:    tokens.Head,
		End:      tokens.Tail,
		Children: []*Node[testTokenType, testNodeType]{child},
	}

	clone, cloneTokens := root.Clone()
	if clone == root {
		t.Fatal("Clone returned original root")
	}
	if clone.Start == root.Start || clone.End == root.End {
		t.Fatal("Clone reused boundary token elements")
	}
	if clone.Children[0] == child {
		t.Fatal("Clone reused child node")
	}
	if clone.Children[0].Start == child.Start {
		t.Fatal("Clone reused child token element")
	}
	if got := string(clone.Bytes()); got != "[1]" {
		t.Fatalf("Clone bytes = %q, want %q", got, "[1]")
	}

	cloneTokens.Remove(clone.Children[0].Start)
	if got := string(root.Bytes()); got != "[1]" {
		t.Fatalf("Mutating clone tokens changed original bytes to %q", got)
	}
}

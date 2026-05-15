package sst

import (
	"iter"
	"strings"

	"github.com/olimci/roundtrip/internal/list"
)

type Node[TT, NT Enum] struct {
	Type     NT
	Start    *list.Elem[Token[TT]]
	End      *list.Elem[Token[TT]]
	Children []*Node[TT, NT]
}

func (n *Node[TT, NT]) Tokens() iter.Seq[Token[TT]] {
	return func(yield func(Token[TT]) bool) {
		if n.Start == nil || n.End == nil {
			return
		}

		for e := n.Start; e != nil; e = e.Next {
			if !IsAnchor(e.Value) && !yield(e.Value) {
				return
			}

			if e == n.End {
				return
			}
		}
	}
}

func (n *Node[TT, NT]) Bytes() []byte {
	var sb strings.Builder

	for tok := range n.Tokens() {
		sb.WriteString(tok.Literal)
	}

	return []byte(sb.String())
}

func (n *Node[TT, NT]) Walk() iter.Seq[*Node[TT, NT]] {
	return func(yield func(*Node[TT, NT]) bool) {
		WalkNodes(n, yield)
	}
}

func (n *Node[TT, NT]) WalkLeaves() iter.Seq[*Node[TT, NT]] {
	return func(yield func(*Node[TT, NT]) bool) {
		WalkLeaves(n, yield)
	}
}

func WalkNodes[TT, NT Enum](n *Node[TT, NT], yield func(*Node[TT, NT]) bool) bool {
	if !yield(n) {
		return false
	}
	for _, child := range n.Children {
		if !WalkNodes(child, yield) {
			return false
		}
	}
	return true
}

func WalkLeaves[TT, NT Enum](n *Node[TT, NT], yield func(*Node[TT, NT]) bool) bool {
	if len(n.Children) == 0 {
		return yield(n)
	}
	for _, child := range n.Children {
		if !WalkLeaves(child, yield) {
			return false
		}
	}
	return true
}

package sst

import (
	"iter"
	"strings"

	"github.com/olimci/roundtrip/internal/list"
)

// Node is one syntax tree node backed by an inclusive token range.
type Node[TT, NT Enum] struct {
	Type     NT
	Start    *list.Elem[Token[TT]]
	End      *list.Elem[Token[TT]]
	Children []*Node[TT, NT]
}

// Tokens iterates over n's inclusive token range, skipping anchor tokens.
//
// n must be non-nil.
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

// Bytes returns the source bytes represented by n.
//
// n must be non-nil.
func (n *Node[TT, NT]) Bytes() []byte {
	var sb strings.Builder

	for tok := range n.Tokens() {
		sb.WriteString(tok.Literal)
	}

	return []byte(sb.String())
}

// Clone returns a detached copy of n and the copied token list backing it.
//
// n must be non-nil.
func (n *Node[TT, NT]) Clone() (*Node[TT, NT], *list.List[Token[TT]]) {
	tokens := new(list.List[Token[TT]])
	elems := map[*list.Elem[Token[TT]]]*list.Elem[Token[TT]]{}
	for e := n.Start; e != nil; e = e.Next {
		elems[e] = tokens.PushBack(e.Value)
		if e == n.End {
			break
		}
	}
	return cloneNode(n, elems), tokens
}

func cloneNode[TT, NT Enum](n *Node[TT, NT], elems map[*list.Elem[Token[TT]]]*list.Elem[Token[TT]]) *Node[TT, NT] {
	clone := &Node[TT, NT]{
		Type:     n.Type,
		Start:    elems[n.Start],
		End:      elems[n.End],
		Children: make([]*Node[TT, NT], len(n.Children)),
	}
	for i, child := range n.Children {
		clone.Children[i] = cloneNode(child, elems)
	}
	return clone
}

// WalkNodes visits n and each descendant in pre-order.
//
// n and yield must be non-nil.
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

// WalkLeaves visits each leaf descendant of n in source order.
//
// n and yield must be non-nil.
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

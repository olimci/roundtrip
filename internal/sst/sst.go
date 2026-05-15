package sst

import (
	"iter"

	"github.com/olimci/roundtrip/internal/list"
)

// TODO: tests...

type SST[TT, NT Enum] struct {
	Tokens *list.List[Token[TT]]
	Root   *Node[TT, NT]
}

func (m *SST[TT, NT]) Nodes() iter.Seq[*Node[TT, NT]] {
	return m.Root.Walk()
}

func (m *SST[TT, NT]) Leaves() iter.Seq[*Node[TT, NT]] {
	return func(yield func(*Node[TT, NT]) bool) {
		WalkLeaves(m.Root, yield)
	}
}

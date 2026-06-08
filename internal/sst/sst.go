package sst

import "github.com/olimci/roundtrip/internal/list"

// SST is a source syntax tree and its backing token list.
type SST[TT, NT Enum] struct {
	Tokens *list.List[Token[TT]]
	Root   *Node[TT, NT]
}

// Clone returns a detached copy of s.
//
// s must contain a non-nil token list and root node.
func (s SST[TT, NT]) Clone() SST[TT, NT] {
	tokens, elems := s.Tokens.Clone()
	return SST[TT, NT]{
		Tokens: tokens,
		Root:   cloneNode(s.Root, elems),
	}
}

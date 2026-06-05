package sst

import "github.com/olimci/roundtrip/internal/list"

// SST is a source syntax tree and its backing token list.
type SST[TT, NT Enum] struct {
	Tokens *list.List[Token[TT]]
	Root   *Node[TT, NT]
}

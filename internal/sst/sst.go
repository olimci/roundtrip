package sst

import "github.com/olimci/roundtrip/internal/list"

// TODO: tests...

type SST[TT, NT Enum] struct {
	Tokens *list.List[Token[TT]]
	Root   *Node[TT, NT]
}

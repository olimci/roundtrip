package sst

import "github.com/olimci/roundtrip/internal/list"

type SST[TT, NT Enum] struct {
	Tokens *list.List[Token[TT]]
	Root   *Node[TT, NT]
}

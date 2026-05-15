package json

import (
	"errors"
	"iter"
	"strconv"
	"strings"

	"github.com/olimci/roundtrip/internal/list"
	"github.com/olimci/roundtrip/internal/sst"
	"github.com/olimci/roundtrip/internal/util/iterutil"
	"github.com/olimci/roundtrip/internal/util/sliceutil"
)

var (
	ErrWrongNodeType        = errors.New("wrong node type")
	ErrObjectFieldNotFound  = errors.New("object field not found")
	ErrArrayIndexOutOfRange = errors.New("array index out of range")
)

type Meta struct {
	SST    sst.SST[TokenType, NodeType]
	Indent string
}

func (m *Meta) node(n *node) Node {
	return Node{meta: m, node: n}
}

type Node struct {
	meta *Meta
	node *node
}

func (n Node) Type() NodeType {
	return n.node.Type
}

func (n Node) Children() []Node {
	return sliceutil.Map(n.node.Children, n.meta.node)
}

func (n Node) Bytes() []byte {
	return n.node.Bytes()
}

func (m *Meta) Root() Node {
	return Node{meta: m, node: m.SST.Root}
}

func (m *Meta) Nodes() iter.Seq[Node] {
	return iterutil.Map(m.SST.Nodes(), func(n *node) Node {
		return Node{meta: m, node: n}
	})
}

func (m *Meta) Leaves() iter.Seq[Node] {
	return iterutil.Map(m.SST.Leaves(), func(n *node) Node {
		return Node{meta: m, node: n}
	})
}

func (n Node) Decode(v any) error {
	return decodeInto(n.meta, n.node, v, decodeOptions{})
}

func (n Node) Replace(v any) error {
	newNode, tokens, err := encode(v, n.meta.Indent, n.depth())
	if err != nil {
		return err
	}

	n.meta.SST.Tokens.ReplaceRange(n.node.Start, n.node.End, tokens)
	*n.node = *newNode
	return nil
}

func (n Node) ObjectField(name string) (Node, bool) {
	if n.node.Type != NodeTypeObject {
		return Node{}, false
	}
	_, value, ok := n.objectField(name)
	if !ok {
		return Node{}, false
	}
	return n.meta.node(value), true
}

func (n Node) InsertObjectField(name string, value any) error {
	if n.node.Type != NodeTypeObject {
		return ErrWrongNodeType
	}
	key, valueNode, encoded, err := encodeObjectField(name, value, n.meta.Indent, n.depth()+1, n.fieldValuePrefix())
	if err != nil {
		return err
	}

	if len(n.node.Children) == 0 {
		encoded.PushFrontList(gapTokens(n.node.Start, n.node.End))
		n.meta.SST.Tokens.InsertListAfter(n.node.Start, encoded)
	} else {
		encoded.PushFront(token{Type: TokenComma, Literal: ","})
		encoded.InsertListAfter(encoded.Head, n.leadingGap(len(n.node.Children)-2))
		n.meta.SST.Tokens.InsertListAfter(n.lastChild().End, encoded)
	}
	appendObjectField(n.node, key, valueNode)
	return nil
}

func (n Node) RemoveObjectField(name string) error {
	if n.node.Type != NodeTypeObject {
		return ErrWrongNodeType
	}
	index, _, _, ok := n.objectFieldIndex(name)
	if !ok {
		return ErrObjectFieldNotFound
	}
	first, last := n.removeChildPairRange(index * 2)
	n.meta.SST.Tokens.ReplaceRange(first, last, new(list.List[token]))
	n.node.Children = append(n.node.Children[:index*2], n.node.Children[index*2+2:]...)
	return nil
}

func (n Node) RenameObjectField(oldName, newName string) error {
	if n.node.Type != NodeTypeObject {
		return ErrWrongNodeType
	}
	key, _, ok := n.objectField(oldName)
	if !ok {
		return ErrObjectFieldNotFound
	}
	newKey, tokens, err := encode(newName, n.meta.Indent, n.depth())
	if err != nil {
		return err
	}
	n.meta.SST.Tokens.ReplaceRange(key.Start, key.End, tokens)
	*key = *newKey
	return nil
}

func (n Node) AppendArrayValue(value any) error {
	if n.node.Type != NodeTypeArray {
		return ErrWrongNodeType
	}
	valueNode, tokens, err := encode(value, n.meta.Indent, n.depth()+1)
	if err != nil {
		return err
	}
	if len(n.node.Children) == 0 {
		tokens.PushFrontList(gapTokens(n.node.Start, n.node.End))
		n.meta.SST.Tokens.InsertListAfter(n.node.Start, tokens)
	} else {
		tokens.PushFront(token{Type: TokenComma, Literal: ","})
		tokens.InsertListAfter(tokens.Head, n.leadingGap(len(n.node.Children)-1))
		n.meta.SST.Tokens.InsertListAfter(n.lastChild().End, tokens)
	}
	n.node.Children = append(n.node.Children, valueNode)
	return nil
}

func (n Node) RemoveArrayValue(index int) error {
	if n.node.Type != NodeTypeArray {
		return ErrWrongNodeType
	}
	if index < 0 || index >= len(n.node.Children) {
		return ErrArrayIndexOutOfRange
	}
	first, last := n.removeChildRange(index)
	n.meta.SST.Tokens.ReplaceRange(first, last, new(list.List[token]))
	n.node.Children = append(n.node.Children[:index], n.node.Children[index+1:]...)
	return nil
}

func (n Node) TrailingComment() (Comment, bool) {
	seenComma := false
	for e := n.node.End.Next; e != nil; e = e.Next {
		t := e.Value
		switch t.Type {
		case TokenWhitespace:
			continue
		case TokenComma:
			if seenComma {
				return Comment{}, false
			}
			seenComma = true
		case TokenComment:
			return Comment{elem: e}, true
		default:
			return Comment{}, false
		}
	}
	return Comment{}, false
}

func (n Node) depth() int {
	return nodeDepth(n.meta.SST.Root, n.node, 0)
}

func (n Node) objectField(name string) (*node, *node, bool) {
	_, key, value, ok := n.objectFieldIndex(name)
	return key, value, ok
}

func (n Node) objectFieldIndex(name string) (int, *node, *node, bool) {
	for i := 0; i < len(n.node.Children); i += 2 {
		key := n.node.Children[i]
		keyName, err := strconv.Unquote(key.Start.Value.Literal)
		if err == nil && keyName == name {
			return i / 2, key, n.node.Children[i+1], true
		}
	}
	return 0, nil, nil, false
}

func (n Node) lastChild() *node {
	return n.node.Children[len(n.node.Children)-1]
}

func (n Node) leadingGap(childIndex int) *list.List[token] {
	tokens := new(list.List[token])
	start := n.node.Start
	if childIndex > 0 {
		start = n.node.Children[childIndex-1].End
	}
	for e := start.Next; e != nil && e != n.node.Children[childIndex].Start; e = e.Next {
		if e.Value.Type == TokenComma {
			tokens = new(list.List[token])
			continue
		}
		switch e.Value.Type {
		case TokenWhitespace, TokenNewline:
			tokens.PushBack(e.Value)
			if e.Value.Type == TokenNewline {
				tokens = list.New[token]()
				tokens.PushBack(e.Value)
			}
		case TokenComment:
			tokens = list.New[token]()
		default:
			return tokens
		}
	}
	return tokens
}

func (n Node) removeChildPairRange(childIndex int) (*list.Elem[token], *list.Elem[token]) {
	if len(n.node.Children) == 2 {
		return n.node.Children[childIndex].Start, n.node.Children[childIndex+1].End
	}
	if childIndex < len(n.node.Children)-2 {
		return n.node.Children[childIndex].Start, n.node.Children[childIndex+2].Start.Prev
	}
	return n.node.Children[childIndex-1].End.Next, n.node.Children[childIndex+1].End
}

func (n Node) removeChildRange(index int) (*list.Elem[token], *list.Elem[token]) {
	if len(n.node.Children) == 1 {
		return n.node.Children[index].Start, n.node.Children[index].End
	}
	if index < len(n.node.Children)-1 {
		return n.node.Children[index].Start, n.node.Children[index+1].Start.Prev
	}
	return n.node.Children[index-1].End.Next, n.node.Children[index].End
}

func (n Node) fieldValuePrefix() string {
	for i := 0; i+1 < len(n.node.Children); i += 2 {
		key := n.node.Children[i]
		value := n.node.Children[i+1]
		afterColon := false
		var b strings.Builder
		for e := key.End.Next; e != value.Start; e = e.Next {
			if afterColon {
				b.WriteString(e.Value.Literal)
			}
			if e.Value.Type == TokenColon {
				afterColon = true
			}
		}
		if afterColon {
			return b.String()
		}
	}
	return ""
}

func objectFields(n *node) iter.Seq2[*node, *node] {
	return func(yield func(*node, *node) bool) {
		for i := 0; i < len(n.Children); i += 2 {
			if !yield(n.Children[i], n.Children[i+1]) {
				return
			}
		}
	}
}

func appendObjectField(n, key, value *node) {
	n.Children = append(n.Children, key, value)
}

func encodeObjectField(name string, value any, indent string, depth int, valuePrefix string) (*node, *node, *list.List[token], error) {
	key, tokens, err := encode(name, indent, depth)
	if err != nil {
		return nil, nil, nil, err
	}
	valueNode, valueTokens, err := encode(value, indent, depth)
	if err != nil {
		return nil, nil, nil, err
	}
	tokens.PushBack(token{Type: TokenColon, Literal: ":"})
	if valuePrefix != "" {
		tokens.PushBack(token{Type: TokenWhitespace, Literal: valuePrefix})
	}
	tokens.PushBackList(valueTokens)
	return key, valueNode, tokens, nil
}

func gapTokens(first, last *list.Elem[token]) *list.List[token] {
	tokens := new(list.List[token])
	for e := first.Next; e != nil && e != last; e = e.Next {
		switch e.Value.Type {
		case TokenWhitespace, TokenNewline:
			tokens.PushBack(e.Value)
		default:
			return tokens
		}
	}
	return tokens
}

func commentText(raw string) string {
	if after, ok := strings.CutPrefix(raw, "//"); ok {
		return strings.TrimSpace(after)
	}
	if strings.HasPrefix(raw, "/*") && strings.HasSuffix(raw, "*/") {
		raw = strings.TrimPrefix(strings.TrimSuffix(raw, "*/"), "/*")
		return strings.TrimSpace(raw)
	}
	return raw
}

func commentLiteralText(lit, text string) (string, error) {
	if strings.HasPrefix(lit, "/*") && strings.HasSuffix(lit, "*/") {
		if strings.Contains(text, "*/") {
			return "", ErrInvalidComment
		}
		return "/* " + text + " */", nil
	}
	if strings.ContainsAny(text, "\r\n") {
		if strings.Contains(text, "*/") {
			return "", ErrInvalidComment
		}
		return "/* " + text + " */", nil
	}
	return "// " + text, nil
}

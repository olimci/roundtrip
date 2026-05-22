package json

import (
	"errors"
	"iter"
	"strconv"
	"strings"

	"github.com/olimci/roundtrip/internal/list"
	"github.com/olimci/roundtrip/internal/sst"
)

var (
	ErrWrongNodeType        = errors.New("wrong node type")
	ErrObjectFieldNotFound  = errors.New("object field not found")
	ErrObjectFieldExists    = errors.New("object field exists")
	ErrArrayIndexOutOfRange = errors.New("array index out of range")
)

type Meta struct {
	SST    sst.SST[TokenType, NodeType]
	Indent string
	syntax SyntaxOptions
}

type Node struct {
	meta *Meta
	node *node
}

func (n Node) Type() NodeType {
	return n.node.Type
}

func (n Node) Children() []Node {
	children := make([]Node, len(n.node.Children))
	for i, child := range n.node.Children {
		children[i] = Node{meta: n.meta, node: child}
	}
	return children
}

func (n Node) Bytes() []byte {
	return n.node.Bytes()
}

func (m *Meta) Root() Node {
	return Node{meta: m, node: m.SST.Root}
}

func (m *Meta) Comments() CommentSet {
	return CommentSet{
		Leading:  commentsBackward(m.SST.Root.Start.Prev),
		Trailing: commentsForward(m.SST.Root.End.Next),
	}
}

func (m *Meta) Nodes() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		sst.WalkNodes(m.SST.Root, func(n *node) bool {
			return yield(Node{meta: m, node: n})
		})
	}
}

func (m *Meta) Leaves() iter.Seq[Node] {
	return func(yield func(Node) bool) {
		sst.WalkLeaves(m.SST.Root, func(n *node) bool {
			return yield(Node{meta: m, node: n})
		})
	}
}

func (n Node) Decode(v any) error {
	return decodeInto(n.meta, n.node, v, decodeOptions{syntax: n.meta.syntax})
}

func (n Node) Replace(v any) error {
	if value, ok := nodeValue(v); ok {
		return n.replaceWithNode(value)
	}

	newNode, tokens, err := encode(v, n.meta.Indent, n.depth())
	if err != nil {
		return err
	}

	n.meta.SST.Tokens.ReplaceRange(n.node.Start, n.node.End, tokens)
	*n.node = *newNode
	return nil
}

func (n Node) replaceWithNode(value Node) error {
	newNode, tokens := value.node.Clone()
	n.meta.SST.Tokens.ReplaceRange(n.node.Start, n.node.End, tokens)
	*n.node = *newNode
	return nil
}

func (n Node) ObjectField(name string) (Node, bool) {
	field, ok := n.ObjectFieldNode(name)
	if !ok {
		return Node{}, false
	}
	return field.Value()
}

func (n Node) ObjectFieldNode(name string) (Node, bool) {
	if n.node.Type != NodeTypeObject {
		return Node{}, false
	}
	_, field, ok := n.objectFieldIndex(name)
	if !ok {
		return Node{}, false
	}
	return Node{meta: n.meta, node: field}, true
}

func (n Node) ObjectFields() iter.Seq2[string, Node] {
	return func(yield func(string, Node) bool) {
		if n.node.Type != NodeTypeObject {
			return
		}
		for _, field := range n.node.Children {
			key := objectFieldKey(field)
			name, err := decodeKeyLiteral(key)
			if err != nil {
				continue
			}
			if !yield(name, Node{meta: n.meta, node: field}) {
				return
			}
		}
	}
}

func (n Node) Key() (Node, bool) {
	if n.node.Type != NodeTypeObjectField {
		return Node{}, false
	}
	return Node{meta: n.meta, node: objectFieldKey(n.node)}, true
}

func (n Node) Value() (Node, bool) {
	switch n.node.Type {
	case NodeTypeObjectField:
		return Node{meta: n.meta, node: objectFieldValue(n.node)}, true
	case NodeTypeArrayElement:
		return Node{meta: n.meta, node: arrayElementValue(n.node)}, true
	default:
		return Node{}, false
	}
}

func (n Node) ReplaceObjectField(name string, value any) error {
	field, ok := n.ObjectField(name)
	if !ok {
		if n.node.Type != NodeTypeObject {
			return ErrWrongNodeType
		}
		return ErrObjectFieldNotFound
	}
	return field.Replace(value)
}

func (n Node) InsertObjectField(name string, value any) error {
	if value, ok := nodeValue(value); ok {
		return n.insertObjectFieldNode(name, value)
	}

	if n.node.Type != NodeTypeObject {
		return ErrWrongNodeType
	}
	field, encoded, err := encodeObjectField(name, value, n.meta.Indent, n.depth()+1, n.fieldValuePrefix())
	if err != nil {
		return err
	}

	if len(n.node.Children) == 0 {
		encoded.PushFrontList(gapTokens(n.node.Start, n.node.End))
		n.meta.SST.Tokens.InsertListAfter(n.node.Start, encoded)
	} else {
		encoded.PushFront(token{Type: TokenComma, Literal: ","})
		encoded.InsertListAfter(encoded.Head, n.leadingGap(len(n.node.Children)-1))
		n.meta.SST.Tokens.InsertListAfter(n.node.Children[len(n.node.Children)-1].End, encoded)
	}
	n.node.Children = append(n.node.Children, field)
	return nil
}

func (n Node) insertObjectFieldNode(name string, value Node) error {
	if n.node.Type != NodeTypeObject {
		return ErrWrongNodeType
	}
	key, tokens, err := encode(name, n.meta.Indent, n.depth()+1)
	if err != nil {
		return err
	}
	valueNode, valueTokens := value.node.Clone()
	tokens.PushBack(token{Type: TokenColon, Literal: ":"})
	if valuePrefix := n.fieldValuePrefix(); valuePrefix != "" {
		tokens.PushBack(token{Type: TokenWhitespace, Literal: valuePrefix})
	}
	tokens.PushBackList(valueTokens)
	start := tokens.PushFront(token{Type: TokenAnchor})
	end := tokens.PushBack(token{Type: TokenAnchor})
	field := objectFieldNode(key, valueNode, start, end)

	if len(n.node.Children) == 0 {
		tokens.PushFrontList(gapTokens(n.node.Start, n.node.End))
		n.meta.SST.Tokens.InsertListAfter(n.node.Start, tokens)
	} else {
		tokens.PushFront(token{Type: TokenComma, Literal: ","})
		tokens.InsertListAfter(tokens.Head, n.leadingGap(len(n.node.Children)-1))
		n.meta.SST.Tokens.InsertListAfter(n.node.Children[len(n.node.Children)-1].End, tokens)
	}
	n.node.Children = append(n.node.Children, field)
	return nil
}

func (n Node) RemoveObjectField(name string) error {
	if n.node.Type != NodeTypeObject {
		return ErrWrongNodeType
	}
	index, _, ok := n.objectFieldIndex(name)
	if !ok {
		return ErrObjectFieldNotFound
	}
	first, last := n.removeChildRange(index)
	n.meta.SST.Tokens.ReplaceRange(first, last, new(list.List[token]))
	n.node.Children = append(n.node.Children[:index], n.node.Children[index+1:]...)
	return nil
}

func (n Node) RenameObjectField(oldName, newName string) error {
	if n.node.Type != NodeTypeObject {
		return ErrWrongNodeType
	}
	_, field, ok := n.objectFieldIndex(oldName)
	if !ok {
		return ErrObjectFieldNotFound
	}
	key := objectFieldKey(field)
	newKey, tokens, err := encode(newName, n.meta.Indent, n.depth())
	if err != nil {
		return err
	}
	n.meta.SST.Tokens.ReplaceRange(key.Start, key.End, tokens)
	*key = *newKey
	return nil
}

func (n Node) ArrayValue(index int) (Node, bool) {
	if n.node.Type != NodeTypeArray || index < 0 || index >= len(n.node.Children) {
		return Node{}, false
	}
	return Node{meta: n.meta, node: arrayElementValue(n.node.Children[index])}, true
}

func (n Node) ArrayElement(index int) (Node, bool) {
	if n.node.Type != NodeTypeArray || index < 0 || index >= len(n.node.Children) {
		return Node{}, false
	}
	return Node{meta: n.meta, node: n.node.Children[index]}, true
}

func (n Node) ReplaceArrayValue(index int, value any) error {
	element, ok := n.ArrayValue(index)
	if !ok {
		if n.node.Type != NodeTypeArray {
			return ErrWrongNodeType
		}
		return ErrArrayIndexOutOfRange
	}
	return element.Replace(value)
}

func (n Node) InsertArrayValue(index int, value any) error {
	if value, ok := nodeValue(value); ok {
		return n.insertArrayValueNode(index, value)
	}

	if n.node.Type != NodeTypeArray {
		return ErrWrongNodeType
	}
	if index < 0 || index > len(n.node.Children) {
		return ErrArrayIndexOutOfRange
	}
	element, tokens, err := encodeArrayElement(value, n.meta.Indent, n.depth()+1)
	if err != nil {
		return err
	}
	switch {
	case len(n.node.Children) == 0:
		tokens.PushFrontList(gapTokens(n.node.Start, n.node.End))
		n.meta.SST.Tokens.InsertListAfter(n.node.Start, tokens)
	case index == 0:
		tokens.PushFrontList(gapTokens(n.node.Start, n.node.Children[0].Start))
		tokens.PushBack(token{Type: TokenComma, Literal: ","})
		n.meta.SST.Tokens.InsertListAfter(n.node.Start, tokens)
	case index == len(n.node.Children):
		tokens.PushFront(token{Type: TokenComma, Literal: ","})
		tokens.InsertListAfter(tokens.Head, n.leadingGap(len(n.node.Children)-1))
		n.meta.SST.Tokens.InsertListAfter(n.node.Children[len(n.node.Children)-1].End, tokens)
	default:
		tokens.PushFront(token{Type: TokenComma, Literal: ","})
		tokens.InsertListAfter(tokens.Head, n.leadingGap(index))
		n.meta.SST.Tokens.InsertListAfter(n.node.Children[index-1].End, tokens)
	}
	n.node.Children = append(n.node.Children, nil)
	copy(n.node.Children[index+1:], n.node.Children[index:])
	n.node.Children[index] = element
	return nil
}

func (n Node) insertArrayValueNode(index int, value Node) error {
	if n.node.Type != NodeTypeArray {
		return ErrWrongNodeType
	}
	if index < 0 || index > len(n.node.Children) {
		return ErrArrayIndexOutOfRange
	}
	valueNode, tokens := value.node.Clone()
	start := tokens.PushFront(token{Type: TokenAnchor})
	end := tokens.PushBack(token{Type: TokenAnchor})
	element := arrayElementNode(valueNode, start, end)
	switch {
	case len(n.node.Children) == 0:
		tokens.PushFrontList(gapTokens(n.node.Start, n.node.End))
		n.meta.SST.Tokens.InsertListAfter(n.node.Start, tokens)
	case index == 0:
		tokens.PushFrontList(gapTokens(n.node.Start, n.node.Children[0].Start))
		tokens.PushBack(token{Type: TokenComma, Literal: ","})
		n.meta.SST.Tokens.InsertListAfter(n.node.Start, tokens)
	case index == len(n.node.Children):
		tokens.PushFront(token{Type: TokenComma, Literal: ","})
		tokens.InsertListAfter(tokens.Head, n.leadingGap(len(n.node.Children)-1))
		n.meta.SST.Tokens.InsertListAfter(n.node.Children[len(n.node.Children)-1].End, tokens)
	default:
		tokens.PushFront(token{Type: TokenComma, Literal: ","})
		tokens.InsertListAfter(tokens.Head, n.leadingGap(index))
		n.meta.SST.Tokens.InsertListAfter(n.node.Children[index-1].End, tokens)
	}
	n.node.Children = append(n.node.Children, nil)
	copy(n.node.Children[index+1:], n.node.Children[index:])
	n.node.Children[index] = element
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

func nodeValue(v any) (Node, bool) {
	switch value := v.(type) {
	case Node:
		return value, true
	case *Meta:
		return value.Root(), true
	default:
		return Node{}, false
	}
}

func (n Node) TrailingComment() (Comment, bool) {
	return n.Comments().Trailing.First()
}

func (n Node) Comments() CommentSet {
	return CommentSet{
		Leading:  commentsBackward(n.node.Start.Prev),
		Trailing: commentsForward(n.node.End.Next),
		Dangling: n.danglingComments(),
	}
}

func (n Node) depth() int {
	return nodeDepth(n.meta.SST.Root, n.node, 0)
}

func (n Node) objectFieldIndex(name string) (int, *node, bool) {
	for i, field := range n.node.Children {
		key := objectFieldKey(field)
		keyName, err := decodeKeyLiteral(key)
		if err == nil && keyName == name {
			return i, field, true
		}
	}
	return 0, nil, false
}

func decodeKeyLiteral(key *node) (string, error) {
	if key.Start.Value.Type == TokenIdentifier {
		return key.Start.Value.Literal, nil
	}
	return strconv.Unquote(key.Start.Value.Literal)
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
				tokens = new(list.List[token])
				tokens.PushBack(e.Value)
			}
		case TokenComment:
			tokens = new(list.List[token])
		default:
			return tokens
		}
	}
	return tokens
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
	for _, field := range n.node.Children {
		key := objectFieldKey(field)
		value := objectFieldValue(field)
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
		for _, field := range n.Children {
			if !yield(objectFieldKey(field), objectFieldValue(field)) {
				return
			}
		}
	}
}

func encodeObjectField(name string, value any, indent string, depth int, valuePrefix string) (*node, *list.List[token], error) {
	key, tokens, err := encode(name, indent, depth)
	if err != nil {
		return nil, nil, err
	}
	valueNode, valueTokens, err := encode(value, indent, depth)
	if err != nil {
		return nil, nil, err
	}
	tokens.PushBack(token{Type: TokenColon, Literal: ":"})
	if valuePrefix != "" {
		tokens.PushBack(token{Type: TokenWhitespace, Literal: valuePrefix})
	}
	tokens.PushBackList(valueTokens)
	start := tokens.PushFront(token{Type: TokenAnchor})
	end := tokens.PushBack(token{Type: TokenAnchor})
	return objectFieldNode(key, valueNode, start, end), tokens, nil
}

func encodeArrayElement(value any, indent string, depth int) (*node, *list.List[token], error) {
	valueNode, tokens, err := encode(value, indent, depth)
	if err != nil {
		return nil, nil, err
	}
	start := tokens.PushFront(token{Type: TokenAnchor})
	end := tokens.PushBack(token{Type: TokenAnchor})
	return arrayElementNode(valueNode, start, end), tokens, nil
}

func objectFieldNode(key, value *node, start, end *list.Elem[token]) *node {
	return &node{
		Type:     NodeTypeObjectField,
		Start:    start,
		End:      end,
		Children: []*node{key, value},
	}
}

func objectFieldKey(field *node) *node {
	return field.Children[0]
}

func objectFieldValue(field *node) *node {
	return field.Children[1]
}

func arrayElementNode(value *node, start, end *list.Elem[token]) *node {
	return &node{
		Type:     NodeTypeArrayElement,
		Start:    start,
		End:      end,
		Children: []*node{value},
	}
}

func arrayElementValue(element *node) *node {
	return element.Children[0]
}

func (n Node) danglingComments() Comments {
	switch n.node.Type {
	case NodeTypeObject, NodeTypeArray:
	default:
		return nil
	}
	if len(n.node.Children) == 0 {
		return commentsBetween(n.node.Start.Next, n.node.End)
	}
	for e := n.node.Children[len(n.node.Children)-1].End.Next; e != nil && e != n.node.End; e = e.Next {
		if e.Value.Type == TokenComma {
			return commentsBetween(e.Next, n.node.End)
		}
	}
	return nil
}

func commentsBackward(e *list.Elem[token]) Comments {
	var comments Comments
	for ; e != nil; e = e.Prev {
		switch e.Value.Type {
		case TokenWhitespace, TokenNewline, TokenAnchor:
			continue
		case TokenComment:
			comments = append(comments, Comment{elem: e})
		default:
			return reverseComments(comments)
		}
	}
	return reverseComments(comments)
}

func commentsForward(e *list.Elem[token]) Comments {
	var comments Comments
	for ; e != nil; e = e.Next {
		switch e.Value.Type {
		case TokenWhitespace, TokenNewline, TokenAnchor:
			continue
		case TokenComment:
			comments = append(comments, Comment{elem: e})
		default:
			return comments
		}
	}
	return comments
}

func commentsBetween(first, stop *list.Elem[token]) Comments {
	var comments Comments
	for e := first; e != nil && e != stop; e = e.Next {
		if e.Value.Type == TokenComment {
			comments = append(comments, Comment{elem: e})
		}
	}
	return comments
}

func reverseComments(comments Comments) Comments {
	for i, j := 0, len(comments)-1; i < j; i, j = i+1, j-1 {
		comments[i], comments[j] = comments[j], comments[i]
	}
	return comments
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

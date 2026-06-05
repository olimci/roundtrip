package json

import (
	"bytes"
	"errors"
	"io"

	"github.com/olimci/roundtrip/internal/list"
	"github.com/olimci/roundtrip/internal/sst"
)

// Unmarshal parses one JSON value from data, stores it in v, and returns the
// parsed metadata tree.
//
// v must be a non-nil pointer. The returned *Meta owns the parsed document;
// Nodes obtained from it remain live handles into that Meta.
func Unmarshal(data []byte, v any) (*Meta, error) {
	d := NewDecoder(bytes.NewReader(data))
	m, err := d.DecodeMeta()
	if err != nil {
		return nil, err
	}
	return m, decodeInto(m, m.SST.Root, v, decodeOptions{
		useNumber:       d.useNumber,
		disallowUnknown: d.disallowUnknown,
		syntax:          m.syntax,
	})
}

// NewDecoder returns a decoder that reads strict JSON from r.
//
// r must be non-nil and must remain usable for the lifetime of the Decoder.
func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		l: newLexer(r),
	}
}

// NewJSONCDecoder returns a decoder that reads JSONC from r.
//
// r must be non-nil and must remain usable for the lifetime of the Decoder.
func NewJSONCDecoder(r io.Reader) *Decoder {
	d := NewDecoder(r)
	d.SyntaxOptions = JSONCSyntaxOptions()
	return d
}

// NewJSON5Decoder returns a decoder that reads JSON5 from r.
//
// r must be non-nil and must remain usable for the lifetime of the Decoder.
func NewJSON5Decoder(r io.Reader) *Decoder {
	d := NewDecoder(r)
	d.SyntaxOptions = JSON5SyntaxOptions()
	return d
}

// Decoder reads JSON values from an input stream.
//
// A Decoder is stateful and not safe for concurrent use. Its methods require a
// non-nil *Decoder returned by NewDecoder, NewJSONCDecoder, or NewJSON5Decoder.
type Decoder struct {
	l           *lexer
	tokens      *list.List[token]
	currentElem *list.Elem[token]
	SyntaxOptions
	useNumber       bool
	disallowUnknown bool
}

// DecodeMeta reads one complete JSON value and returns its metadata tree.
func (d *Decoder) DecodeMeta() (*Meta, error) {
	return d.decodeMeta(true)
}

func (d *Decoder) decodeMeta(requireEOF bool) (*Meta, error) {
	d.tokens = new(list.List[token])
	d.currentElem = nil

	root, err := d.parse()
	if err != nil {
		return nil, err
	}

	if requireEOF {
		if err := d.consume(); err != nil {
			return nil, err
		}
		t := d.l.peekToken()
		if t.Type != TokenEOF {
			return nil, ParseError{ErrUnexpectedToken, t}
		}
	}

	return &Meta{
		Indent: detectIndent(root),
		syntax: d.SyntaxOptions,
		SST: sst.SST[TokenType, NodeType]{
			Tokens: d.tokens,
			Root:   root,
		},
	}, nil
}

// Decode reads the next JSON value, stores it in v, and returns its metadata
// tree.
//
// v must be a non-nil pointer. At the end of a stream, Decode returns io.EOF.
func (d *Decoder) Decode(v any) (*Meta, error) {
	m, err := d.decodeMeta(false)
	if err != nil {
		if errors.Is(err, ErrUnexpectedEOF) && d.onlyConsumedTrivia() {
			return nil, io.EOF
		}
		return nil, err
	}
	return m, decodeInto(m, m.SST.Root, v, decodeOptions{
		useNumber:       d.useNumber,
		disallowUnknown: d.disallowUnknown,
		syntax:          m.syntax,
	})
}

// UseNumber causes Decode and Unmarshal to store numbers in interface values as
// Number instead of float64.
func (d *Decoder) UseNumber() {
	d.useNumber = true
}

// DisallowUnknownFields causes Decode to reject object keys that do not match a
// destination struct field.
func (d *Decoder) DisallowUnknownFields() {
	d.disallowUnknown = true
}

// More reports whether another element is available in the current array or
// object being decoded.
func (d *Decoder) More() bool {
	for {
		t := d.l.peekToken()
		if t.Type == TokenWhitespace || t.Type == TokenNewline || (t.Type == TokenComment && d.commentAllowed(t)) {
			_ = d.l.next()
			continue
		}
		return t.Type != TokenEOF && !isCloseDelim(t)
	}
}

// Buffered returns a reader for bytes already read from the underlying reader
// but not consumed by the decoder.
func (d *Decoder) Buffered() io.Reader {
	var b bytes.Buffer
	if d.l.peek != nil {
		b.WriteString(d.l.peek.Literal)
	}
	if n := d.l.reader.Buffered(); n > 0 {
		buf, _ := d.l.reader.Peek(n)
		b.Write(buf)
	}
	return bytes.NewReader(b.Bytes())
}

// InputOffset returns the byte offset of the decoder's current input position.
func (d *Decoder) InputOffset() int64 {
	return int64(d.l.cursor.Offset)
}

func (d *Decoder) parse() (*node, error) {
	if err := d.consume(); err != nil {
		return nil, err
	}

	t := d.l.peekToken()
	if t.Type == TokenEOF {
		return nil, ParseError{ErrUnexpectedEOF, t}
	}

	switch t.Type {
	case TokenString:
		return d.parseScalar(NodeTypeString)
	case TokenNumber:
		return d.parseScalar(NodeTypeNumber)
	case TokenIdentifier:
		return d.parseIdentifier()
	case TokenDelim:
		switch t.Literal {
		case "{":
			return d.parseObject()
		case "[":
			return d.parseArray()
		}
		return nil, ParseError{ErrUnexpectedToken, t}
	default:
		return nil, ParseError{ErrUnexpectedToken, t}
	}
}

func (d *Decoder) parseObject() (*node, error) {
	_ = d.next()
	start := d.currentElem

	n := &node{Type: NodeTypeObject, Start: start}
	if err := d.consume(); err != nil {
		return nil, err
	}

	t := d.l.peekToken()
	if isRightBrace(t) {
		_ = d.next()
		n.End = d.currentElem
		return n, nil
	}

	names := map[string]struct{}{}
	for {
		if t.Type == TokenEOF {
			return nil, ParseError{ErrUnexpectedEOF, t}
		}
		if t.Type != TokenString && !(d.ECMAScriptIdentifiers && t.Type == TokenIdentifier) {
			return nil, ParseError{ErrUnexpectedToken, t}
		}

		key, err := d.parseObjectKey()
		if err != nil {
			return nil, err
		}
		name, err := d.objectKeyName(key)
		if err != nil {
			return nil, ParseError{ErrInvalidString, key.Start.Value}
		}
		if _, exists := names[name]; exists {
			return nil, ParseError{ErrDuplicateObjectKey, key.Start.Value}
		}
		names[name] = struct{}{}

		if err := d.consume(); err != nil {
			return nil, err
		}
		t = d.l.peekToken()
		if t.Type == TokenEOF {
			return nil, ParseError{ErrUnexpectedEOF, t}
		}
		if t.Type != TokenColon {
			return nil, ParseError{ErrUnexpectedToken, t}
		}
		_ = d.next()

		value, err := d.parse()
		if err != nil {
			return nil, err
		}
		start := d.tokens.InsertBefore(key.Start, token{Type: TokenAnchor})
		end := d.tokens.InsertAfter(value.End, token{Type: TokenAnchor})
		n.Children = append(n.Children, objectFieldNode(key, value, start, end))

		if err := d.consume(); err != nil {
			return nil, err
		}
		t = d.l.peekToken()
		if t.Type == TokenEOF {
			return nil, ParseError{ErrUnexpectedEOF, t}
		}
		switch t.Type {
		case TokenComma:
			_ = d.next()
			if err := d.consume(); err != nil {
				return nil, err
			}
			t = d.l.peekToken()
			if isRightBrace(t) {
				if !d.TrailingCommas {
					return nil, ParseError{ErrUnexpectedToken, t}
				}
				_ = d.next()
				n.End = d.currentElem
				return n, nil
			}
		case TokenDelim:
			if !isRightBrace(t) {
				return nil, ParseError{ErrUnexpectedToken, t}
			}
			_ = d.next()
			n.End = d.currentElem
			return n, nil
		default:
			return nil, ParseError{ErrUnexpectedToken, t}
		}
	}
}

func (d *Decoder) parseArray() (*node, error) {
	_ = d.next()
	start := d.currentElem

	n := &node{Type: NodeTypeArray, Start: start}
	if err := d.consume(); err != nil {
		return nil, err
	}

	t := d.l.peekToken()
	if isRightBracket(t) {
		_ = d.next()
		n.End = d.currentElem
		return n, nil
	}

	for {
		if t.Type == TokenEOF {
			return nil, ParseError{ErrUnexpectedEOF, t}
		}

		value, err := d.parse()
		if err != nil {
			return nil, err
		}
		start := d.tokens.InsertBefore(value.Start, token{Type: TokenAnchor})
		end := d.tokens.InsertAfter(value.End, token{Type: TokenAnchor})
		n.Children = append(n.Children, arrayElementNode(value, start, end))

		if err := d.consume(); err != nil {
			return nil, err
		}
		t = d.l.peekToken()
		if t.Type == TokenEOF {
			return nil, ParseError{ErrUnexpectedEOF, t}
		}
		switch t.Type {
		case TokenComma:
			_ = d.next()
			if err := d.consume(); err != nil {
				return nil, err
			}
			t = d.l.peekToken()
			if isRightBracket(t) {
				if !d.TrailingCommas {
					return nil, ParseError{ErrUnexpectedToken, t}
				}
				_ = d.next()
				n.End = d.currentElem
				return n, nil
			}
		case TokenDelim:
			if !isRightBracket(t) {
				return nil, ParseError{ErrUnexpectedToken, t}
			}
			_ = d.next()
			n.End = d.currentElem
			return n, nil
		default:
			return nil, ParseError{ErrUnexpectedToken, t}
		}
	}
}

func (d *Decoder) parseScalar(t NodeType) (*node, error) {
	tok := d.l.peekToken()
	switch t {
	case NodeTypeString:
		if err := d.validateString(tok); err != nil {
			return nil, err
		}
	case NodeTypeNumber:
		if err := d.validateNumber(tok); err != nil {
			return nil, err
		}
	}
	_ = d.next()
	i := d.currentElem
	return &node{Type: t, Start: i, End: i}, nil
}

func (d *Decoder) parseObjectKey() (*node, error) {
	t := d.l.peekToken()
	if t.Type == TokenIdentifier {
		if !isJSON5Identifier(t.Literal) {
			return nil, ParseError{ErrUnexpectedToken, t}
		}
		_ = d.next()
		i := d.currentElem
		return &node{Type: NodeTypeString, Start: i, End: i}, nil
	}
	return d.parseScalar(NodeTypeString)
}

func (d *Decoder) objectKeyName(key *node) (string, error) {
	if key.Start.Value.Type == TokenIdentifier {
		return key.Start.Value.Literal, nil
	}
	return unquoteString(key.Start.Value.Literal, d.SyntaxOptions)
}

func (d *Decoder) parseIdentifier() (*node, error) {
	t := d.l.peekToken()
	switch t.Literal {
	case "true", "false":
		return d.parseScalar(NodeTypeBool)
	case "null":
		return d.parseScalar(NodeTypeNull)
	default:
		if validNumberWithOptions(t.Literal, d.SyntaxOptions) {
			return d.parseScalar(NodeTypeNumber)
		}
		return nil, ParseError{ErrUnexpectedToken, t}
	}
}

func (d *Decoder) consume() error {
	for {
		t := d.l.peekToken()
		if t.Type == TokenWhitespace || t.Type == TokenNewline || (t.Type == TokenComment && d.commentAllowed(t)) {
			if (t.Type == TokenWhitespace || t.Type == TokenNewline) && !d.AdditionalWhitespace && !validStrictSpace(t.Literal) {
				return ParseError{ErrInvalidSpace, t}
			}
			_ = d.next()
			continue
		}
		break
	}
	return nil
}

func (d *Decoder) next() token {
	t := d.l.next()
	d.currentElem = d.tokens.PushBack(t)
	return t
}

func (d *Decoder) validateString(t token) error {
	if t.Type != TokenString {
		return ParseError{ErrUnexpectedToken, t}
	}
	if _, err := unquoteStringOptions(t.Literal, stringOptions{
		allowSingleQuote:      d.SingleQuotedStrings,
		allowCharacterEscapes: d.StringCharacterEscapes,
		allowLineContinuation: d.MultilineStrings,
	}); err != nil {
		return ParseError{ErrInvalidString, t}
	}
	return nil
}

func (d *Decoder) validateNumber(t token) error {
	if t.Type != TokenNumber && t.Type != TokenIdentifier {
		return ParseError{ErrUnexpectedToken, t}
	}
	if validNumberWithOptions(t.Literal, d.SyntaxOptions) {
		return nil
	}
	return ParseError{ErrInvalidNumber, t}
}

func (d *Decoder) commentAllowed(t token) bool {
	return commentAllowed(t, d.SyntaxOptions)
}

func (d *Decoder) onlyConsumedTrivia() bool {
	for e := d.tokens.Head; e != nil; e = e.Next {
		switch e.Value.Type {
		case TokenWhitespace, TokenNewline:
			continue
		case TokenComment:
			if d.commentAllowed(e.Value) {
				continue
			}
		}
		return false
	}
	return true
}

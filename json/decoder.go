package json

import (
	"bytes"
	"errors"
	"io"

	"github.com/olimci/roundtrip/internal/list"
	"github.com/olimci/roundtrip/internal/sst"
)

func Unmarshal(data []byte, v any) (*Meta, error) {
	d := NewDecoder(bytes.NewReader(data))
	m, err := d.DecodeMeta()
	if err != nil {
		return nil, err
	}
	return m, decodeInto(m, m.SST.Root, v, decodeOptions{
		useNumber:       d.useNumber,
		disallowUnknown: d.disallowUnknown,
		json5:           m.json5,
	})
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		l: newLexer(r),
	}
}

func NewJSONCDecoder(r io.Reader) *Decoder {
	d := NewDecoder(r)
	d.SetAllowComments(true)
	d.SetAllowTrailingComma(true)
	return d
}

func NewJSON5Decoder(r io.Reader) *Decoder {
	d := NewJSONCDecoder(r)
	d.SetAllowIdentifierKeys(true)
	d.SetAllowSingleQuotedStrings(true)
	d.SetAllowJSON5Numbers(true)
	d.SetAllowJSON5Whitespace(true)
	d.SetAllowStringLineContinuations(true)
	return d
}

type Decoder struct {
	l                            *lexer
	tokens                       *list.List[token]
	currentElem                  *list.Elem[token]
	AllowComments                bool
	AllowTrailingComma           bool
	AllowIdentifierKeys          bool
	AllowSingleQuotedStrings     bool
	AllowJSON5Numbers            bool
	AllowJSON5Whitespace         bool
	AllowStringLineContinuations bool
	useNumber                    bool
	disallowUnknown              bool
}

func (d *Decoder) DecodeMeta() (*Meta, error) {
	return d.decodeMeta(true)
}

func (d *Decoder) decodeMeta(requireEOF bool) (*Meta, error) {
	d.tokens = list.New[token]()
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
		json5:  d.json5(),
		SST: sst.SST[TokenType, NodeType]{
			Tokens: d.tokens,
			Root:   root,
		},
	}, nil
}

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
		json5:           m.json5,
	})
}

func (d *Decoder) UseNumber() {
	d.useNumber = true
}

func (d *Decoder) DisallowUnknownFields() {
	d.disallowUnknown = true
}

func (d *Decoder) SetAllowComments(on bool) {
	d.AllowComments = on
}

func (d *Decoder) SetAllowTrailingComma(on bool) {
	d.AllowTrailingComma = on
}

func (d *Decoder) SetAllowIdentifierKeys(on bool) {
	d.AllowIdentifierKeys = on
}

func (d *Decoder) SetAllowSingleQuotedStrings(on bool) {
	d.AllowSingleQuotedStrings = on
}

func (d *Decoder) SetAllowJSON5Numbers(on bool) {
	d.AllowJSON5Numbers = on
}

func (d *Decoder) SetAllowJSON5Whitespace(on bool) {
	d.AllowJSON5Whitespace = on
}

func (d *Decoder) SetAllowStringLineContinuations(on bool) {
	d.AllowStringLineContinuations = on
}

func (d *Decoder) More() bool {
	for {
		t := d.l.peekToken()
		if t.Type == TokenWhitespace || t.Type == TokenNewline || (t.Type == TokenComment && d.AllowComments) {
			_ = d.l.next()
			continue
		}
		return t.Type != TokenEOF && t.Type != TokenRightBrace && t.Type != TokenRightBracket
	}
}

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
	case TokenLeftBrace:
		return d.parseObject()
	case TokenLeftBracket:
		return d.parseArray()
	case TokenString:
		return d.parseScalar(NodeTypeString)
	case TokenNumber:
		return d.parseScalar(NodeTypeNumber)
	case TokenIdentifier:
		return d.parseIdentifier()
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
	if t.Type == TokenRightBrace {
		_ = d.next()
		n.End = d.currentElem
		return n, nil
	}

	for {
		if t.Type == TokenEOF {
			return nil, ParseError{ErrUnexpectedEOF, t}
		}
		if t.Type != TokenString && !(d.AllowIdentifierKeys && t.Type == TokenIdentifier) {
			return nil, ParseError{ErrUnexpectedToken, t}
		}

		key, err := d.parseObjectKey()
		if err != nil {
			return nil, err
		}

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
		appendObjectField(n, key, value)

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
			if t.Type == TokenRightBrace {
				if !d.AllowTrailingComma {
					return nil, ParseError{ErrUnexpectedToken, t}
				}
				_ = d.next()
				n.End = d.currentElem
				return n, nil
			}
		case TokenRightBrace:
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
	if t.Type == TokenRightBracket {
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
		n.Children = append(n.Children, value)

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
			if t.Type == TokenRightBracket {
				if !d.AllowTrailingComma {
					return nil, ParseError{ErrUnexpectedToken, t}
				}
				_ = d.next()
				n.End = d.currentElem
				return n, nil
			}
		case TokenRightBracket:
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

func (d *Decoder) parseIdentifier() (*node, error) {
	t := d.l.peekToken()
	switch t.Literal {
	case "true", "false":
		return d.parseScalar(NodeTypeBool)
	case "null":
		return d.parseScalar(NodeTypeNull)
	default:
		if d.AllowJSON5Numbers && validJSON5Number(t.Literal) {
			return d.parseScalar(NodeTypeNumber)
		}
		return nil, ParseError{ErrUnexpectedToken, t}
	}
}

func (d *Decoder) consume() error {
	for {
		t := d.l.peekToken()
		if t.Type == TokenWhitespace || t.Type == TokenNewline || (t.Type == TokenComment && d.AllowComments) {
			if (t.Type == TokenWhitespace || t.Type == TokenNewline) && !d.AllowJSON5Whitespace && !validStrictSpace(t.Literal) {
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
		allowSingleQuote:      d.AllowSingleQuotedStrings,
		allowJSON5Escapes:     d.AllowSingleQuotedStrings || d.AllowStringLineContinuations,
		allowLineContinuation: d.AllowStringLineContinuations,
	}); err != nil {
		return ParseError{ErrInvalidString, t}
	}
	return nil
}

func (d *Decoder) validateNumber(t token) error {
	if t.Type != TokenNumber && t.Type != TokenIdentifier {
		return ParseError{ErrUnexpectedToken, t}
	}
	if d.AllowJSON5Numbers {
		if validJSON5Number(t.Literal) {
			return nil
		}
		return ParseError{ErrInvalidNumber, t}
	}
	if validNumber(t.Literal) {
		return nil
	}
	return ParseError{ErrInvalidNumber, t}
}

func (d *Decoder) json5() bool {
	return d.AllowIdentifierKeys ||
		d.AllowSingleQuotedStrings ||
		d.AllowJSON5Numbers ||
		d.AllowJSON5Whitespace ||
		d.AllowStringLineContinuations
}

func (d *Decoder) onlyConsumedTrivia() bool {
	for e := d.tokens.Head; e != nil; e = e.Next {
		switch e.Value.Type {
		case TokenWhitespace, TokenNewline:
			continue
		case TokenComment:
			if d.AllowComments {
				continue
			}
		}
		return false
	}
	return true
}

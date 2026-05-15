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
	})
}

func NewDecoder(r io.Reader) *Decoder {
	return &Decoder{
		l: newLexer(r),
	}
}

type Decoder struct {
	l                  *lexer
	tokens             *list.List[token]
	currentElem        *list.Elem[token]
	AllowComments      bool
	AllowTrailingComma bool
	useNumber          bool
	disallowUnknown    bool
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
		d.consume()
		t := d.l.peekToken()
		if t.Type != TokenEOF {
			return nil, ParseError{ErrUnexpectedToken, t}
		}
	}

	return &Meta{
		Indent: detectIndent(root),
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
	d.consume()

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
	d.consume()

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
		if t.Type != TokenString {
			return nil, ParseError{ErrUnexpectedToken, t}
		}

		key, err := d.parseScalar(NodeTypeString)
		if err != nil {
			return nil, err
		}

		d.consume()
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

		d.consume()
		t = d.l.peekToken()
		if t.Type == TokenEOF {
			return nil, ParseError{ErrUnexpectedEOF, t}
		}
		switch t.Type {
		case TokenComma:
			_ = d.next()
			d.consume()
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
	d.consume()

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

		d.consume()
		t = d.l.peekToken()
		if t.Type == TokenEOF {
			return nil, ParseError{ErrUnexpectedEOF, t}
		}
		switch t.Type {
		case TokenComma:
			_ = d.next()
			d.consume()
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
	_ = d.next()
	i := d.currentElem
	return &node{Type: t, Start: i, End: i}, nil
}

func (d *Decoder) parseIdentifier() (*node, error) {
	t := d.l.peekToken()
	switch t.Literal {
	case "true", "false":
		return d.parseScalar(NodeTypeBool)
	case "null":
		return d.parseScalar(NodeTypeNull)
	default:
		return nil, ParseError{ErrUnexpectedToken, t}
	}
}

func (d *Decoder) consume() {
	for {
		t := d.l.peekToken()
		if t.Type == TokenWhitespace || t.Type == TokenNewline || (t.Type == TokenComment && d.AllowComments) {
			_ = d.next()
			continue
		}
		break
	}
}

func (d *Decoder) next() token {
	t := d.l.next()
	d.currentElem = d.tokens.PushBack(t)
	return t
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

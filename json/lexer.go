package json

import (
	"bufio"
	"bytes"
	"io"
	"iter"
	"slices"
	"unicode"

	"github.com/olimci/roundtrip/internal/cursor"
	"github.com/olimci/roundtrip/internal/list"
)

func newLexer(r io.Reader) *lexer {
	return &lexer{
		reader:  bufio.NewReader(r),
		list:    list.New[token](),
		cursor:  cursor.NewCursor(),
		collect: true,
	}
}

// todo: is it worth making this streaming?
func lex(data []byte) iter.Seq[token] {
	return func(yield func(token) bool) {
		l := newLexer(bytes.NewReader(data))
		l.collect = false
		for {
			t := l.next()
			if t.Type == TokenEOF {
				break
			}
			if !yield(t) {
				return
			}
		}
	}
}

type lexer struct {
	reader *bufio.Reader
	peek   *token

	list        *list.List[token]
	currentElem *list.Elem[token]
	collect     bool

	cursor *cursor.Cursor
}

func (l *lexer) next() token {
	t := l.readToken()
	if l.collect {
		l.currentElem = l.list.PushBack(t)
	}
	return t
}

func (l *lexer) peekToken() token {
	if l.peek == nil {
		t := l.readToken()
		l.peek = &t
	}
	return *l.peek
}

func (l *lexer) readToken() token {
	if l.peek != nil {
		t := *l.peek
		l.peek = nil
		return t
	}

	pos := l.cursor.Position
	t := l.scanToken()
	t.Position = pos

	return t
}

func (l *lexer) assert(s string) bool {
	bs, err := l.reader.Peek(len(s))
	if err != nil {
		return false
	}
	return slices.Equal(bs, []byte(s))
}

func (l *lexer) scanToken() token {
	r, err := l.peekRune()
	if err != nil {
		return token{Type: TokenEOF}
	}

	switch r {
	case ':':
		return l.consumeSingle(TokenColon)
	case ',':
		return l.consumeSingle(TokenComma)
	case '{':
		return l.consumeSingle(TokenLeftBrace)
	case '}':
		return l.consumeSingle(TokenRightBrace)
	case '[':
		return l.consumeSingle(TokenLeftBracket)
	case ']':
		return l.consumeSingle(TokenRightBracket)
	case '/':
		if l.assert("//") {
			return l.consumeComment()
		}
		if l.assert("/*") {
			return l.consumeMultiLineComment()
		}
	case '"':
		return l.consumeString()
	case '-':
		return l.consumeNumber()
	case '\n', '\r':
		return l.consumeNewline()
	}

	switch {
	case isHorizontalSpace(r):
		return l.consumeWhitespace()
	case unicode.IsNumber(r):
		return l.consumeNumber()
	case unicode.IsLetter(r):
		return l.consumeIdentifier()
	}

	r, _ = l.readRune()
	return token{Type: TokenIllegal, Literal: string(r)}
}

func (l *lexer) consumeSingle(typ TokenType) token {
	r, _ := l.readRune()
	return token{Type: typ, Literal: string(r)}
}

func (l *lexer) peekRune() (rune, error) {
	r, err := l.readRune()
	if err != nil {
		return r, err
	}

	if err := l.unreadRune(); err != nil {
		return 0, err
	}

	return r, nil
}

func (l *lexer) consumeWhitespace() token {
	lit := ""

	for {
		r, err := l.readRune()
		if err != nil {
			if lit != "" {
				return token{Type: TokenWhitespace, Literal: lit}
			}
			return token{Type: TokenEOF}
		}

		if !isHorizontalSpace(r) {
			_ = l.unreadRune()
			break
		}

		lit += string(r)
	}

	return token{Type: TokenWhitespace, Literal: lit}
}

func (l *lexer) consumeNewline() token {
	lit := ""

	for {
		r, err := l.readRune()
		if err != nil {
			if lit != "" {
				return token{Type: TokenNewline, Literal: lit}
			}
			return token{Type: TokenEOF}
		}

		if r != '\n' && r != '\r' {
			_ = l.unreadRune()
			break
		}

		lit += string(r)
	}

	return token{Type: TokenNewline, Literal: lit}
}

func (l *lexer) consumeComment() token {
	lit := ""

	for {
		r, err := l.readRune()
		if err != nil {
			return token{Type: TokenComment, Literal: lit}
		}

		if r == '\n' || r == '\r' {
			_ = l.unreadRune()
			break
		}

		lit += string(r)
	}

	return token{Type: TokenComment, Literal: lit}
}

func (l *lexer) consumeMultiLineComment() token {
	lit := ""
	var prev rune

	for {
		r, err := l.readRune()
		if err != nil {
			return token{Type: TokenIllegal, Literal: lit}
		}

		lit += string(r)

		if prev == '*' && r == '/' {
			return token{Type: TokenComment, Literal: lit}
		}

		prev = r
	}
}

func (l *lexer) consumeString() token {
	lit := ""

	for {
		r, err := l.readRune()
		if err != nil {
			return token{Type: TokenIllegal, Literal: lit}
		}

		lit += string(r)

		if r == '\\' {
			r, err := l.readRune()
			if err != nil {
				return token{Type: TokenIllegal, Literal: lit}
			}
			lit += string(r)

			switch r {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't':
			case 'u':
				for range 4 {
					r, err := l.readRune()
					if err != nil {
						return token{Type: TokenIllegal, Literal: lit}
					}
					lit += string(r)
					if !isHex(r) {
						return token{Type: TokenIllegal, Literal: lit}
					}
				}
			default:
				return token{Type: TokenIllegal, Literal: lit}
			}
			continue
		}

		if r < 0x20 {
			return token{Type: TokenIllegal, Literal: lit}
		}

		if r == '"' && len(lit) > 1 {
			return token{Type: TokenString, Literal: lit}
		}
	}
}

func (l *lexer) consumeNumber() token {
	lit := ""

	for {
		r, err := l.readRune()
		if err != nil {
			if validNumber(lit) {
				return token{Type: TokenNumber, Literal: lit}
			}
			return token{Type: TokenIllegal, Literal: lit}
		}

		if isNumberDelimiter(r) {
			_ = l.unreadRune()
			if validNumber(lit) {
				return token{Type: TokenNumber, Literal: lit}
			}
			return token{Type: TokenIllegal, Literal: lit}
		}

		lit += string(r)
	}
}

func (l *lexer) consumeIdentifier() token {
	lit := ""

	for {
		r, err := l.readRune()
		if err != nil {
			return token{Type: TokenIdentifier, Literal: lit}
		}

		if !unicode.IsLetter(r) && !unicode.IsNumber(r) {
			_ = l.unreadRune()
			return token{Type: TokenIdentifier, Literal: lit}
		}

		lit += string(r)
	}
}

func (l *lexer) readRune() (rune, error) {
	r, size, err := l.reader.ReadRune()
	if err != nil {
		return r, err
	}

	l.cursor.Next(r, size)

	return r, nil
}

func (l *lexer) unreadRune() error {
	if err := l.reader.UnreadRune(); err != nil {
		return err
	}

	l.cursor.Prev()

	return nil
}

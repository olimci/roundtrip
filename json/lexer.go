package json

import (
	"bufio"
	"bytes"
	"io"
	"iter"
	"slices"
	"unicode"

	"github.com/olimci/roundtrip/internal/cursor"
)

func newLexer(r io.Reader) *lexer {
	return &lexer{
		reader: bufio.NewReader(r),
		cursor: cursor.NewCursor(),
	}
}

// todo: is it worth making this streaming?
func lex(data []byte) iter.Seq[token] {
	return func(yield func(token) bool) {
		l := newLexer(bytes.NewReader(data))
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

	cursor *cursor.Cursor
}

func (l *lexer) next() token {
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

func (l *lexer) peekToken() token {
	if l.peek == nil {
		t := l.next()
		l.peek = &t
	}
	return *l.peek
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
	case '"', '\'':
		return l.consumeString(r)
	case '-', '+', '.':
		return l.consumeNumber()
	case '\n', '\r', '\u2028', '\u2029':
		return l.consumeNewline()
	}

	switch {
	case isHorizontalSpace(r):
		return l.consumeWhitespace()
	case unicode.IsNumber(r):
		return l.consumeNumber()
	case unicode.IsLetter(r) || r == '_' || r == '$':
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

		if !isNewline(r) {
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

func (l *lexer) consumeString(quote rune) token {
	lit := ""

	for {
		r, err := l.readRune()
		if err != nil {
			return token{Type: TokenString, Literal: lit}
		}

		lit += string(r)

		if r == '\\' {
			r, err := l.readRune()
			if err != nil {
				return token{Type: TokenString, Literal: lit}
			}
			lit += string(r)
			continue
		}

		if r == quote && len(lit) > 1 {
			return token{Type: TokenString, Literal: lit}
		}
	}
}

func (l *lexer) consumeNumber() token {
	lit := ""

	for {
		r, err := l.readRune()
		if err != nil {
			return token{Type: TokenNumber, Literal: lit}
		}

		if isNumberDelimiter(r) {
			_ = l.unreadRune()
			return token{Type: TokenNumber, Literal: lit}
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

		if !unicode.IsLetter(r) && !unicode.IsNumber(r) && r != '_' && r != '$' {
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

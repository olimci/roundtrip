package json

import (
	"strings"
	"testing"

	"github.com/olimci/roundtrip/internal/cursor"
)

func TestLexerTokens(t *testing.T) {
	input := "{\n\t// c\n\t'a': +.5,\n\tb: /* block */ [true, null]\n}"
	want := []token{
		{Type: TokenDelim, Literal: "{", Position: cursor.Position{Line: 1, Column: 1, Offset: 0}},
		{Type: TokenNewline, Literal: "\n", Position: cursor.Position{Line: 1, Column: 2, Offset: 1}},
		{Type: TokenWhitespace, Literal: "\t", Position: cursor.Position{Line: 2, Column: 1, Offset: 2}},
		{Type: TokenComment, Literal: "// c", Position: cursor.Position{Line: 2, Column: 2, Offset: 3}},
		{Type: TokenNewline, Literal: "\n", Position: cursor.Position{Line: 2, Column: 6, Offset: 7}},
		{Type: TokenWhitespace, Literal: "\t", Position: cursor.Position{Line: 3, Column: 1, Offset: 8}},
		{Type: TokenString, Literal: "'a'", Position: cursor.Position{Line: 3, Column: 2, Offset: 9}},
		{Type: TokenColon, Literal: ":", Position: cursor.Position{Line: 3, Column: 5, Offset: 12}},
		{Type: TokenWhitespace, Literal: " ", Position: cursor.Position{Line: 3, Column: 6, Offset: 13}},
		{Type: TokenNumber, Literal: "+.5", Position: cursor.Position{Line: 3, Column: 7, Offset: 14}},
		{Type: TokenComma, Literal: ",", Position: cursor.Position{Line: 3, Column: 10, Offset: 17}},
		{Type: TokenNewline, Literal: "\n", Position: cursor.Position{Line: 3, Column: 11, Offset: 18}},
		{Type: TokenWhitespace, Literal: "\t", Position: cursor.Position{Line: 4, Column: 1, Offset: 19}},
		{Type: TokenIdentifier, Literal: "b", Position: cursor.Position{Line: 4, Column: 2, Offset: 20}},
		{Type: TokenColon, Literal: ":", Position: cursor.Position{Line: 4, Column: 3, Offset: 21}},
		{Type: TokenWhitespace, Literal: " ", Position: cursor.Position{Line: 4, Column: 4, Offset: 22}},
		{Type: TokenComment, Literal: "/* block */", Position: cursor.Position{Line: 4, Column: 5, Offset: 23}},
		{Type: TokenWhitespace, Literal: " ", Position: cursor.Position{Line: 4, Column: 16, Offset: 34}},
		{Type: TokenDelim, Literal: "[", Position: cursor.Position{Line: 4, Column: 17, Offset: 35}},
		{Type: TokenIdentifier, Literal: "true", Position: cursor.Position{Line: 4, Column: 18, Offset: 36}},
		{Type: TokenComma, Literal: ",", Position: cursor.Position{Line: 4, Column: 22, Offset: 40}},
		{Type: TokenWhitespace, Literal: " ", Position: cursor.Position{Line: 4, Column: 23, Offset: 41}},
		{Type: TokenIdentifier, Literal: "null", Position: cursor.Position{Line: 4, Column: 24, Offset: 42}},
		{Type: TokenDelim, Literal: "]", Position: cursor.Position{Line: 4, Column: 28, Offset: 46}},
		{Type: TokenNewline, Literal: "\n", Position: cursor.Position{Line: 4, Column: 29, Offset: 47}},
		{Type: TokenDelim, Literal: "}", Position: cursor.Position{Line: 5, Column: 1, Offset: 48}},
	}

	var got []token
	var joined strings.Builder
	for tok := range lex([]byte(input)) {
		got = append(got, tok)
		joined.WriteString(tok.Literal)
	}

	if joined.String() != input {
		t.Fatalf("token literals reconstructed %q, want %q", joined.String(), input)
	}
	if len(got) != len(want) {
		t.Fatalf("token count = %d, want %d:\n%#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("token %d = %#v, want %#v", i, got[i], want[i])
		}
	}
}

func TestLexerMalformedTokens(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want token
	}{
		{
			name: "unterminated block comment",
			in:   "/* open",
			want: token{Type: TokenIllegal, Literal: "/* open", Position: cursor.Position{Line: 1, Column: 1}},
		},
		{
			name: "unknown rune",
			in:   "@",
			want: token{Type: TokenIllegal, Literal: "@", Position: cursor.Position{Line: 1, Column: 1}},
		},
		{
			name: "unterminated string remains string token",
			in:   `"open`,
			want: token{Type: TokenString, Literal: `"open`, Position: cursor.Position{Line: 1, Column: 1}},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l := newLexer(strings.NewReader(tc.in))
			if got := l.next(); got != tc.want {
				t.Fatalf("next() = %#v, want %#v", got, tc.want)
			}
			if got := l.next(); got.Type != TokenEOF {
				t.Fatalf("second next() type = %v, want EOF", got.Type)
			}
		})
	}
}

func TestLexerPeekTokenDoesNotConsume(t *testing.T) {
	l := newLexer(strings.NewReader(`{"a":1}`))

	peeked := l.peekToken()
	next := l.next()
	if next != peeked {
		t.Fatalf("next() after peek = %#v, want %#v", next, peeked)
	}
	if next.Type != TokenDelim || next.Literal != "{" {
		t.Fatalf("first token = %#v, want left brace delimiter", next)
	}
}

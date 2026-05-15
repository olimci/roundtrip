package json

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

func Valid(data []byte) bool {
	_, err := NewDecoder(bytes.NewReader(data)).DecodeMeta()
	return err == nil
}

func Compact(dst *bytes.Buffer, src []byte) error {
	m, err := decodeFormatMeta(src)
	if err != nil {
		return err
	}
	for t := range m.SST.Tokens.Values() {
		switch t.Type {
		case TokenWhitespace, TokenNewline:
			continue
		case TokenComment:
			dst.WriteString(t.Literal)
			if strings.HasPrefix(t.Literal, "//") {
				dst.WriteByte('\n')
			}
		default:
			dst.WriteString(t.Literal)
		}
	}
	return nil
}

func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	m, err := decodeFormatMeta(src)
	if err != nil {
		return err
	}

	tokens := formatTokens(m)
	for i, t := range tokens {
		switch t.Type {
		case TokenLeftBrace, TokenLeftBracket:
			dst.WriteString(t.Literal)
			if next, ok := nextFormatToken(tokens, i); ok && !isClosingToken(next.Type) {
				writeIndentNewline(dst, prefix, indent, formatDepth(tokens, i)+1)
			}
		case TokenRightBrace, TokenRightBracket:
			if prev, ok := prevFormatToken(tokens, i); ok && !isOpeningToken(prev.Type) {
				writeIndentNewline(dst, prefix, indent, formatDepth(tokens, i)-1)
			}
			dst.WriteString(t.Literal)
		case TokenColon:
			dst.WriteString(": ")
		case TokenComma:
			dst.WriteByte(',')
			if next, ok := nextFormatToken(tokens, i); ok && next.Type == TokenComment {
				dst.WriteByte(' ')
			} else if ok && isClosingToken(next.Type) {
				continue
			} else {
				writeIndentNewline(dst, prefix, indent, formatDepth(tokens, i))
			}
		case TokenComment:
			dst.WriteString(t.Literal)
			if strings.HasPrefix(t.Literal, "//") {
				writeIndentNewline(dst, prefix, indent, formatDepth(tokens, i))
			}
		default:
			dst.WriteString(t.Literal)
		}
	}
	dst.Write(trailingJSONSpace(src))
	return nil
}

func HTMLEscape(dst *bytes.Buffer, src []byte) {
	inString := false
	start := 0
	for i := 0; i < len(src); {
		b := src[i]
		if !inString {
			if b == '"' {
				inString = true
			}
			i++
			continue
		}
		if b == '\\' {
			i += 2
			continue
		}
		if b == '"' {
			inString = false
			i++
			continue
		}
		if b < utf8.RuneSelf {
			if b != '<' && b != '>' && b != '&' {
				i++
				continue
			}
			dst.Write(src[start:i])
			switch b {
			case '<':
				dst.WriteString(`\u003c`)
			case '>':
				dst.WriteString(`\u003e`)
			case '&':
				dst.WriteString(`\u0026`)
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRune(src[i:])
		if r == '\u2028' || r == '\u2029' {
			dst.Write(src[start:i])
			if r == '\u2028' {
				dst.WriteString(`\u2028`)
			} else {
				dst.WriteString(`\u2029`)
			}
			i += size
			start = i
			continue
		}
		i += size
	}
	dst.Write(src[start:])
}

func decodeFormatMeta(src []byte) (*Meta, error) {
	d := NewDecoder(bytes.NewReader(src))
	d.AllowComments = true
	d.AllowTrailingComma = true
	return d.DecodeMeta()
}

func formatTokens(m *Meta) []token {
	tokens := []token{}
	for t := range m.SST.Tokens.Values() {
		switch t.Type {
		case TokenWhitespace, TokenNewline, TokenEOF:
			continue
		default:
			tokens = append(tokens, t)
		}
	}
	return tokens
}

func formatDepth(tokens []token, index int) int {
	depth := 0
	for _, t := range tokens[:index] {
		switch t.Type {
		case TokenLeftBrace, TokenLeftBracket:
			depth++
		case TokenRightBrace, TokenRightBracket:
			depth--
		}
	}
	return depth
}

func nextFormatToken(tokens []token, index int) (token, bool) {
	if index+1 >= len(tokens) {
		return token{}, false
	}
	return tokens[index+1], true
}

func prevFormatToken(tokens []token, index int) (token, bool) {
	if index == 0 {
		return token{}, false
	}
	return tokens[index-1], true
}

func isOpeningToken(t TokenType) bool {
	return t == TokenLeftBrace || t == TokenLeftBracket
}

func isClosingToken(t TokenType) bool {
	return t == TokenRightBrace || t == TokenRightBracket
}

func writeIndentNewline(dst *bytes.Buffer, prefix, indent string, depth int) {
	dst.WriteByte('\n')
	dst.WriteString(prefix)
	dst.WriteString(strings.Repeat(indent, depth))
}

func trailingJSONSpace(src []byte) []byte {
	i := len(src)
	for i > 0 {
		switch src[i-1] {
		case ' ', '\t', '\r', '\n':
			i--
		default:
			return src[i:]
		}
	}
	return src
}

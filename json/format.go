package json

import (
	"bytes"
	"strings"
)

// Valid reports whether data is a complete strict JSON value.
func Valid(data []byte) bool {
	return ValidWithOptions(data, SyntaxOptions{})
}

// ValidWithOptions reports whether data is a complete JSON value for opts.
func ValidWithOptions(data []byte, opts SyntaxOptions) bool {
	d := NewDecoder(bytes.NewReader(data))
	d.SyntaxOptions = opts
	_, err := d.DecodeMeta()
	return err == nil
}

// Compact appends src to dst with insignificant whitespace removed.
//
// dst must be non-nil. Compact accepts strict JSON.
func Compact(dst *bytes.Buffer, src []byte) error {
	return CompactWithOptions(dst, src, SyntaxOptions{})
}

// CompactWithOptions appends src to dst with insignificant whitespace removed.
//
// dst must be non-nil. CompactWithOptions accepts syntax enabled by opts and
// preserves comments.
func CompactWithOptions(dst *bytes.Buffer, src []byte, opts SyntaxOptions) error {
	m, err := decodeMetaWithOptions(src, opts)
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

// Indent appends an indented form of src to dst.
//
// dst must be non-nil. Indent accepts strict JSON.
func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error {
	return IndentWithOptions(dst, src, prefix, indent, SyntaxOptions{})
}

// IndentWithOptions appends an indented form of src to dst.
//
// dst must be non-nil. IndentWithOptions accepts syntax enabled by opts and
// preserves comments.
func IndentWithOptions(dst *bytes.Buffer, src []byte, prefix, indent string, opts SyntaxOptions) error {
	m, err := decodeMetaWithOptions(src, opts)
	if err != nil {
		return err
	}

	tokens := formatTokens(m)
	for i, t := range tokens {
		switch t.Type {
		case TokenDelim:
			if isOpenDelim(t) {
				dst.WriteString(t.Literal)
				if i+1 < len(tokens) && !isCloseDelim(tokens[i+1]) {
					writeIndentNewline(dst, prefix, indent, formatDepth(tokens, i)+1)
				}
			} else if isCloseDelim(t) {
				if i > 0 && !isOpenDelim(tokens[i-1]) {
					writeIndentNewline(dst, prefix, indent, formatDepth(tokens, i)-1)
				}
				dst.WriteString(t.Literal)
			} else {
				dst.WriteString(t.Literal)
			}
		case TokenColon:
			dst.WriteString(": ")
		case TokenComma:
			dst.WriteByte(',')
			if i+1 < len(tokens) && tokens[i+1].Type == TokenComment {
				dst.WriteByte(' ')
			} else if i+1 < len(tokens) && isCloseDelim(tokens[i+1]) {
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

func decodeMetaWithOptions(src []byte, opts SyntaxOptions) (*Meta, error) {
	d := NewDecoder(bytes.NewReader(src))
	d.SyntaxOptions = opts
	return d.DecodeMeta()
}

// HTMLEscape appends src to dst with HTML-significant characters escaped inside
// JSON strings.
//
// dst must be non-nil.
func HTMLEscape(dst *bytes.Buffer, src []byte) {
	dst.Grow(len(src))
	dst.Write(appendHTMLEscape(dst.AvailableBuffer(), src))
}

func appendHTMLEscape(dst, src []byte) []byte {
	const hex = "0123456789abcdef"

	start := 0
	for i, c := range src {
		if c == '<' || c == '>' || c == '&' {
			dst = append(dst, src[start:i]...)
			dst = append(dst, '\\', 'u', '0', '0', hex[c>>4], hex[c&0xf])
			start = i + 1
		}
		if c == 0xe2 && i+2 < len(src) && src[i+1] == 0x80 && src[i+2]&^1 == 0xa8 {
			dst = append(dst, src[start:i]...)
			dst = append(dst, '\\', 'u', '2', '0', '2', hex[src[i+2]&0xf])
			start = i + len("\u2029")
		}
	}
	return append(dst, src[start:]...)
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
		switch {
		case isOpenDelim(t):
			depth++
		case isCloseDelim(t):
			depth--
		}
	}
	return depth
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

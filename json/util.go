package json

import "unicode"

func isHorizontalSpace(r rune) bool {
	return unicode.IsSpace(r) && !isNewline(r)
}

func isStrictHorizontalSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func isNewline(r rune) bool {
	return r == '\n' || r == '\r' || r == '\u2028' || r == '\u2029'
}

func isStrictNewline(r rune) bool {
	return r == '\n' || r == '\r'
}

func validStrictSpace(s string) bool {
	for _, r := range s {
		if !isStrictHorizontalSpace(r) && !isStrictNewline(r) {
			return false
		}
	}
	return true
}

func isNumberDelimiter(r rune) bool {
	switch r {
	case ',', ']', '}', ':', '[', '{', '/':
		return true
	default:
		return isHorizontalSpace(r) || isNewline(r)
	}
}

func isLeftBrace(t token) bool {
	return t.Type == TokenDelim && t.Literal == "{"
}

func isRightBrace(t token) bool {
	return t.Type == TokenDelim && t.Literal == "}"
}

func isLeftBracket(t token) bool {
	return t.Type == TokenDelim && t.Literal == "["
}

func isRightBracket(t token) bool {
	return t.Type == TokenDelim && t.Literal == "]"
}

func isOpenDelim(t token) bool {
	return isLeftBrace(t) || isLeftBracket(t)
}

func isCloseDelim(t token) bool {
	return isRightBrace(t) || isRightBracket(t)
}

func validNumber(s string) bool {
	if s == "" {
		return false
	}

	i := 0
	if s[i] == '-' {
		i++
		if i == len(s) {
			return false
		}
	}

	switch {
	case s[i] == '0':
		i++
	case '1' <= s[i] && s[i] <= '9':
		for i < len(s) && '0' <= s[i] && s[i] <= '9' {
			i++
		}
	default:
		return false
	}

	if i < len(s) && s[i] == '.' {
		i++
		if i == len(s) || s[i] < '0' || s[i] > '9' {
			return false
		}
		for i < len(s) && '0' <= s[i] && s[i] <= '9' {
			i++
		}
	}

	if i < len(s) && (s[i] == 'e' || s[i] == 'E') {
		i++
		if i < len(s) && (s[i] == '+' || s[i] == '-') {
			i++
		}
		if i == len(s) || s[i] < '0' || s[i] > '9' {
			return false
		}
		for i < len(s) && '0' <= s[i] && s[i] <= '9' {
			i++
		}
	}

	if i != len(s) {
		return false
	}

	return true
}

func isHex(r rune) bool {
	return ('0' <= r && r <= '9') ||
		('a' <= r && r <= 'f') ||
		('A' <= r && r <= 'F')
}

func isJSON5Identifier(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if i == 0 {
			if r == '$' || r == '_' || unicode.IsLetter(r) {
				continue
			}
			return false
		}
		if r == '$' || r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) {
			continue
		}
		return false
	}
	return true
}

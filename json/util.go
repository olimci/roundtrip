package json

func isHorizontalSpace(r rune) bool {
	return r == ' ' || r == '\t'
}

func isNumberDelimiter(r rune) bool {
	switch r {
	case ',', ']', '}', ':', '[', '{', '/':
		return true
	default:
		return r == ' ' || r == '\t' || r == '\n' || r == '\r'
	}
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

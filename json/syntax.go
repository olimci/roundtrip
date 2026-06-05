package json

import "strings"

// SyntaxOptions controls optional JSON syntax extensions accepted by the
// decoder and, where applicable, emitted by the encoder.
type SyntaxOptions struct {
	// ECMAScriptIdentifiers allows unquoted object keys that are valid
	// ECMAScript identifiers.
	ECMAScriptIdentifiers bool
	// TrailingCommas allows a comma after the final array element or object field.
	TrailingCommas bool
	// SingleQuotedStrings allows strings quoted with single quotes.
	SingleQuotedStrings bool
	// MultilineStrings allows line continuations inside string literals.
	MultilineStrings bool
	// StringCharacterEscapes allows JSON5 character escapes that strict JSON
	// does not permit.
	StringCharacterEscapes bool
	// HexadecimalNumbers allows hexadecimal number literals.
	HexadecimalNumbers bool
	// LeadingOrTrailingDecimalPoints allows number literals such as .1 and 1.
	LeadingOrTrailingDecimalPoints bool
	// LeadingPlusSigns allows a leading plus sign on number literals.
	LeadingPlusSigns bool
	// IEEE754Numbers allows Infinity, -Infinity, and NaN number literals.
	IEEE754Numbers bool
	// SingleLineComments allows // comments.
	SingleLineComments bool
	// MultilineComments allows block comments.
	MultilineComments bool
	// AdditionalWhitespace allows JSON5 whitespace beyond strict JSON's space,
	// tab, carriage return, and line feed.
	AdditionalWhitespace bool
}

// JSONCSyntaxOptions returns the syntax extensions commonly used by JSONC.
func JSONCSyntaxOptions() SyntaxOptions {
	return SyntaxOptions{
		TrailingCommas:     true,
		SingleLineComments: true,
		MultilineComments:  true,
	}
}

// JSON5SyntaxOptions returns the full set of JSON5 syntax extensions.
func JSON5SyntaxOptions() SyntaxOptions {
	opts := JSONCSyntaxOptions()
	opts.ECMAScriptIdentifiers = true
	opts.SingleQuotedStrings = true
	opts.MultilineStrings = true
	opts.StringCharacterEscapes = true
	opts.HexadecimalNumbers = true
	opts.LeadingOrTrailingDecimalPoints = true
	opts.LeadingPlusSigns = true
	opts.IEEE754Numbers = true
	opts.AdditionalWhitespace = true
	return opts
}

func (opts SyntaxOptions) strictNumbers() bool {
	return !opts.HexadecimalNumbers &&
		!opts.LeadingOrTrailingDecimalPoints &&
		!opts.LeadingPlusSigns &&
		!opts.IEEE754Numbers
}

func commentAllowed(t token, opts SyntaxOptions) bool {
	if strings.HasPrefix(t.Literal, "//") {
		return opts.SingleLineComments
	}
	if strings.HasPrefix(t.Literal, "/*") {
		return opts.MultilineComments
	}
	return false
}

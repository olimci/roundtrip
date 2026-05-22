package json

import "strings"

// SyntaxOptions controls optional JSON syntax extensions accepted by the
// decoder and, where applicable, emitted by the encoder.
type SyntaxOptions struct {
	ECMAScriptIdentifiers          bool
	TrailingCommas                 bool
	SingleQuotedStrings            bool
	MultilineStrings               bool
	StringCharacterEscapes         bool
	HexadecimalNumbers             bool
	LeadingOrTrailingDecimalPoints bool
	LeadingPlusSigns               bool
	IEEE754Numbers                 bool
	SingleLineComments             bool
	MultilineComments              bool
	AdditionalWhitespace           bool
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

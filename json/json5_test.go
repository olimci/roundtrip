package json

import (
	"bytes"
	"math"
	"strings"
	"testing"
)

func TestJSON5Decoder(t *testing.T) {
	input := `{
		identifier: 'line\
continued',
		$hex: 0x10,
		_float: +.5,
		trailing: [1, 2,],
		escaped: '\x41\v\0',
		nan: NaN,
		inf: -Infinity,
	}`

	var got struct {
		Identifier string  `json:"identifier"`
		Hex        int     `json:"$hex"`
		Float      float64 `json:"_float"`
		Trailing   []int   `json:"trailing"`
		Escaped    string  `json:"escaped"`
		NaN        float64 `json:"nan"`
		Inf        float64 `json:"inf"`
	}

	if _, err := NewJSON5Decoder(strings.NewReader(input)).Decode(&got); err != nil {
		t.Fatalf("Decode JSON5: %v", err)
	}
	if got.Identifier != "linecontinued" {
		t.Fatalf("Identifier = %q", got.Identifier)
	}
	if got.Hex != 16 {
		t.Fatalf("Hex = %d", got.Hex)
	}
	if got.Float != 0.5 {
		t.Fatalf("Float = %v", got.Float)
	}
	if len(got.Trailing) != 2 || got.Trailing[0] != 1 || got.Trailing[1] != 2 {
		t.Fatalf("Trailing = %#v", got.Trailing)
	}
	if got.Escaped != "A\v\x00" {
		t.Fatalf("Escaped = %q", got.Escaped)
	}
	if !math.IsNaN(got.NaN) {
		t.Fatalf("NaN = %v", got.NaN)
	}
	if !math.IsInf(got.Inf, -1) {
		t.Fatalf("Inf = %v", got.Inf)
	}
}

func TestJSON5UseNumberPreservesLiteral(t *testing.T) {
	d := NewJSON5Decoder(strings.NewReader(`{hex:0x10, plus:+1}`))
	d.UseNumber()

	var got map[string]any
	if _, err := d.Decode(&got); err != nil {
		t.Fatalf("Decode JSON5 numbers: %v", err)
	}
	if got["hex"] != Number("0x10") {
		t.Fatalf("hex = %#v", got["hex"])
	}
	if got["plus"] != Number("+1") {
		t.Fatalf("plus = %#v", got["plus"])
	}
}

func TestJSON5EncoderUnquotedKeys(t *testing.T) {
	var b bytes.Buffer
	e := NewJSON5Encoder(&b)
	if err := e.Encode(map[string]any{
		"alpha":      1,
		"$beta":      2,
		"needs dash": 3,
	}); err != nil {
		t.Fatalf("Encode JSON5: %v", err)
	}

	got := b.String()
	if !strings.Contains(got, `alpha:`) {
		t.Fatalf("missing unquoted alpha key: %s", got)
	}
	if !strings.Contains(got, `$beta:`) {
		t.Fatalf("missing unquoted $beta key: %s", got)
	}
	if !strings.Contains(got, `"needs dash":`) {
		t.Fatalf("missing quoted fallback key: %s", got)
	}
}

func TestStrictDecoderRejectsJSON5Syntax(t *testing.T) {
	tests := []string{
		`{key:1}`,
		`{'key':1}`,
		`{"key":0x10}`,
		"{\"key\":1\v}",
	}

	for _, input := range tests {
		if _, err := NewDecoder(strings.NewReader(input)).DecodeMeta(); err == nil {
			t.Fatalf("strict DecodeMeta accepted %q", input)
		}
		if Valid([]byte(input)) {
			t.Fatalf("Valid accepted %q", input)
		}
	}
}

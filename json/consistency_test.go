package json

import (
	"bytes"
	"errors"
	"math"
	"strings"
	"testing"
)

func TestFormatHelpersUseExplicitSyntaxOptions(t *testing.T) {
	input := []byte("{// comment\n\"a\": [1,],\n}")

	if Valid(input) {
		t.Fatal("Valid accepted JSONC")
	}
	jsonc := JSONCSyntaxOptions()
	if !ValidWithOptions(input, jsonc) {
		t.Fatal("ValidWithOptions rejected JSONC")
	}

	var compacted bytes.Buffer
	if err := CompactWithOptions(&compacted, input, jsonc); err != nil {
		t.Fatalf("CompactWithOptions rejected JSONC: %v", err)
	}
	if !strings.Contains(compacted.String(), "// comment\n") {
		t.Fatalf("CompactWithOptions did not preserve line comment: %q", compacted.String())
	}

	var indented bytes.Buffer
	if err := IndentWithOptions(&indented, input, "", "  ", jsonc); err != nil {
		t.Fatalf("IndentWithOptions rejected JSONC: %v", err)
	}
}

func TestDuplicateObjectKeysRejected(t *testing.T) {
	tests := []struct {
		name  string
		input string
		opts  SyntaxOptions
	}{
		{name: "strict", input: `{"a":1,"a":2}`},
		{name: "nested", input: `{"outer":{"a":1,"a":2}}`},
		{name: "json5 identifier collision", input: `{a:1,"a":2}`, opts: JSON5SyntaxOptions()},
		{name: "json5 string collision", input: `{'a':1,"a":2}`, opts: JSON5SyntaxOptions()},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := NewDecoder(strings.NewReader(tt.input))
			d.SyntaxOptions = tt.opts
			_, err := d.DecodeMeta()
			if !errors.Is(err, ErrDuplicateObjectKey) {
				t.Fatalf("DecodeMeta error = %v", err)
			}
		})
	}
}

func TestObjectEditsRejectDuplicateKeys(t *testing.T) {
	m, err := NewDecoder(strings.NewReader(`{"a":1,"b":2}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}
	root := m.Root()

	if err := root.InsertObjectField("a", 3); !errors.Is(err, ErrObjectFieldExists) {
		t.Fatalf("InsertObjectField duplicate error = %v", err)
	}
	if err := root.RenameObjectField("b", "a"); !errors.Is(err, ErrObjectFieldExists) {
		t.Fatalf("RenameObjectField duplicate error = %v", err)
	}
	if err := root.RenameObjectField("a", "a"); err != nil {
		t.Fatalf("RenameObjectField same name: %v", err)
	}
}

func TestEncodeMetaChecksEncoderSyntax(t *testing.T) {
	m, err := NewJSONCDecoder(strings.NewReader("{// comment\n\"a\":1}")).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta JSONC: %v", err)
	}

	var strict bytes.Buffer
	if err := NewEncoder(&strict).EncodeMeta(m); !errors.Is(err, ErrUnexpectedToken) {
		t.Fatalf("strict EncodeMeta error = %v", err)
	}

	var jsonc bytes.Buffer
	e := NewEncoder(&jsonc)
	e.SyntaxOptions = JSONCSyntaxOptions()
	if err := e.EncodeMeta(m); err != nil {
		t.Fatalf("JSONC EncodeMeta: %v", err)
	}
	if got := jsonc.String(); !strings.Contains(got, "// comment") {
		t.Fatalf("EncodeMeta output = %q", got)
	}

	out, err := MarshalMeta(m)
	if err != nil {
		t.Fatalf("MarshalMeta with parsed syntax: %v", err)
	}
	if !bytes.Contains(out, []byte("// comment")) {
		t.Fatalf("MarshalMeta output = %q", out)
	}
}

func TestGeneratedOutputFollowsSyntaxOptions(t *testing.T) {
	var strict bytes.Buffer
	if err := NewEncoder(&strict).Encode(map[string]float64{"n": math.NaN()}); err == nil {
		t.Fatal("strict encoder accepted NaN")
	}

	var json5 bytes.Buffer
	e := NewJSON5Encoder(&json5)
	if err := e.Encode(map[string]float64{"n": math.NaN()}); err != nil {
		t.Fatalf("JSON5 encoder rejected NaN: %v", err)
	}
	if got := json5.String(); !strings.Contains(got, "n:NaN") {
		t.Fatalf("JSON5 encoder output = %q", got)
	}
}

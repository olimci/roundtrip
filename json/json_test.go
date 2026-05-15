package json

import (
	"bytes"
	stdjson "encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
)

type parityStruct struct {
	Name       string         `json:"name"`
	Age        int            `json:"age,string"`
	Empty      string         `json:"empty,omitempty"`
	Zero       int            `json:"zero,omitzero"`
	Bytes      []byte         `json:"bytes"`
	Tags       []string       `json:"tags"`
	Counts     map[int]string `json:"counts"`
	Ignored    string         `json:"-"`
	unexported string
}

type textKey int

func (k textKey) MarshalText() ([]byte, error) {
	return []byte(fmt.Sprintf("key-%02d", k)), nil
}

func (k *textKey) UnmarshalText(b []byte) error {
	var n int
	if _, err := fmt.Sscanf(string(b), "key-%02d", &n); err != nil {
		return err
	}
	*k = textKey(n)
	return nil
}

type customJSON struct {
	Value string
}

func (c customJSON) MarshalJSON() ([]byte, error) {
	return []byte(strconv.Quote("custom:" + c.Value)), nil
}

func (c *customJSON) UnmarshalJSON(b []byte) error {
	var s string
	if err := stdjson.Unmarshal(b, &s); err != nil {
		return err
	}
	c.Value = strings.TrimPrefix(s, "custom:")
	return nil
}

func TestMarshalParityWithStdlib(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{"nil", nil},
		{"bool", true},
		{"string escapes", "hello\n\t\"world\"\u2028"},
		{"ints", []int{-2, -1, 0, 1, 2}},
		{"uints", []uint{0, 1, 2}},
		{"floats", []float64{0, -0, 1.25, 1e-9, 1e9}},
		{"slice", []any{"x", float64(2), false, nil}},
		{"bytes", []byte("hello")},
		{"map sort", map[string]any{"z": 1, "a": []string{"b", "c"}}},
		{"int map keys", map[int]string{10: "ten", -1: "minus"}},
		{"text map keys", map[textKey]string{2: "two", 1: "one"}},
		{"struct tags", parityStruct{
			Name:       "Ada",
			Age:        42,
			Bytes:      []byte("abc"),
			Tags:       []string{"compiler", "math"},
			Counts:     map[int]string{2: "two", 1: "one"},
			Ignored:    "hidden",
			unexported: "hidden",
		}},
		{"custom marshaler", customJSON{Value: "value"}},
		{"raw message", RawMessage(`{"kept":true}`)},
		{"number", Number("12.5")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := Marshal(tt.value)
			want, wantErr := stdjson.Marshal(tt.value)
			if (gotErr != nil) != (wantErr != nil) {
				t.Fatalf("Marshal error mismatch: got %v want %v", gotErr, wantErr)
			}
			if gotErr != nil {
				return
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("Marshal mismatch:\ngot  %s\nwant %s", got, want)
			}
		})
	}
}

func TestMarshalUnsupportedValuesParity(t *testing.T) {
	tests := []struct {
		name  string
		value any
	}{
		{"nan", math.NaN()},
		{"positive inf", math.Inf(1)},
		{"negative inf", math.Inf(-1)},
		{"chan", make(chan int)},
		{"func", func() {}},
		{"bad number", Number("01")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, gotErr := Marshal(tt.value)
			_, wantErr := stdjson.Marshal(tt.value)
			if gotErr == nil || wantErr == nil {
				t.Fatalf("expected both encoders to fail, got %v want %v", gotErr, wantErr)
			}
		})
	}
}

func TestUnmarshalParityWithStdlib(t *testing.T) {
	type payloadStruct struct {
		Name   string         `json:"name"`
		Age    int            `json:"age,string"`
		Bytes  []byte         `json:"bytes"`
		Counts map[int]string `json:"counts"`
		Custom customJSON     `json:"custom"`
	}

	tests := []struct {
		name string
		data string
		new  func() any
	}{
		{"interface object", `{"a":1,"b":[true,null,"x"],"c":{"d":2.5}}`, func() any { var v any; return &v }},
		{"slice", `[1,2,3]`, func() any { return new([]int) }},
		{"array truncates", `[1,2,3]`, func() any { return new([2]int) }},
		{"map string", `{"a":1,"b":2}`, func() any { return new(map[string]int) }},
		{"map int keys", `{"-1":"minus","2":"two"}`, func() any { return new(map[int]string) }},
		{"map text keys", `{"key-01":"one","key-02":"two"}`, func() any { return new(map[textKey]string) }},
		{"bytes", `"aGVsbG8="`, func() any { return new([]byte) }},
		{"struct", `{"name":"Ada","age":"42","bytes":"YWJj","counts":{"1":"one"},"custom":"custom:value","extra":true}`, func() any {
			return new(payloadStruct)
		}},
		{"raw message", `{"x":[1,true,null]}`, func() any { return new(RawMessage) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.new()
			want := tt.new()
			_, gotErr := Unmarshal([]byte(tt.data), got)
			wantErr := stdjson.Unmarshal([]byte(tt.data), want)
			if (gotErr != nil) != (wantErr != nil) {
				t.Fatalf("Unmarshal error mismatch: got %v want %v", gotErr, wantErr)
			}
			if gotErr != nil {
				return
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("Unmarshal mismatch:\ngot  %#v\nwant %#v", got, want)
			}
		})
	}
}

func TestUnmarshalInvalidInputsParity(t *testing.T) {
	tests := []string{
		``,
		`{`,
		`{"a":}`,
		`{"a":1,}`,
		`[1,]`,
		`{"a":01}`,
		`{"a":"unterminated}`,
		`{"a":"bad\q"}`,
		`{"a":tru}`,
		`true false`,
	}

	for _, data := range tests {
		t.Run(data, func(t *testing.T) {
			var got any
			var want any
			_, gotErr := Unmarshal([]byte(data), &got)
			wantErr := stdjson.Unmarshal([]byte(data), &want)
			if gotErr == nil || wantErr == nil {
				t.Fatalf("expected both decoders to fail, got %v want %v", gotErr, wantErr)
			}
		})
	}
}

func TestDecoderOptions(t *testing.T) {
	t.Run("UseNumber", func(t *testing.T) {
		d := NewDecoder(strings.NewReader(`{"n":12345678901234567890}`))
		d.UseNumber()
		var got map[string]any
		if _, err := d.Decode(&got); err != nil {
			t.Fatal(err)
		}
		n, ok := got["n"].(Number)
		if !ok || n.String() != "12345678901234567890" {
			t.Fatalf("got %#v, want json.Number-like value", got["n"])
		}
	})

	t.Run("DisallowUnknownFields", func(t *testing.T) {
		type dst struct {
			A int `json:"a"`
		}
		d := NewDecoder(strings.NewReader(`{"a":1,"b":2}`))
		d.DisallowUnknownFields()
		var got dst
		if _, err := d.Decode(&got); err == nil || !strings.Contains(err.Error(), `unknown field "b"`) {
			t.Fatalf("got error %v, want unknown field error", err)
		}
	})

	t.Run("comments and trailing commas", func(t *testing.T) {
		d := NewDecoder(strings.NewReader("{\n  // item count\n  \"n\": 1,\n}\n"))
		d.SetAllowComments(true)
		d.SetAllowTrailingComma(true)
		var got map[string]int
		m, err := d.Decode(&got)
		if err != nil {
			t.Fatal(err)
		}
		if got["n"] != 1 {
			t.Fatalf("got %v", got)
		}
		if string(m.Root().Bytes()) != "{\n  // item count\n  \"n\": 1,\n}" {
			t.Fatalf("root bytes lost trivia: %q", m.Root().Bytes())
		}
	})
}

func TestDecoderRepeatedDecodeParity(t *testing.T) {
	input := `{"a":1} [2,3] true`
	d := NewDecoder(strings.NewReader(input))
	std := stdjson.NewDecoder(strings.NewReader(input))

	var got1 map[string]int
	var want1 map[string]int
	if _, err := d.Decode(&got1); err != nil {
		t.Fatal(err)
	}
	if err := std.Decode(&want1); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got1, want1) {
		t.Fatalf("first decode got %#v want %#v", got1, want1)
	}

	if d.InputOffset() != std.InputOffset() {
		t.Fatalf("offset after first decode got %d want %d", d.InputOffset(), std.InputOffset())
	}
	buffered, err := io.ReadAll(d.Buffered())
	if err != nil {
		t.Fatal(err)
	}
	if string(buffered) != ` [2,3] true` {
		t.Fatalf("buffered after first decode got %q", buffered)
	}

	var got2 []int
	var want2 []int
	if _, err := d.Decode(&got2); err != nil {
		t.Fatal(err)
	}
	if err := std.Decode(&want2); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got2, want2) {
		t.Fatalf("second decode got %#v want %#v", got2, want2)
	}

	var got3 bool
	var want3 bool
	if _, err := d.Decode(&got3); err != nil {
		t.Fatal(err)
	}
	if err := std.Decode(&want3); err != nil {
		t.Fatal(err)
	}
	if got3 != want3 {
		t.Fatalf("third decode got %v want %v", got3, want3)
	}
	if _, err := d.Decode(new(any)); !errors.Is(err, io.EOF) {
		t.Fatalf("final decode got %v want io.EOF", err)
	}
}

func TestDecoderRepeatedDecodeMetaListsAreSeparate(t *testing.T) {
	d := NewDecoder(strings.NewReader(`{"a":1}{"b":2}`))

	first, err := d.decodeMeta(false)
	if err != nil {
		t.Fatal(err)
	}
	second, err := d.decodeMeta(false)
	if err != nil {
		t.Fatal(err)
	}

	if got := string(first.Root().Bytes()); got != `{"a":1}` {
		t.Fatalf("first root got %q", got)
	}
	if got := string(second.Root().Bytes()); got != `{"b":2}` {
		t.Fatalf("second root got %q", got)
	}
	if first.SST.Tokens == second.SST.Tokens {
		t.Fatal("DecodeMeta reused token list across values")
	}
}

func TestDecoderMore(t *testing.T) {
	d := NewDecoder(strings.NewReader(` 1 2 ]`))
	if !d.More() {
		t.Fatal("More before first value got false")
	}
	var first int
	if _, err := d.Decode(&first); err != nil {
		t.Fatal(err)
	}
	if first != 1 {
		t.Fatalf("first got %d", first)
	}
	if !d.More() {
		t.Fatal("More before second value got false")
	}
	var second int
	if _, err := d.Decode(&second); err != nil {
		t.Fatal(err)
	}
	if second != 2 {
		t.Fatalf("second got %d", second)
	}
	if d.More() {
		t.Fatal("More before closing delimiter got true")
	}
}

func TestUtilityFunctionsParityWithStdlib(t *testing.T) {
	validTests := []string{
		`null`,
		`{"a":[1,true,null],"b":"x"}`,
		``,
		`{"a":1,}`,
		`{"a":// comment` + "\n" + `1}`,
	}
	for _, data := range validTests {
		if got, want := Valid([]byte(data)), stdjson.Valid([]byte(data)); got != want {
			t.Fatalf("Valid(%q) = %v, want %v", data, got, want)
		}
	}

	formatTests := []string{
		`{"b":2,"a":[1,true,null],"s":"<&>"}`,
		" \n\t" + `{"nested":{"x":1},"arr":[1,2]}`,
		`{"trailing":true}` + "\n\t ",
	}
	for _, data := range formatTests {
		t.Run("Compact/"+data, func(t *testing.T) {
			var got bytes.Buffer
			var want bytes.Buffer
			gotErr := Compact(&got, []byte(data))
			wantErr := stdjson.Compact(&want, []byte(data))
			if (gotErr != nil) != (wantErr != nil) {
				t.Fatalf("Compact error got %v want %v", gotErr, wantErr)
			}
			if got.String() != want.String() {
				t.Fatalf("Compact got %q want %q", got.String(), want.String())
			}
		})

		t.Run("Indent/"+data, func(t *testing.T) {
			var got bytes.Buffer
			var want bytes.Buffer
			gotErr := Indent(&got, []byte(data), ">", "  ")
			wantErr := stdjson.Indent(&want, []byte(data), ">", "  ")
			if (gotErr != nil) != (wantErr != nil) {
				t.Fatalf("Indent error got %v want %v", gotErr, wantErr)
			}
			if got.String() != want.String() {
				t.Fatalf("Indent got:\n%s\nwant:\n%s", got.String(), want.String())
			}
		})
	}

	html := []byte(`{"s":"<&>` + "\u2028\u2029" + `"}`)
	var got bytes.Buffer
	var want bytes.Buffer
	HTMLEscape(&got, html)
	stdjson.HTMLEscape(&want, html)
	if got.String() != want.String() {
		t.Fatalf("HTMLEscape got %q want %q", got.String(), want.String())
	}
}

func TestUtilityFunctionsPreserveComments(t *testing.T) {
	input := []byte("{\n  // before\n  \"a\": 1, // after\n  \"b\": [2,],\n}\n")

	var compact bytes.Buffer
	if err := Compact(&compact, input); err != nil {
		t.Fatal(err)
	}
	compactWant := "{// before\n\"a\":1,// after\n\"b\":[2,],}"
	if compact.String() != compactWant {
		t.Fatalf("Compact with comments got %q want %q", compact.String(), compactWant)
	}

	var indented bytes.Buffer
	if err := Indent(&indented, input, "", "  "); err != nil {
		t.Fatal(err)
	}
	indentWant := "{\n  // before\n  \"a\": 1, // after\n  \"b\": [\n    2,\n  ],\n}\n"
	if indented.String() != indentWant {
		t.Fatalf("Indent with comments got:\n%s\nwant:\n%s", indented.String(), indentWant)
	}
}

func TestEncoderOptionErgonomics(t *testing.T) {
	var got bytes.Buffer
	enc := NewEncoder(&got)
	enc.SetIndent(">", "  ")
	if err := enc.Encode(map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	var want bytes.Buffer
	std := stdjson.NewEncoder(&want)
	std.SetIndent(">", "  ")
	if err := std.Encode(map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	if got.String() != strings.TrimSuffix(want.String(), "\n") {
		t.Fatalf("SetIndent got:\n%s\nwant:\n%s", got.String(), want.String())
	}

	got.Reset()
	enc = NewEncoder(&got)
	enc.SetEscapeHTML(false)
	if err := enc.Encode("<&>"); err != nil {
		t.Fatal(err)
	}
	if got.String() != `"<&>"` {
		t.Fatalf("SetEscapeHTML(false) got %q", got.String())
	}

	encoded, err := Marshal("<&>")
	if err != nil {
		t.Fatal(err)
	}
	if string(encoded) != `"\u003c\u0026\u003e"` {
		t.Fatalf("default HTML escaping got %s", encoded)
	}
}

func TestProductionDoesNotImportEncodingJSON(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), `"encoding/json"`) {
			t.Fatalf("%s imports encoding/json", file)
		}
	}
}

func TestLexerTokens(t *testing.T) {
	input := "{\n  \"a\": -1.25e+3, // line\n  \"b\": \"x\\n\\u0041\", /* block */ true\n}"
	var got []token
	for tok := range lex([]byte(input)) {
		got = append(got, tok)
	}

	want := []struct {
		typ TokenType
		lit string
	}{
		{TokenLeftBrace, "{"},
		{TokenNewline, "\n"},
		{TokenWhitespace, "  "},
		{TokenString, `"a"`},
		{TokenColon, ":"},
		{TokenWhitespace, " "},
		{TokenNumber, "-1.25e+3"},
		{TokenComma, ","},
		{TokenWhitespace, " "},
		{TokenComment, "// line"},
		{TokenNewline, "\n"},
		{TokenWhitespace, "  "},
		{TokenString, `"b"`},
		{TokenColon, ":"},
		{TokenWhitespace, " "},
		{TokenString, `"x\n\u0041"`},
		{TokenComma, ","},
		{TokenWhitespace, " "},
		{TokenComment, "/* block */"},
		{TokenWhitespace, " "},
		{TokenIdentifier, "true"},
		{TokenNewline, "\n"},
		{TokenRightBrace, "}"},
	}
	if len(got) != len(want) {
		t.Fatalf("token count got %d want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Type != want[i].typ || got[i].Literal != want[i].lit {
			t.Fatalf("token %d got %s %q want %s %q", i, got[i].Type, got[i].Literal, want[i].typ, want[i].lit)
		}
	}
	if got[3].Position.Line != 2 || got[3].Position.Column != 3 {
		t.Fatalf("string token position got %+v, want line 2 column 3", got[3].Position)
	}
}

func TestLexerRejectsMalformedTokens(t *testing.T) {
	tests := []struct {
		input string
		lit   string
	}{
		{`"unterminated`, `"unterminated`},
		{`"bad\q"`, `"bad\q`},
		{`"bad` + "\n" + `"`, `"bad` + "\n"},
		{`/* missing close`, `/* missing close`},
		{`01`, `01`},
		{`1.`, `1.`},
		{`1e`, `1e`},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			l := newLexer(strings.NewReader(tt.input))
			tok := l.next()
			if tok.Type != TokenIllegal || tok.Literal != tt.lit {
				t.Fatalf("got %s %q, want ILLEGAL %q", tok.Type, tok.Literal, tt.lit)
			}
		})
	}
}

func TestValidNumberParityWithStdlibSyntax(t *testing.T) {
	tests := []string{
		"0", "-0", "1", "-1", "12", "0.1", "-0.1", "1e9", "1E-9", "1.2e+3",
		"", "-", "+1", "01", "-01", "1.", ".1", "1e", "1e+", "1e9999", "NaN", "Infinity",
	}
	for _, s := range tests {
		got := validNumber(s)
		want := stdjson.Valid([]byte(s))
		if got != want {
			t.Fatalf("validNumber(%q) = %v, want %v", s, got, want)
		}
	}
}

func TestMetaExactRoundTripAndNodeEdits(t *testing.T) {
	const input = "{\n  \"name\": \"Ada\", // keep\n  \"items\": [\n    1,\n    2\n  ]\n}"
	d := NewDecoder(strings.NewReader(input))
	d.AllowComments = true
	m, err := d.DecodeMeta()
	if err != nil {
		t.Fatal(err)
	}
	if m.Indent != "  " {
		t.Fatalf("indent got %q want two spaces", m.Indent)
	}
	if got, err := MarshalMeta(m); err != nil || string(got) != input {
		t.Fatalf("MarshalMeta got %q err %v, want exact input", got, err)
	}

	root := m.Root()
	name, ok := root.ObjectField("name")
	if !ok {
		t.Fatal("missing name field")
	}
	comment, ok := name.TrailingComment()
	if !ok || comment.Text() != "keep" {
		t.Fatalf("trailing comment got %q ok %v", comment.Text(), ok)
	}
	if err := comment.ReplaceText("kept"); err != nil {
		t.Fatal(err)
	}
	if err := name.Replace("Grace"); err != nil {
		t.Fatal(err)
	}
	items, ok := root.ObjectField("items")
	if !ok {
		t.Fatal("missing items field")
	}
	if err := items.AppendArrayValue(3); err != nil {
		t.Fatal(err)
	}
	if err := items.RemoveArrayValue(0); err != nil {
		t.Fatal(err)
	}
	if err := root.RenameObjectField("name", "full_name"); err != nil {
		t.Fatal(err)
	}
	if err := root.InsertObjectField("active", true); err != nil {
		t.Fatal(err)
	}

	got, err := MarshalMeta(m)
	if err != nil {
		t.Fatal(err)
	}
	want := "{\n  \"full_name\": \"Grace\", // kept\n  \"items\": [\n    2,\n    3\n  ],\n  \"active\": true\n}"
	if string(got) != want {
		t.Fatalf("edited JSON mismatch:\ngot:\n%s\nwant:\n%s", got, want)
	}

	var decoded struct {
		FullName string `json:"full_name"`
		Items    []int  `json:"items"`
		Active   bool   `json:"active"`
	}
	d = NewDecoder(bytes.NewReader(got))
	d.AllowComments = true
	if _, err := d.Decode(&decoded); err != nil {
		t.Fatal(err)
	}
	if decoded.FullName != "Grace" || !slices.Equal(decoded.Items, []int{2, 3}) || !decoded.Active {
		t.Fatalf("decoded edited JSON: %#v", decoded)
	}
}

func TestRoundTripSafety(t *testing.T) {
	tests := []string{
		`null`,
		`true`,
		`"hello\nworld"`,
		`123.5`,
		`[1,true,null,"x",{"nested":[2,3]}]`,
		`{"z":1,"a":[{"b":"c"}],"empty":{}}`,
	}
	for _, data := range tests {
		t.Run(data, func(t *testing.T) {
			var got any
			if _, err := Unmarshal([]byte(data), &got); err != nil {
				t.Fatal(err)
			}
			encoded, err := Marshal(got)
			if err != nil {
				t.Fatal(err)
			}
			assertSameJSON(t, encoded, []byte(data))
		})
	}
}

func TestEncodeDecodeErrors(t *testing.T) {
	var dst int
	if _, err := Unmarshal([]byte("1"), dst); err == nil {
		t.Fatal("Unmarshal into non-pointer succeeded")
	} else {
		var target InvalidUnmarshalError
		if !errors.As(err, &target) {
			t.Fatalf("got %T, want InvalidUnmarshalError", err)
		}
	}

	if _, err := Unmarshal([]byte(`"x"`), &dst); err == nil {
		t.Fatal("Unmarshal string into int succeeded")
	} else {
		var target *UnmarshalTypeError
		if !errors.As(err, &target) {
			t.Fatalf("got %T, want UnmarshalTypeError", err)
		}
	}

	var buf bytes.Buffer
	errWriter := writerFunc(func([]byte) (int, error) { return 0, io.ErrClosedPipe })
	if err := NewEncoder(errWriter).Encode(map[string]int{"a": 1}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("Encode writer error got %v, want closed pipe", err)
	}
	if err := NewEncoder(&buf).Encode(math.NaN()); err == nil {
		t.Fatal("Encode NaN succeeded")
	}
}

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(b []byte) (int, error) {
	return f(b)
}

func FuzzUnmarshalParity(f *testing.F) {
	for _, seed := range []string{
		`null`,
		`true`,
		`123`,
		`"hello"`,
		`[1,2,3]`,
		`{"a":1,"b":[false,null,"x"]}`,
		`{"unicode":"\u0041","escape":"\n"}`,
		`{"bad":}`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data string) {
		var want any
		wantErr := stdjson.Unmarshal([]byte(data), &want)
		var got any
		_, gotErr := Unmarshal([]byte(data), &got)
		if wantErr != nil {
			if gotErr == nil {
				t.Fatalf("stdlib rejected %q but decoder accepted %#v", data, got)
			}
			return
		}
		if gotErr != nil {
			t.Fatalf("decoder rejected stdlib-valid JSON %q: %v", data, gotErr)
		}
		gotJSON, err := Marshal(got)
		if err != nil {
			t.Fatalf("re-marshal got value: %v", err)
		}
		wantJSON, err := stdjson.Marshal(want)
		if err != nil {
			t.Fatalf("re-marshal stdlib value: %v", err)
		}
		assertSameJSON(t, gotJSON, wantJSON)
	})
}

func FuzzMarshalRoundTrip(f *testing.F) {
	for _, seed := range []string{
		`null`,
		`false`,
		`1.25`,
		`"x"`,
		`["x",1,true]`,
		`{"a":1,"b":{"c":[2]}}`,
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data string) {
		var v any
		if err := stdjson.Unmarshal([]byte(data), &v); err != nil {
			return
		}
		encoded, err := Marshal(v)
		if err != nil {
			t.Fatalf("Marshal failed for %#v: %v", v, err)
		}
		var decoded any
		if _, err := Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("Unmarshal of encoded JSON %s failed: %v", encoded, err)
		}
		assertSameJSON(t, encoded, []byte(data))
	})
}

func assertSameJSON(t *testing.T, got, want []byte) {
	t.Helper()
	var gotValue any
	if err := stdjson.Unmarshal(got, &gotValue); err != nil {
		t.Fatalf("got invalid JSON %q: %v", got, err)
	}
	var wantValue any
	if err := stdjson.Unmarshal(want, &wantValue); err != nil {
		t.Fatalf("want invalid JSON %q: %v", want, err)
	}
	if !reflect.DeepEqual(gotValue, wantValue) {
		t.Fatalf("JSON mismatch:\ngot  %s\nwant %s", got, want)
	}
}

package json

import (
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"io"
	"math"
	"reflect"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzUnmarshalParity(f *testing.F) {
	for _, seed := range []string{
		`null`,
		`true`,
		`false`,
		`123`,
		`-0`,
		`1.25`,
		`1e9`,
		`"hello"`,
		`"hello\nworld"`,
		`"\u0041\u2028\u2029"`,
		`[]`,
		`[1,2,3]`,
		`{"a":1,"b":[false,null,"x"]}`,
		`{"unicode":"\u0041","escape":"\n"}`,
		`{"bad":}`,
		`{"a":01}`,
		`true false`,
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

		assertCanonicalJSONEqual(t, gotJSON, wantJSON)
	})
}

func FuzzMarshalRoundTripFromValidJSON(f *testing.F) {
	for _, seed := range []string{
		`null`,
		`false`,
		`1.25`,
		`"x"`,
		`["x",1,true]`,
		`{"a":1,"b":{"c":[2]}}`,
		`{"z":1,"a":[{"b":"c"}],"empty":{}}`,
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

		assertCanonicalJSONEqual(t, encoded, []byte(data))
	})
}

func FuzzGeneratedJSONRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		{0},
		{1, 2, 3, 4},
		[]byte("scalar"),
		[]byte("array with nested values"),
		[]byte("object with nested values"),
		{0xff, 0x00, 0x7f, 0x80, 0x40},
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		g := newJSONGenerator(data)

		v := g.value(8)
		input, err := stdjson.Marshal(v)
		if err != nil {
			t.Fatalf("stdlib could not marshal generated JSON value %#v: %v", v, err)
		}

		var decoded any
		if _, err := Unmarshal(input, &decoded); err != nil {
			t.Fatalf("decoder rejected generated valid JSON:\n%s\nerror: %v", input, err)
		}

		output, err := Marshal(decoded)
		if err != nil {
			t.Fatalf("Marshal after decoding generated JSON failed: %v\ninput: %s", err, input)
		}

		var reparsed any
		if err := stdjson.Unmarshal(output, &reparsed); err != nil {
			t.Fatalf("Marshal emitted invalid JSON:\n%s\nerror: %v", output, err)
		}

		assertCanonicalJSONEqual(t, output, input)
	})
}

func FuzzGeneratedJSONCRoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		{0},
		{1, 2, 3, 4},
		[]byte("jsonc comments"),
		[]byte("jsonc whitespace"),
		[]byte("jsonc nested containers"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		g := newJSONGenerator(data)

		v := g.value(8)
		input, err := stdjson.Marshal(v)
		if err != nil {
			t.Fatalf("stdlib could not marshal generated JSON value %#v: %v", v, err)
		}

		decorated := injectJSONTrivia(input, g, false)

		var decoded any
		if _, err := NewJSONCDecoder(bytes.NewReader(decorated)).Decode(&decoded); err != nil {
			t.Fatalf("JSONC decoder rejected decorated JSON:\n%s\nbase: %s\nerror: %v", decorated, input, err)
		}

		output, err := Marshal(decoded)
		if err != nil {
			t.Fatalf("Marshal after JSONC decode failed: %v\ndecorated: %s", err, decorated)
		}

		assertCanonicalJSONEqual(t, output, input)
	})
}

func FuzzGeneratedJSON5RoundTrip(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		{0},
		{1, 2, 3, 4},
		[]byte("json5 comments"),
		[]byte("json5 whitespace"),
		[]byte("json5 nested containers"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		g := newJSONGenerator(data)

		v := g.value(8)
		input, err := stdjson.Marshal(v)
		if err != nil {
			t.Fatalf("stdlib could not marshal generated JSON value %#v: %v", v, err)
		}

		decorated := injectJSONTrivia(input, g, true)

		var decoded any
		if _, err := NewJSON5Decoder(bytes.NewReader(decorated)).Decode(&decoded); err != nil {
			t.Fatalf("JSON5 decoder rejected decorated JSON:\n%s\nbase: %s\nerror: %v", decorated, input, err)
		}

		var b bytes.Buffer
		if err := NewJSON5Encoder(&b).Encode(decoded); err != nil {
			t.Fatalf("JSON5 encode after decode failed: %v\ndecorated: %s", err, decorated)
		}

		var reparsed any
		if _, err := NewJSON5Decoder(bytes.NewReader(b.Bytes())).Decode(&reparsed); err != nil {
			t.Fatalf("JSON5 decoder rejected JSON5 encoder output:\n%s\nerror: %v", b.Bytes(), err)
		}

		output, err := Marshal(reparsed)
		if err != nil {
			t.Fatalf("Marshal after JSON5 reparse failed: %v\nencoded: %s", err, b.Bytes())
		}

		assertCanonicalJSONEqual(t, output, input)
	})
}

func FuzzMetaMutations(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		{0},
		{1, 2, 3, 4},
		[]byte("mutate object fields"),
		[]byte("mutate array values"),
		[]byte("replace nested scalar"),
		[]byte("json5 decorated mutations"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		g := newJSONGenerator(data)

		input, err := stdjson.Marshal(g.value(5))
		if err != nil {
			t.Fatalf("stdlib could not marshal generated JSON: %v", err)
		}
		if g.bool() {
			input = injectJSONTrivia(input, g, true)
		}

		m, err := NewJSON5Decoder(bytes.NewReader(input)).DecodeMeta()
		if err != nil {
			t.Fatalf("DecodeMeta rejected generated input:\n%s\nerror: %v", input, err)
		}

		for range 12 {
			switch g.intn(6) {
			case 0:
				nodes := metaValueNodes(m)
				n := nodes[g.intn(len(nodes))]
				if err := n.Replace(g.value(3)); err != nil {
					t.Fatalf("Replace failed for %v: %v", n.Type(), err)
				}
			case 1:
				nodes := metaNodesOfType(m, NodeTypeObject)
				if len(nodes) == 0 {
					continue
				}
				n := nodes[g.intn(len(nodes))]
				if err := n.InsertObjectField(g.objectKeyName(), g.value(3)); err != nil {
					t.Fatalf("InsertObjectField failed: %v", err)
				}
			case 2:
				nodes := metaNodesOfType(m, NodeTypeObject)
				if len(nodes) == 0 {
					continue
				}
				n := nodes[g.intn(len(nodes))]
				names := objectFieldNames(n)
				if len(names) == 0 {
					continue
				}
				if err := n.RenameObjectField(names[g.intn(len(names))], g.objectKeyName()); err != nil {
					t.Fatalf("RenameObjectField failed: %v", err)
				}
			case 3:
				nodes := metaNodesOfType(m, NodeTypeObject)
				if len(nodes) == 0 {
					continue
				}
				n := nodes[g.intn(len(nodes))]
				names := objectFieldNames(n)
				if len(names) == 0 {
					continue
				}
				if err := n.RemoveObjectField(names[g.intn(len(names))]); err != nil {
					t.Fatalf("RemoveObjectField failed: %v", err)
				}
			case 4:
				nodes := metaNodesOfType(m, NodeTypeArray)
				if len(nodes) == 0 {
					continue
				}
				n := nodes[g.intn(len(nodes))]
				if err := n.AppendArrayValue(g.value(3)); err != nil {
					t.Fatalf("AppendArrayValue failed: %v", err)
				}
			default:
				nodes := metaNodesOfType(m, NodeTypeArray)
				if len(nodes) == 0 {
					continue
				}
				n := nodes[g.intn(len(nodes))]
				if len(n.Children()) == 0 {
					continue
				}
				if err := n.RemoveArrayValue(g.intn(len(n.Children()))); err != nil {
					t.Fatalf("RemoveArrayValue failed: %v", err)
				}
			}

			assertMetaStillRoundTrips(t, m)
		}
	})
}

func FuzzStreamingDecoder(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		{0},
		{1, 2, 3, 4},
		[]byte("stream strict values"),
		[]byte("stream jsonc trivia"),
		[]byte("stream nested values"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		g := newJSONGenerator(data)
		jsonc := g.bool()
		count := g.intn(8) + 1

		var input bytes.Buffer
		for i := range count {
			b, err := stdjson.Marshal(g.value(4))
			if err != nil {
				t.Fatalf("stdlib could not marshal generated value: %v", err)
			}
			if i > 0 {
				if jsonc {
					input.WriteString(g.trivia(false))
				} else {
					input.WriteString(g.strictTrivia())
				}
				input.WriteByte('\n')
			}
			input.Write(b)
		}
		if jsonc {
			input.WriteString(g.trivia(false))
		} else {
			input.WriteString(g.strictTrivia())
		}

		d := NewDecoder(bytes.NewReader(input.Bytes()))
		if jsonc {
			d = NewJSONCDecoder(bytes.NewReader(input.Bytes()))
		}

		lastOffset := int64(-1)
		decoded := 0
		for {
			_ = d.More()
			offset := d.InputOffset()
			if offset < lastOffset {
				t.Fatalf("InputOffset moved backwards: %d then %d", lastOffset, offset)
			}
			lastOffset = offset

			var v any
			_, err := d.Decode(&v)
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("Decode failed after %d values from %q: %v", decoded, input.String(), err)
			}
			decoded++

			out, err := Marshal(v)
			if err != nil {
				t.Fatalf("Marshal of streamed value failed: %v", err)
			}
			if !Valid(out) {
				t.Fatalf("streamed value re-marshaled to invalid JSON: %s", out)
			}

			buf, err := io.ReadAll(d.Buffered())
			if err != nil {
				t.Fatalf("Buffered read failed: %v", err)
			}
			if len(buf) > input.Len() {
				t.Fatalf("Buffered returned more data than input: %d > %d", len(buf), input.Len())
			}
			if decoded > count+1 {
				t.Fatalf("decoded too many values from stream: got %d want at most %d", decoded, count)
			}
		}

		if decoded != count {
			t.Fatalf("decoded %d values, want %d from %q", decoded, count, input.String())
		}
	})
}

func FuzzJSON5Syntax(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		{0},
		{1, 2, 3, 4},
		[]byte("identifier keys"),
		[]byte("single quoted strings"),
		[]byte("hex numbers"),
		[]byte("nan infinity trailing comma"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		g := newJSONGenerator(data)
		input, json5Only := g.json5Value(4)
		input = g.trivia(true) + input + g.trivia(true)

		d := NewJSON5Decoder(strings.NewReader(input))
		d.UseNumber()
		var decoded any
		if _, err := d.Decode(&decoded); err != nil {
			t.Fatalf("JSON5 decoder rejected generated JSON5 %q: %v", input, err)
		}

		var b bytes.Buffer
		if err := NewJSON5Encoder(&b).Encode(decoded); err != nil {
			t.Fatalf("JSON5 encoder rejected decoded value from %q: %v", input, err)
		}
		if _, err := NewJSON5Decoder(bytes.NewReader(b.Bytes())).DecodeMeta(); err != nil {
			t.Fatalf("JSON5 decoder rejected encoder output %q: %v", b.String(), err)
		}

		if json5Only {
			if _, err := NewDecoder(strings.NewReader(input)).DecodeMeta(); err == nil {
				t.Fatalf("strict decoder accepted JSON5-only input %q", input)
			}
		}
	})
}

func FuzzGeneratedJSONMarshalParity(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		{0},
		{1, 2, 3, 4},
		[]byte("deep object"),
		[]byte("wide array"),
		[]byte("unicode strings \xf0\x9f\x98\x80"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		g := newJSONGenerator(data)

		v := g.value(8)

		got, gotErr := Marshal(v)
		want, wantErr := stdjson.Marshal(v)

		if (gotErr != nil) != (wantErr != nil) {
			t.Fatalf("Marshal error mismatch for %#v:\ngot  %v\nwant %v", v, gotErr, wantErr)
		}
		if gotErr != nil {
			return
		}

		if !bytes.Equal(got, want) {
			t.Fatalf("Marshal output mismatch for %#v:\ngot  %s\nwant %s", v, got, want)
		}
	})
}

func FuzzGeneratedJSONUnmarshalTypedParity(f *testing.F) {
	for _, seed := range [][]byte{
		{},
		{1, 2, 3},
		[]byte("typed struct"),
		[]byte("map int keys"),
		[]byte("byte slice"),
	} {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		g := newJSONGenerator(data)

		switch g.intn(12) {
		case 0:
			v := generatedPayload{
				Name:   g.string(),
				Age:    g.intn(120),
				Bytes:  []byte(g.string()),
				Counts: g.stringMap(8),
				Tags:   g.stringSlice(8),
			}
			checkTypedRoundTripParity(t, v, func() any { return new(generatedPayload) })

		case 1:
			v := g.stringMap(16)
			checkTypedRoundTripParity(t, v, func() any { return new(map[string]string) })

		case 2:
			v := g.intStringMap(16)
			checkTypedRoundTripParity(t, v, func() any { return new(map[int]string) })

		case 3:
			v := g.intSlice(32)
			checkTypedRoundTripParity(t, v, func() any { return new([]int) })

		case 4:
			v := []byte(g.string())
			checkTypedRoundTripParity(t, v, func() any { return new([]byte) })

		case 5:
			v := generatedNestedPayload{
				Payload: &generatedPayload{
					Name:   g.string(),
					Age:    g.intn(120),
					Counts: g.stringMap(4),
					Tags:   g.stringSlice(4),
				},
				Values: [3]int{g.intn(100), g.intn(100), g.intn(100)},
				Flags:  map[string]bool{"a": g.bool(), "b": g.bool()},
			}
			checkTypedRoundTripParity(t, v, func() any { return new(generatedNestedPayload) })

		case 6:
			v := map[uint16][]int{
				uint16(g.intn(1000)): g.intSlice(8),
				uint16(g.intn(1000)): g.intSlice(8),
			}
			checkTypedRoundTripParity(t, v, func() any { return new(map[uint16][]int) })

		case 7:
			v := map[textKey]string{
				textKey(g.intn(100)): g.string(),
				textKey(g.intn(100)): g.string(),
			}
			checkTypedRoundTripParity(t, v, func() any { return new(map[textKey]string) })

		case 8:
			v := customJSON{Value: g.jsonSafeString()}
			checkTypedRoundTripParity(t, v, func() any { return new(customJSON) })

		case 9:
			raw, err := stdjson.Marshal(g.value(4))
			if err != nil {
				t.Fatalf("stdlib could not marshal raw value: %v", err)
			}
			checkTypedRoundTripParity(t, RawMessage(raw), func() any { return new(RawMessage) })

		case 10:
			checkUseNumberParity(t, g)

		default:
			checkDisallowUnknownFieldsParity(t, g)
		}
	})
}

type generatedPayload struct {
	Name   string            `json:"name"`
	Age    int               `json:"age,string"`
	Bytes  []byte            `json:"bytes,omitempty"`
	Counts map[string]string `json:"counts,omitempty"`
	Tags   []string          `json:"tags,omitempty"`
}

type generatedNestedPayload struct {
	Payload *generatedPayload `json:"payload,omitempty"`
	Values  [3]int            `json:"values"`
	Flags   map[string]bool   `json:"flags,omitempty"`
}

func checkTypedRoundTripParity[T any](t *testing.T, value T, newDst func() any) {
	t.Helper()

	input, err := stdjson.Marshal(value)
	if err != nil {
		t.Fatalf("stdlib could not marshal %#v: %v", value, err)
	}

	got := newDst()
	want := newDst()

	_, gotErr := Unmarshal(input, got)
	wantErr := stdjson.Unmarshal(input, want)

	if (gotErr != nil) != (wantErr != nil) {
		t.Fatalf("Unmarshal error mismatch for %s:\ngot  %v\nwant %v", input, gotErr, wantErr)
	}
	if gotErr != nil {
		return
	}

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("typed Unmarshal mismatch for %s:\ngot  %#v\nwant %#v", input, got, want)
	}

	gotOut, gotErr := Marshal(got)
	wantOut, wantErr := stdjson.Marshal(want)

	if (gotErr != nil) != (wantErr != nil) {
		t.Fatalf("Marshal error mismatch after typed decode:\ngot  %v\nwant %v", gotErr, wantErr)
	}
	if gotErr != nil {
		return
	}

	if !bytes.Equal(gotOut, wantOut) {
		t.Fatalf("typed Marshal mismatch:\ngot  %s\nwant %s", gotOut, wantOut)
	}
}

func checkUseNumberParity(t *testing.T, g *jsonGenerator) {
	t.Helper()

	input, err := stdjson.Marshal(g.value(4))
	if err != nil {
		t.Fatalf("stdlib could not marshal generated value: %v", err)
	}

	var got any
	gotDecoder := NewDecoder(bytes.NewReader(input))
	gotDecoder.UseNumber()
	_, gotErr := gotDecoder.Decode(&got)

	var want any
	wantDecoder := stdjson.NewDecoder(bytes.NewReader(input))
	wantDecoder.UseNumber()
	wantErr := wantDecoder.Decode(&want)

	if (gotErr != nil) != (wantErr != nil) {
		t.Fatalf("UseNumber error mismatch for %s:\ngot  %v\nwant %v", input, gotErr, wantErr)
	}
	if gotErr != nil {
		return
	}

	gotJSON, err := Marshal(got)
	if err != nil {
		t.Fatalf("Marshal after UseNumber decode failed: %v", err)
	}
	wantJSON, err := stdjson.Marshal(want)
	if err != nil {
		t.Fatalf("stdlib Marshal after UseNumber decode failed: %v", err)
	}
	if !bytes.Equal(gotJSON, wantJSON) {
		t.Fatalf("UseNumber output mismatch:\ngot  %s\nwant %s", gotJSON, wantJSON)
	}
}

func checkDisallowUnknownFieldsParity(t *testing.T, g *jsonGenerator) {
	t.Helper()

	input := fmt.Sprintf(
		`{"name":%q,"age":%q,"extra":%q}`,
		g.string(),
		fmt.Sprint(g.intn(120)),
		g.string(),
	)

	var got generatedPayload
	gotDecoder := NewDecoder(strings.NewReader(input))
	gotDecoder.DisallowUnknownFields()
	_, gotErr := gotDecoder.Decode(&got)

	var want generatedPayload
	wantDecoder := stdjson.NewDecoder(strings.NewReader(input))
	wantDecoder.DisallowUnknownFields()
	wantErr := wantDecoder.Decode(&want)

	if (gotErr != nil) != (wantErr != nil) {
		t.Fatalf("DisallowUnknownFields error mismatch for %s:\ngot  %v\nwant %v", input, gotErr, wantErr)
	}
	if gotErr == nil && !reflect.DeepEqual(got, want) {
		t.Fatalf("DisallowUnknownFields value mismatch:\ngot  %#v\nwant %#v", got, want)
	}
}

type jsonGenerator struct {
	data []byte
	pos  int
}

func newJSONGenerator(data []byte) *jsonGenerator {
	return &jsonGenerator{data: data}
}

func (g *jsonGenerator) value(depth int) any {
	if depth <= 0 {
		return g.scalar()
	}

	switch g.intn(8) {
	case 0:
		return nil
	case 1:
		return g.bool()
	case 2:
		return g.string()
	case 3:
		return g.number()
	case 4, 5:
		return g.array(depth - 1)
	default:
		return g.object(depth - 1)
	}
}

func (g *jsonGenerator) scalar() any {
	switch g.intn(4) {
	case 0:
		return nil
	case 1:
		return g.bool()
	case 2:
		return g.string()
	default:
		return g.number()
	}
}

func (g *jsonGenerator) array(depth int) []any {
	n := g.intn(containerWidth(depth))
	out := make([]any, 0, n)

	for range n {
		out = append(out, g.value(depth))
	}

	return out
}

func (g *jsonGenerator) object(depth int) map[string]any {
	n := g.intn(containerWidth(depth))
	out := make(map[string]any, n)

	for i := range n {
		key := g.string()
		if key == "" {
			key = fmt.Sprintf("k%d", i)
		}
		out[key] = g.value(depth)
	}

	return out
}

func (g *jsonGenerator) objectKeyName() string {
	switch g.intn(6) {
	case 0:
		return "alpha"
	case 1:
		return "beta"
	case 2:
		return "$json5"
	case 3:
		return "_value"
	default:
		key := g.string()
		if key == "" {
			return fmt.Sprintf("k%d", g.intn(1000))
		}
		return key
	}
}

func (g *jsonGenerator) number() float64 {
	switch g.intn(12) {
	case 0:
		return 0
	case 1:
		return math.Copysign(0, -1)
	case 2:
		return 1
	case 3:
		return -1
	case 4:
		return 1.25
	case 5:
		return -1.25
	case 6:
		return 1e-9
	case 7:
		return 1e9
	default:
		n := g.int63()
		return float64(n%1_000_000) / float64(g.intn(999)+1)
	}
}

func (g *jsonGenerator) bool() bool {
	return g.byte()%2 == 0
}

func (g *jsonGenerator) string() string {
	n := g.intn(64)

	var b strings.Builder
	for range n {
		switch g.intn(12) {
		case 0:
			b.WriteByte('\n')
		case 1:
			b.WriteByte('\t')
		case 2:
			b.WriteByte('"')
		case 3:
			b.WriteByte('\\')
		case 4:
			b.WriteRune('\u2028')
		case 5:
			b.WriteRune('\u2029')
		case 6:
			b.WriteRune(rune(g.intn(0x80)))
		default:
			r := rune(g.intn(0x10ffff))
			if r == utf8.RuneError || !utf8.ValidRune(r) {
				r = rune('a' + g.intn(26))
			}
			b.WriteRune(r)
		}
	}

	return b.String()
}

func (g *jsonGenerator) jsonSafeString() string {
	n := g.intn(64)

	var b strings.Builder
	for range n {
		switch g.intn(8) {
		case 0:
			b.WriteByte('<')
		case 1:
			b.WriteByte('>')
		case 2:
			b.WriteByte('&')
		case 3:
			b.WriteByte('"')
		case 4:
			b.WriteByte('\\')
		default:
			b.WriteRune('a' + rune(g.intn(26)))
		}
	}

	return b.String()
}

func (g *jsonGenerator) stringSlice(max int) []string {
	n := g.intn(max + 1)
	out := make([]string, 0, n)

	for range n {
		out = append(out, g.string())
	}

	return out
}

func (g *jsonGenerator) intSlice(max int) []int {
	n := g.intn(max + 1)
	out := make([]int, 0, n)

	for range n {
		out = append(out, int(g.int63()%10_000))
	}

	return out
}

func (g *jsonGenerator) stringMap(max int) map[string]string {
	n := g.intn(max + 1)
	out := make(map[string]string, n)

	for i := range n {
		key := g.string()
		if key == "" {
			key = fmt.Sprintf("k%d", i)
		}
		out[key] = g.string()
	}

	return out
}

func (g *jsonGenerator) intStringMap(max int) map[int]string {
	n := g.intn(max + 1)
	out := make(map[int]string, n)

	for range n {
		out[int(g.int63()%10_000)] = g.string()
	}

	return out
}

func (g *jsonGenerator) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(g.byte() % byte(n))
}

func containerWidth(depth int) int {
	return max(2, depth+1)
}

func (g *jsonGenerator) int63() int64 {
	var x uint64
	for range 8 {
		x <<= 8
		x |= uint64(g.byte())
	}
	return int64(x &^ (1 << 63))
}

func (g *jsonGenerator) byte() byte {
	if len(g.data) == 0 {
		g.pos++
		return byte(g.pos * 131)
	}

	b := g.data[g.pos%len(g.data)]
	g.pos++
	return b
}

func injectJSONTrivia(input []byte, g *jsonGenerator, json5 bool) []byte {
	var b bytes.Buffer
	for t := range lex(input) {
		b.WriteString(g.trivia(json5))
		b.WriteString(t.Literal)
		b.WriteString(g.trivia(json5))
	}
	return b.Bytes()
}

func (g *jsonGenerator) trivia(json5 bool) string {
	switch g.intn(12) {
	case 0:
		return " "
	case 1:
		return "\t"
	case 2:
		return "\n"
	case 3:
		return "\r\n"
	case 4:
		return "// " + g.commentText() + "\n"
	case 5:
		return "/* " + g.commentText() + " */"
	case 6:
		if json5 {
			return "\v"
		}
	case 7:
		if json5 {
			return "\u2028"
		}
	case 8:
		if json5 {
			return "\u2029"
		}
	}
	return ""
}

func (g *jsonGenerator) commentText() string {
	n := g.intn(16)
	var b strings.Builder
	for range n {
		b.WriteRune('a' + rune(g.intn(26)))
	}
	return b.String()
}

func (g *jsonGenerator) strictTrivia() string {
	switch g.intn(5) {
	case 0:
		return " "
	case 1:
		return "\t"
	case 2:
		return "\n"
	case 3:
		return "\r\n"
	default:
		return ""
	}
}

func (g *jsonGenerator) json5Value(depth int) (string, bool) {
	if depth <= 0 {
		return g.json5Scalar()
	}

	switch g.intn(6) {
	case 0:
		return g.json5Scalar()
	case 1, 2:
		return g.json5Array(depth - 1)
	default:
		return g.json5Object(depth - 1)
	}
}

func (g *jsonGenerator) json5Scalar() (string, bool) {
	switch g.intn(10) {
	case 0:
		return "null", false
	case 1:
		return "true", false
	case 2:
		return "false", false
	case 3:
		return quoteString(g.string()), false
	case 4:
		return "'" + g.json5SingleQuotedString() + "'", true
	case 5:
		return "0x" + fmt.Sprintf("%x", g.intn(0xffff)+1), true
	case 6:
		return "+" + fmt.Sprint(g.intn(1000)), true
	case 7:
		return ".5", true
	case 8:
		if g.bool() {
			return "NaN", true
		}
		return "-Infinity", true
	default:
		return fmt.Sprint(g.number()), false
	}
}

func (g *jsonGenerator) json5Array(depth int) (string, bool) {
	n := g.intn(containerWidth(depth))
	var b strings.Builder
	json5Only := false
	b.WriteByte('[')
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
			b.WriteString(g.trivia(true))
		}
		value, only := g.json5Value(depth)
		json5Only = json5Only || only
		b.WriteString(value)
	}
	if n > 0 && g.bool() {
		b.WriteByte(',')
		json5Only = true
	}
	b.WriteByte(']')
	return b.String(), json5Only
}

func (g *jsonGenerator) json5Object(depth int) (string, bool) {
	n := g.intn(containerWidth(depth))
	var b strings.Builder
	json5Only := false
	b.WriteByte('{')
	for i := range n {
		if i > 0 {
			b.WriteByte(',')
			b.WriteString(g.trivia(true))
		}
		key, only := g.json5ObjectKey(i)
		value, valueOnly := g.json5Value(depth)
		json5Only = json5Only || only || valueOnly
		b.WriteString(key)
		b.WriteByte(':')
		b.WriteString(g.trivia(true))
		b.WriteString(value)
	}
	if n > 0 && g.bool() {
		b.WriteByte(',')
		json5Only = true
	}
	b.WriteByte('}')
	return b.String(), json5Only
}

func (g *jsonGenerator) json5ObjectKey(i int) (string, bool) {
	switch g.intn(4) {
	case 0:
		return fmt.Sprintf("key%d", i), true
	case 1:
		return "$value", true
	case 2:
		return "'" + g.json5SingleQuotedString() + "'", true
	default:
		return quoteString(g.objectKeyName()), false
	}
}

func (g *jsonGenerator) json5SingleQuotedString() string {
	n := g.intn(24)
	var b strings.Builder
	for range n {
		switch g.intn(10) {
		case 0:
			b.WriteString(`\'`)
		case 1:
			b.WriteString(`\"`)
		case 2:
			b.WriteString(`\x41`)
		case 3:
			b.WriteString(`\v`)
		case 4:
			b.WriteString("\\\n")
		case 5:
			b.WriteString(`\0`)
		default:
			r := 'a' + rune(g.intn(26))
			b.WriteRune(r)
		}
	}
	return b.String()
}

func metaValueNodes(m *Meta) []Node {
	nodes := []Node{m.Root()}
	for n := range m.Nodes() {
		children := n.Children()
		switch n.Type() {
		case NodeTypeArray:
			nodes = append(nodes, children...)
		case NodeTypeObject:
			for i := 1; i < len(children); i += 2 {
				nodes = append(nodes, children[i])
			}
		}
	}
	return nodes
}

func metaNodesOfType(m *Meta, typ NodeType) []Node {
	var nodes []Node
	for n := range m.Nodes() {
		if n.Type() == typ {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

func objectFieldNames(n Node) []string {
	var names []string
	for i := 0; i+1 < len(n.node.Children); i += 2 {
		name, err := decodeKeyLiteral(n.node.Children[i])
		if err == nil {
			names = append(names, name)
		}
	}
	return names
}

func assertMetaStillRoundTrips(t *testing.T, m *Meta) {
	t.Helper()

	out, err := MarshalMeta(m)
	if err != nil {
		t.Fatalf("MarshalMeta failed after mutation: %v", err)
	}
	reparsed, err := NewJSON5Decoder(bytes.NewReader(out)).DecodeMeta()
	if err != nil {
		t.Fatalf("mutated Meta emitted invalid JSON/JSON5:\n%s\nerror: %v", out, err)
	}
	var decoded any
	if err := reparsed.Root().Decode(&decoded); err != nil {
		t.Fatalf("mutated Meta root could not decode:\n%s\nerror: %v", out, err)
	}
}

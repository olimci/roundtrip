package json

import (
	"bytes"
	stdjson "encoding/json"
	"fmt"
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

		switch g.intn(5) {
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

		default:
			v := []byte(g.string())
			checkTypedRoundTripParity(t, v, func() any { return new([]byte) })
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

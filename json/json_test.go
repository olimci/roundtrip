package json

import (
	"bytes"
	stdjson "encoding/json"
	"fmt"
	"reflect"
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
	return fmt.Appendf(nil, "key-%02d", k), nil
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

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(b []byte) (int, error) {
	return f(b)
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

func canonicalJSON(t *testing.T, data []byte) []byte {
	t.Helper()

	var v any
	if err := stdjson.Unmarshal(data, &v); err != nil {
		t.Fatalf("invalid JSON %q: %v", data, err)
	}

	out, err := stdjson.Marshal(v)
	if err != nil {
		t.Fatalf("canonical marshal failed for %#v: %v", v, err)
	}

	return out
}

func assertCanonicalJSONEqual(t *testing.T, got, want []byte) {
	t.Helper()

	gotCanon := canonicalJSON(t, got)
	wantCanon := canonicalJSON(t, want)

	if !bytes.Equal(gotCanon, wantCanon) {
		t.Fatalf("canonical JSON mismatch:\ngot  %s\nwant %s\nraw got  %s\nraw want %s",
			gotCanon, wantCanon, got, want)
	}
}

type PromotedInner struct {
	ID      int
	Renamed string `json:"renamed"`
}

type PromotedPtr struct {
	PtrName string
}

type promotedOuter struct {
	PromotedInner
	*PromotedPtr
	Own string
}

type conflictLeft struct {
	Conflict string
}

type conflictRight struct {
	Conflict string
}

type conflictOuter struct {
	conflictLeft
	conflictRight
}

type zeroByMethod int

func (z zeroByMethod) IsZero() bool {
	return z == 42
}

type omitOptionsPayload struct {
	ZeroStruct struct{}      `json:"zeroStruct,omitzero"`
	Special    zeroByMethod  `json:"special,omitzero"`
	Regular    zeroByMethod  `json:"regular,omitempty"`
	Ptr        *zeroByMethod `json:"ptr,omitzero"`
}

func TestMarshalStructFieldResolutionParity(t *testing.T) {
	tests := []any{
		promotedOuter{
			PromotedInner: PromotedInner{ID: 7, Renamed: "inner"},
			PromotedPtr:   &PromotedPtr{PtrName: "ptr"},
			Own:           "own",
		},
		promotedOuter{
			PromotedInner: PromotedInner{ID: 7, Renamed: "inner"},
			Own:           "own",
		},
		conflictOuter{
			conflictLeft:  conflictLeft{Conflict: "left"},
			conflictRight: conflictRight{Conflict: "right"},
		},
	}

	for _, tc := range tests {
		got, gotErr := Marshal(tc)
		want, wantErr := stdjson.Marshal(tc)
		if (gotErr != nil) != (wantErr != nil) {
			t.Fatalf("Marshal error mismatch for %#v:\ngot  %v\nwant %v", tc, gotErr, wantErr)
		}
		if gotErr != nil {
			continue
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("Marshal mismatch for %#v:\ngot  %s\nwant %s", tc, got, want)
		}
	}
}

func TestUnmarshalStructFieldResolutionParity(t *testing.T) {
	input := []byte(`{"ID":7,"renamed":"inner","PtrName":"ptr","Own":"own"}`)

	var got promotedOuter
	var want promotedOuter
	_, gotErr := Unmarshal(input, &got)
	wantErr := stdjson.Unmarshal(input, &want)
	if (gotErr != nil) != (wantErr != nil) {
		t.Fatalf("Unmarshal error mismatch:\ngot  %v\nwant %v", gotErr, wantErr)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("Unmarshal mismatch:\ngot  %#v\nwant %#v", got, want)
	}

	conflictInput := []byte(`{"Conflict":"ignored"}`)
	var gotConflict conflictOuter
	var wantConflict conflictOuter
	if _, err := Unmarshal(conflictInput, &gotConflict); err != nil {
		t.Fatalf("Unmarshal conflict payload: %v", err)
	}
	if err := stdjson.Unmarshal(conflictInput, &wantConflict); err != nil {
		t.Fatalf("stdlib Unmarshal conflict payload: %v", err)
	}
	if !reflect.DeepEqual(gotConflict, wantConflict) {
		t.Fatalf("conflict Unmarshal mismatch:\ngot  %#v\nwant %#v", gotConflict, wantConflict)
	}
}

func TestOmitEmptyAndOmitZeroParity(t *testing.T) {
	v := omitOptionsPayload{
		Special: 42,
		Regular: 42,
	}

	got, gotErr := Marshal(v)
	want, wantErr := stdjson.Marshal(v)
	if (gotErr != nil) != (wantErr != nil) {
		t.Fatalf("Marshal error mismatch:\ngot  %v\nwant %v", gotErr, wantErr)
	}
	if gotErr != nil {
		return
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Marshal mismatch:\ngot  %s\nwant %s", got, want)
	}
}

func TestMarshalerEscapesHTMLParity(t *testing.T) {
	v := customJSON{Value: "<&>"}

	got, gotErr := Marshal(v)
	want, wantErr := stdjson.Marshal(v)
	if (gotErr != nil) != (wantErr != nil) {
		t.Fatalf("Marshal error mismatch:\ngot  %v\nwant %v", gotErr, wantErr)
	}
	if gotErr != nil {
		return
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("Marshal mismatch:\ngot  %s\nwant %s", got, want)
	}

	var gotBuf bytes.Buffer
	gotEncoder := NewEncoder(&gotBuf)
	gotEncoder.SetEscapeHTML(false)
	if err := gotEncoder.Encode(v); err != nil {
		t.Fatalf("Encode with disabled HTML escaping: %v", err)
	}

	var wantBuf bytes.Buffer
	wantEncoder := stdjson.NewEncoder(&wantBuf)
	wantEncoder.SetEscapeHTML(false)
	if err := wantEncoder.Encode(v); err != nil {
		t.Fatalf("stdlib Encode with disabled HTML escaping: %v", err)
	}

	if !bytes.Equal(gotBuf.Bytes(), bytes.TrimSuffix(wantBuf.Bytes(), []byte("\n"))) {
		t.Fatalf("Encode mismatch with disabled HTML escaping:\ngot  %s\nwant %s", gotBuf.Bytes(), wantBuf.Bytes())
	}
}

func TestDecoderDisallowUnknownFields(t *testing.T) {
	var accepted promotedOuter
	d := NewDecoder(bytes.NewReader([]byte(`{"ID":7,"renamed":"inner","PtrName":"ptr","Own":"own"}`)))
	d.DisallowUnknownFields()
	if _, err := d.Decode(&accepted); err != nil {
		t.Fatalf("Decode with promoted known fields: %v", err)
	}

	var rejected promotedOuter
	d = NewDecoder(bytes.NewReader([]byte(`{"ID":7,"extra":true}`)))
	d.DisallowUnknownFields()
	_, err := d.Decode(&rejected)
	if err == nil {
		t.Fatal("Decode accepted unknown field")
	}
	if !strings.Contains(err.Error(), `json: unknown field "extra"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

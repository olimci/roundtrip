package json

import (
	"errors"
	"strings"
	"testing"
)

func TestMergePatchNode(t *testing.T) {
	target, err := NewJSON5Decoder(strings.NewReader(`{
		"title": "old",
		"author": {"name": "Ann", "active": true},
		"tags": ["old"]
	}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("Decode target: %v", err)
	}
	patch, err := NewJSON5Decoder(strings.NewReader(`{
		"title": "new",
		"author": {"active": null, "email": "ann@example.com"},
		"tags": null,
		"extra": [1, 2]
	}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("Decode patch: %v", err)
	}

	if err := target.Root().Merge(patch.Root()); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	out, err := MarshalMeta(target)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	assertCanonicalJSONEqual(t, out, []byte(`{
		"title": "new",
		"author": {"name": "Ann", "email": "ann@example.com"},
		"extra": [1, 2]
	}`))
}

func TestMergePatchNonObjectReplacesTarget(t *testing.T) {
	target, err := NewDecoder(strings.NewReader(`{"a":1}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("Decode target: %v", err)
	}
	patch, err := NewDecoder(strings.NewReader(`[1,2,3]`)).DecodeMeta()
	if err != nil {
		t.Fatalf("Decode patch: %v", err)
	}

	if err := target.Merge(patch); err != nil {
		t.Fatalf("Merge: %v", err)
	}

	out, err := MarshalMeta(target)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	assertCanonicalJSONEqual(t, out, []byte(`[1,2,3]`))
}

func TestJSONPatchOperations(t *testing.T) {
	target, err := NewDecoder(strings.NewReader(`{"items":["a","b"],"meta":{"count":2}}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("Decode target: %v", err)
	}
	patch, err := DecodePatch(strings.NewReader(`[
		{"op":"test","path":"/meta/count","value":2.0},
		{"op":"test","path":"/meta/count","value":2e0},
		{"op":"add","path":"/items/-","value":"c"},
		{"op":"copy","from":"/items/0","path":"/first"},
		{"op":"move","from":"/meta/count","path":"/count"},
		{"op":"replace","path":"/items/1","value":"B"},
		{"op":"remove","path":"/meta"}
	]`))
	if err != nil {
		t.Fatalf("DecodePatch: %v", err)
	}

	if err := target.Patch(patch...); err != nil {
		t.Fatalf("Patch: %v", err)
	}

	out, err := MarshalMeta(target)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	assertCanonicalJSONEqual(t, out, []byte(`{"items":["a","B","c"],"first":"a","count":2}`))
}

func TestJSONPatchObjectAddReplacesExistingField(t *testing.T) {
	target, err := NewDecoder(strings.NewReader(`{"name":"old"}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("Decode target: %v", err)
	}

	if err := target.Root().Patch(Patch{Op: "add", Path: "/name", Value: "new"}); err != nil {
		t.Fatalf("Patch: %v", err)
	}

	out, err := MarshalMeta(target)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	assertCanonicalJSONEqual(t, out, []byte(`{"name":"new"}`))
}

func TestJSONPatchCopyFromDocumentRoot(t *testing.T) {
	target, err := NewDecoder(strings.NewReader(`{"name":"root"}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("Decode target: %v", err)
	}
	patch, err := DecodePatch(strings.NewReader(`[{"op":"copy","from":"","path":"/snapshot"}]`))
	if err != nil {
		t.Fatalf("DecodePatch: %v", err)
	}

	if err := target.Patch(patch...); err != nil {
		t.Fatalf("Patch: %v", err)
	}

	out, err := MarshalMeta(target)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	assertCanonicalJSONEqual(t, out, []byte(`{"name":"root","snapshot":{"name":"root"}}`))
}

func TestJSONPatchFailureIsAtomic(t *testing.T) {
	target, err := NewDecoder(strings.NewReader(`{"items":[1,2]}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("Decode target: %v", err)
	}
	patches := []Patch{
		{Op: "add", Path: "/items/-", Value: 3},
		{Op: "test", Path: "/items/0", Value: 9},
	}

	if err := target.Patch(patches...); !errors.Is(err, ErrPatchTestFailed) {
		t.Fatalf("Patch error = %v, want ErrPatchTestFailed", err)
	}

	out, err := MarshalMeta(target)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	assertCanonicalJSONEqual(t, out, []byte(`{"items":[1,2]}`))
}

func TestJSONPatchDecodedValuePreservesSyntax(t *testing.T) {
	target, err := NewJSON5Decoder(strings.NewReader(`{"items":[]}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("Decode target: %v", err)
	}
	patch, err := DecodePatch(strings.NewReader(`[
		{"op":"add","path":"/items/-","value":{/* value */unquoted: 'yes'}}
	]`))
	if err != nil {
		t.Fatalf("DecodePatch: %v", err)
	}

	if err := target.Patch(patch...); err != nil {
		t.Fatalf("Patch: %v", err)
	}

	out, err := MarshalMeta(target)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	if !strings.Contains(string(out), "{/* value */unquoted: 'yes'}") {
		t.Fatalf("patched output did not preserve decoded value syntax:\n%s", out)
	}
}

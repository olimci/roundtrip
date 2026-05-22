package json

import (
	"errors"
	"strings"
	"testing"
)

func TestNodeAt(t *testing.T) {
	m, err := NewJSON5Decoder(strings.NewReader(`{
		"items": [
			{"name": "first"},
			{"name": "second"}
		],
		"0": "object key"
	}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	root := m.Root()
	node, err := root.At("items", 1, "name")
	if err != nil {
		t.Fatalf("At: %v", err)
	}
	var got string
	if err := node.Decode(&got); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got != "second" {
		t.Fatalf("At decoded %q", got)
	}

	node, err = root.JSONPointer("/0")
	if err != nil {
		t.Fatalf("JSONPointer object key: %v", err)
	}
	if err := node.Decode(&got); err != nil {
		t.Fatalf("Decode object key: %v", err)
	}
	if got != "object key" {
		t.Fatalf("JSONPointer decoded %q", got)
	}
}

func TestJSONPointerEscaping(t *testing.T) {
	m, err := NewDecoder(strings.NewReader(`{"a/b":{"~key":[10]}}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	root := m.Root()
	node, err := root.JSONPointer("/a~1b/~0key/0")
	if err != nil {
		t.Fatalf("JSONPointer: %v", err)
	}
	var got int
	if err := node.Decode(&got); err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if got != 10 {
		t.Fatalf("JSONPointer decoded %d", got)
	}

	if _, err := root.JSONPointer("a/b"); !errors.Is(err, ErrInvalidJSONPointer) {
		t.Fatalf("missing leading slash error = %v", err)
	}
	if _, err := root.JSONPointer("/a~2b"); !errors.Is(err, ErrInvalidJSONPointer) {
		t.Fatalf("bad escape error = %v", err)
	}
	if _, err := root.JSONPointer("/a~1b/~0key/-"); !errors.Is(err, ErrInvalidAppend) {
		t.Fatalf("read append error = %v", err)
	}
}

func TestPathMutations(t *testing.T) {
	m, err := NewDecoder(strings.NewReader(`{
		"items": [
			{"name": "first"},
			{"name": "second"}
		]
	}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	root := m.Root()
	if err := root.ReplaceAt("updated", "items", 1, "name"); err != nil {
		t.Fatalf("ReplaceAt: %v", err)
	}
	if err := root.InsertAt(map[string]string{"name": "inserted"}, "items", 1); err != nil {
		t.Fatalf("InsertAt array: %v", err)
	}
	if err := root.InsertAt(true, "enabled"); err != nil {
		t.Fatalf("InsertAt object: %v", err)
	}
	if err := root.RemoveAt("items", 0); err != nil {
		t.Fatalf("RemoveAt: %v", err)
	}

	out, err := MarshalMeta(m)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	assertCanonicalJSONEqual(t, out, []byte(`{
		"items": [
			{"name": "inserted"},
			{"name": "updated"}
		],
		"enabled": true
	}`))
}

func TestJSONPointerMutations(t *testing.T) {
	m, err := NewDecoder(strings.NewReader(`{"items":[1,3],"meta":{"old":true}}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	root := m.Root()
	if err := root.InsertJSONPointer("/items/1", 2); err != nil {
		t.Fatalf("InsertJSONPointer index: %v", err)
	}
	if err := root.InsertJSONPointer("/items/-", 4); err != nil {
		t.Fatalf("InsertJSONPointer append: %v", err)
	}
	if err := root.ReplaceJSONPointer("/meta/old", false); err != nil {
		t.Fatalf("ReplaceJSONPointer: %v", err)
	}
	if err := root.RemoveJSONPointer("/items/0"); err != nil {
		t.Fatalf("RemoveJSONPointer: %v", err)
	}

	out, err := MarshalMeta(m)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	assertCanonicalJSONEqual(t, out, []byte(`{"items":[2,3,4],"meta":{"old":false}}`))
}

func TestPathErrors(t *testing.T) {
	m, err := NewDecoder(strings.NewReader(`{"items":[1]}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	root := m.Root()
	if _, err := root.At("missing"); !errors.Is(err, ErrObjectFieldNotFound) {
		t.Fatalf("missing field error = %v", err)
	}
	if _, err := root.At("items", 2); !errors.Is(err, ErrArrayIndexOutOfRange) {
		t.Fatalf("array range error = %v", err)
	}
	if _, err := root.At("items", "bad"); !errors.Is(err, ErrWrongNodeType) {
		t.Fatalf("wrong type error = %v", err)
	}
	if err := root.InsertAt(2, "items", Append); err != nil {
		t.Fatalf("InsertAt append sentinel: %v", err)
	}
	if err := root.ReplaceAt(3, "items", Append); !errors.Is(err, ErrInvalidAppend) {
		t.Fatalf("replace append error = %v", err)
	}
	if err := root.InsertAt(1, "items"); !errors.Is(err, ErrObjectFieldExists) {
		t.Fatalf("duplicate insert error = %v", err)
	}
	if err := root.RemoveAt(); !errors.Is(err, ErrEmptyPath) {
		t.Fatalf("empty remove error = %v", err)
	}
}

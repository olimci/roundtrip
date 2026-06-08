package json

import (
	"bytes"
	"strings"
	"testing"
)

func TestMetaCloneIsIndependent(t *testing.T) {
	m, err := NewJSONCDecoder(strings.NewReader(`// leading
{
  "a": 1,
  "b": 2,
}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	clone := m.Clone()
	if clone == m {
		t.Fatal("Clone returned original Meta")
	}
	if clone.SST.Root == m.SST.Root {
		t.Fatal("Clone reused root node")
	}
	if clone.SST.Root.Start == m.SST.Root.Start {
		t.Fatal("Clone reused root start token")
	}
	if got := clone.Comments().Leading.Text(); got != " leading" {
		t.Fatalf("clone leading comments = %q", got)
	}

	if err := clone.Root().ReplaceObjectField("a", 10); err != nil {
		t.Fatalf("clone ReplaceObjectField: %v", err)
	}
	if err := m.Root().ReplaceObjectField("b", 20); err != nil {
		t.Fatalf("original ReplaceObjectField: %v", err)
	}

	var cloneValue map[string]int
	if _, err := NewJSONCDecoder(bytes.NewReader(clone.Root().Bytes())).Decode(&cloneValue); err != nil {
		t.Fatalf("Decode clone: %v", err)
	}
	if cloneValue["a"] != 10 || cloneValue["b"] != 2 {
		t.Fatalf("clone value = %#v", cloneValue)
	}

	var originalValue map[string]int
	if _, err := NewJSONCDecoder(bytes.NewReader(m.Root().Bytes())).Decode(&originalValue); err != nil {
		t.Fatalf("Decode original: %v", err)
	}
	if originalValue["a"] != 1 || originalValue["b"] != 20 {
		t.Fatalf("original value = %#v", originalValue)
	}
}

func TestNodeCloneIsIndependent(t *testing.T) {
	m, err := NewJSONCDecoder(strings.NewReader(`{
  "items": [
    {"name": "first"},
    {"name": "second"},
  ]
}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	node, err := m.Root().At("items", 0)
	if err != nil {
		t.Fatalf("At: %v", err)
	}
	clone := node.Clone()
	if clone.node == node.node {
		t.Fatal("Clone reused node")
	}
	if clone.node.Start == node.node.Start {
		t.Fatal("Clone reused node token")
	}

	if err := clone.ReplaceObjectField("name", "clone"); err != nil {
		t.Fatalf("clone ReplaceObjectField: %v", err)
	}
	if err := node.ReplaceObjectField("name", "original"); err != nil {
		t.Fatalf("original ReplaceObjectField: %v", err)
	}

	var cloneValue map[string]string
	if err := clone.Decode(&cloneValue); err != nil {
		t.Fatalf("clone Decode: %v", err)
	}
	if cloneValue["name"] != "clone" {
		t.Fatalf("clone value = %#v", cloneValue)
	}

	var originalValue map[string]string
	if err := node.Decode(&originalValue); err != nil {
		t.Fatalf("original Decode: %v", err)
	}
	if originalValue["name"] != "original" {
		t.Fatalf("original value = %#v", originalValue)
	}
}

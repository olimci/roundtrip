package json

import (
	"strings"
	"testing"
)

func TestWrapperNodeAccessors(t *testing.T) {
	m, err := NewJSON5Decoder(strings.NewReader(`{"a":[1]}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	root := m.Root()
	children := root.Children()
	if len(children) != 1 || children[0].Type() != NodeTypeObjectField {
		t.Fatalf("object child type = %v, want one OBJECT_FIELD", children)
	}

	field, ok := root.ObjectFieldNode("a")
	if !ok {
		t.Fatal("missing object field node")
	}
	key, ok := field.Key()
	if !ok || key.Type() != NodeTypeString {
		t.Fatalf("field key = %v, %v; want STRING", key.Type(), ok)
	}
	value, ok := field.Value()
	if !ok || value.Type() != NodeTypeArray {
		t.Fatalf("field value = %v, %v; want ARRAY", value.Type(), ok)
	}

	elements := value.Children()
	if len(elements) != 1 || elements[0].Type() != NodeTypeArrayElement {
		t.Fatalf("array child type = %v, want one ARRAY_ELEMENT", elements)
	}
	element, ok := value.ArrayElement(0)
	if !ok || element.Type() != NodeTypeArrayElement {
		t.Fatalf("array element = %v, %v; want ARRAY_ELEMENT", element.Type(), ok)
	}
	item, ok := element.Value()
	if !ok || item.Type() != NodeTypeNumber {
		t.Fatalf("array element value = %v, %v; want NUMBER", item.Type(), ok)
	}
}

func TestCommentAccessors(t *testing.T) {
	input := `// file leading
{
  // field leading
  "a" /* key trailing */ : /* value leading */ 1 /* value trailing */,
  "b": [
    // element leading
    2 /* element trailing */
  ],
  // dangling
}`
	m, err := NewJSONCDecoder(strings.NewReader(input)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	if got := m.Comments().Leading.Text(); got != "file leading" {
		t.Fatalf("meta leading comments = %q", got)
	}

	field, ok := m.Root().ObjectFieldNode("a")
	if !ok {
		t.Fatal("missing field a")
	}
	if got := field.Comments().Leading.Text(); got != "field leading" {
		t.Fatalf("field leading comments = %q", got)
	}
	if got := field.Comments().Trailing.Text(); got != "value trailing" {
		t.Fatalf("field trailing comments = %q", got)
	}
	key, _ := field.Key()
	if got := key.Comments().Trailing.Text(); got != "key trailing" {
		t.Fatalf("key trailing comments = %q", got)
	}
	value, _ := field.Value()
	if got := value.Comments().Leading.Text(); got != "value leading" {
		t.Fatalf("value leading comments = %q", got)
	}
	if got := value.Comments().Trailing.Text(); got != "value trailing" {
		t.Fatalf("value trailing comments = %q", got)
	}

	array, ok := m.Root().ObjectField("b")
	if !ok {
		t.Fatal("missing field b")
	}
	element, ok := array.ArrayElement(0)
	if !ok {
		t.Fatal("missing array element")
	}
	if got := element.Comments().Leading.Text(); got != "element leading" {
		t.Fatalf("element leading comments = %q", got)
	}
	item, _ := element.Value()
	if got := item.Comments().Trailing.Text(); got != "element trailing" {
		t.Fatalf("element value trailing comments = %q", got)
	}
	c, ok := item.Comments().First()
	if !ok {
		t.Fatal("missing first element value comment")
	}
	if got := c.Text(); got != "element leading" {
		t.Fatalf("first element value comment = %q", got)
	}

	if got := m.Root().Comments().Dangling.Text(); got != "dangling" {
		t.Fatalf("root dangling comments = %q", got)
	}
}

func TestCommentTextJoinsSourceOrder(t *testing.T) {
	m, err := NewJSONCDecoder(strings.NewReader(`[
  // first
  // second
  1
]`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}
	item, ok := m.Root().ArrayValue(0)
	if !ok {
		t.Fatal("missing array value")
	}
	if got := item.Comments().Leading.Text(); got != "first\nsecond" {
		t.Fatalf("joined comments = %q", got)
	}
}

func TestReplaceInsideWrapperPreservesJSONDepth(t *testing.T) {
	m, err := NewJSONCDecoder(strings.NewReader(`{
  "items": [
    1
  ]
}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	if err := m.Root().ReplaceAt(map[string]int{"nested": 2}, "items", 0); err != nil {
		t.Fatalf("ReplaceAt: %v", err)
	}
	out, err := MarshalMeta(m)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	want := `{
  "items": [
    {
      "nested": 2
    }
  ]
}`
	if string(out) != want {
		t.Fatalf("replacement output:\ngot:\n%s\nwant:\n%s", out, want)
	}
}

func TestCommentsSurviveObjectValueReplacementAndRename(t *testing.T) {
	m, err := NewJSONCDecoder(strings.NewReader(`{
  "keep": 0,
  // field leading
  "name" /* key trailing */ : /* value leading */ 1 /* value trailing */
}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	if err := m.Root().ReplaceObjectField("name", map[string]int{"nested": 2}); err != nil {
		t.Fatalf("ReplaceObjectField: %v", err)
	}
	if err := m.Root().RenameObjectField("name", "renamed"); err != nil {
		t.Fatalf("RenameObjectField: %v", err)
	}

	field, ok := m.Root().ObjectFieldNode("renamed")
	if !ok {
		t.Fatal("missing renamed field")
	}
	if got := field.Comments().Leading.Text(); got != "field leading" {
		t.Fatalf("field leading comments = %q", got)
	}
	if got := field.Comments().Trailing.Text(); got != "value trailing" {
		t.Fatalf("field trailing comments = %q", got)
	}
	key, _ := field.Key()
	if got := key.Comments().Trailing.Text(); got != "key trailing" {
		t.Fatalf("key trailing comments = %q", got)
	}
	value, _ := field.Value()
	if got := value.Comments().Leading.Text(); got != "value leading" {
		t.Fatalf("value leading comments = %q", got)
	}

	out, err := MarshalMeta(m)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	if _, err := NewJSONCDecoder(strings.NewReader(string(out))).DecodeMeta(); err != nil {
		t.Fatalf("mutated output did not reparse:\n%s\n%v", out, err)
	}
}

func TestCommentsSurviveInsertAndSiblingRemoval(t *testing.T) {
	m, err := NewJSONCDecoder(strings.NewReader(`{
  // keep leading
  "keep": [
    // item leading
    1 /* item trailing */,
    // removed item leading
    2
  ] /* keep trailing */,
  "drop": 1
}`)).DecodeMeta()
	if err != nil {
		t.Fatalf("DecodeMeta: %v", err)
	}

	root := m.Root()
	if err := root.InsertObjectField("added", true); err != nil {
		t.Fatalf("InsertObjectField: %v", err)
	}
	if err := root.RemoveObjectField("drop"); err != nil {
		t.Fatalf("RemoveObjectField: %v", err)
	}

	keep, ok := root.ObjectFieldNode("keep")
	if !ok {
		t.Fatal("missing keep field")
	}
	if got := keep.Comments().Leading.Text(); got != "keep leading" {
		t.Fatalf("keep leading comments = %q", got)
	}
	if got := keep.Comments().Trailing.Text(); got != "keep trailing" {
		t.Fatalf("keep trailing comments = %q", got)
	}

	array, _ := root.ObjectField("keep")
	item, ok := array.ArrayValue(0)
	if !ok {
		t.Fatal("missing remaining array item")
	}
	if got := item.Comments().Leading.Text(); got != "item leading" {
		t.Fatalf("item leading comments = %q", got)
	}
	if got := item.Comments().Trailing.Text(); got != "item trailing" {
		t.Fatalf("item trailing comments = %q", got)
	}
	if err := array.RemoveArrayValue(1); err != nil {
		t.Fatalf("RemoveArrayValue: %v", err)
	}

	out, err := MarshalMeta(m)
	if err != nil {
		t.Fatalf("MarshalMeta: %v", err)
	}
	if _, err := NewJSONCDecoder(strings.NewReader(string(out))).DecodeMeta(); err != nil {
		t.Fatalf("mutated output did not reparse:\n%s\n%v", out, err)
	}
}

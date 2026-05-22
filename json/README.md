# roundtrip/json

Package `json` provides a JSON, JSONC, and JSON5 parser that preserves source
text trivia so documents can be decoded, inspected, edited, patched, and written
back without discarding comments or formatting.

```go
import "github.com/olimci/roundtrip/json"
```

## Encoding and decoding

The high-level API mirrors the shape of `encoding/json`, but decode operations
also return a `*Meta` tree that keeps the parsed source structure.

```go
var dst any
m, err := json.Unmarshal(data, &dst)
out, err := json.Marshal(dst)
```

- `func Unmarshal(data []byte, v any) (*Meta, error)` parses one strict JSON
  value, decodes it into `v`, and returns the roundtrippable metadata tree.
- `func Marshal(v any) ([]byte, error)` encodes `v` as JSON.
- `func MarshalMeta(m *Meta) ([]byte, error)` writes the exact bytes represented
  by a metadata tree.
- `type Marshaler interface { MarshalJSON() ([]byte, error) }` is honored by
  encoders.
- `type Unmarshaler interface { UnmarshalJSON([]byte) error }` is honored by
  decoders.
- `type RawMessage []byte` implements `MarshalJSON` and `UnmarshalJSON`.
- `type Number string` preserves a number literal. It provides `String()`,
  `MarshalJSON()`, `Float64()`, and `Int64()`.

The encoder and decoder support the usual JSON struct tags:

- field names from `json:"name"`
- field exclusion with `json:"-"`
- `omitempty`
- `omitzero`, including `IsZero() bool` when available
- `string` for quoted bool, integer, unsigned integer, float, and string values

Maps may use string, signed integer, unsigned integer, or
`encoding.TextMarshaler` / `encoding.TextUnmarshaler` key types. Byte slices
encode as base64 strings.

## Streaming decoders

```go
d := json.NewJSON5Decoder(r)
d.UseNumber()

var dst any
m, err := d.Decode(&dst)
```

- `func NewDecoder(r io.Reader) *Decoder` creates a strict JSON decoder.
- `func NewJSONCDecoder(r io.Reader) *Decoder` enables JSONC syntax options.
- `func NewJSON5Decoder(r io.Reader) *Decoder` enables JSON5 syntax options.
- `func (d *Decoder) Decode(v any) (*Meta, error)` decodes the next value from a
  stream and returns its metadata tree. Whitespace/comment-only input returns
  `io.EOF`.
- `func (d *Decoder) DecodeMeta() (*Meta, error)` parses one value and requires
  end of input.
- `func (d *Decoder) UseNumber()` decodes numbers in `interface{}` values as
  `json.Number` instead of `float64`.
- `func (d *Decoder) DisallowUnknownFields()` rejects object members that do not
  match a destination struct field.
- `func (d *Decoder) More() bool` reports whether another value or container
  element remains.
- `func (d *Decoder) Buffered() io.Reader` returns buffered unread data.
- `func (d *Decoder) InputOffset() int64` returns the current input offset.
- `type Decoder struct { SyntaxOptions }` embeds `SyntaxOptions`, so callers may
  configure individual syntax extensions directly.

## Streaming encoders

```go
e := json.NewEncoder(w)
e.SetIndent("", "  ")
e.SetEscapeHTML(false)
err := e.Encode(v)
```

- `func NewEncoder(w io.Writer) *Encoder` creates a strict JSON encoder.
- `func NewJSON5Encoder(w io.Writer) *Encoder` creates an encoder with JSON5
  syntax options.
- `func (e *Encoder) Encode(v any) error` writes one encoded value.
- `func (e *Encoder) EncodeMeta(m *Meta) error` writes the exact bytes
  represented by `m`.
- `func (e *Encoder) SetIndent(prefix, indent string)` formats generated output.
- `func (e *Encoder) SetEscapeHTML(on bool)` controls escaping of `<`, `>`, `&`,
  U+2028, and U+2029 in generated strings.
- `type Encoder struct { Indent string; Prefix string; SyntaxOptions }` exposes
  the same formatting and syntax controls as fields.

When JSON5 syntax options are enabled, the encoder may emit unquoted ECMAScript
identifier object keys, trailing commas, and IEEE 754 literals such as `NaN` and
`Infinity` where applicable.

## Syntax modes

`SyntaxOptions` controls optional syntax accepted by the parser and, where
applicable, emitted by the encoder.

```go
type SyntaxOptions struct {
	ECMAScriptIdentifiers          bool
	TrailingCommas                 bool
	SingleQuotedStrings            bool
	MultilineStrings               bool
	StringCharacterEscapes         bool
	HexadecimalNumbers             bool
	LeadingOrTrailingDecimalPoints bool
	LeadingPlusSigns               bool
	IEEE754Numbers                 bool
	SingleLineComments             bool
	MultilineComments              bool
	AdditionalWhitespace           bool
}
```

- `func JSONCSyntaxOptions() SyntaxOptions` enables trailing commas and `//` /
  `/* */` comments.
- `func JSON5SyntaxOptions() SyntaxOptions` enables the full JSON5 option set.

Strict JSON is the zero value.

## Metadata and nodes

`Meta` is the roundtrippable parse result. It owns the source token tree, the
root node, and the detected indentation style.

```go
m, err := json.NewJSONCDecoder(r).DecodeMeta()
root := m.Root()
```

- `type Meta struct { SST sst.SST[TokenType, NodeType]; Indent string }`
  exposes the underlying syntax tree and detected indent string.
- `func (m *Meta) Root() Node` returns the document root.
- `func (m *Meta) Nodes() iter.Seq[Node]` walks all nodes.
- `func (m *Meta) Leaves() iter.Seq[Node]` walks leaf nodes.
- `func (m *Meta) Comments() CommentSet` returns comments before or after the
  root value.

`Node` is a handle into a `Meta` tree.

- `func (n Node) Type() NodeType` returns the node kind.
- `func (n Node) Children() []Node` returns wrapper nodes for child entries.
- `func (n Node) Bytes() []byte` returns the original bytes for that node.
- `func (n Node) Decode(v any) error` decodes the node into `v`.
- `func (n Node) Replace(v any) error` replaces the node with an encoded Go
  value, another `Node`, or a `*Meta`.
- `func (n Node) Comments() CommentSet` returns leading, trailing, and dangling
  comments attached to the node.
- `func (n Node) TrailingComment() (Comment, bool)` returns the first trailing
  comment, if present.

Node kinds:

- `NodeTypeIllegal`
- `NodeTypeObject`
- `NodeTypeObjectField`
- `NodeTypeArray`
- `NodeTypeArrayElement`
- `NodeTypeString`
- `NodeTypeNumber`
- `NodeTypeBool`
- `NodeTypeNull`

`func (n NodeType) String() string` returns the symbolic node kind.

## Object editing

Object methods require a `NodeTypeObject` receiver unless noted.

- `func (n Node) ObjectField(name string) (Node, bool)` returns the field value.
- `func (n Node) ObjectFieldNode(name string) (Node, bool)` returns the
  `NodeTypeObjectField` wrapper.
- `func (n Node) ObjectFields() iter.Seq2[string, Node]` iterates field names
  and field wrapper nodes.
- `func (n Node) Key() (Node, bool)` returns an object field key node.
- `func (n Node) Value() (Node, bool)` returns the value of an object field or
  array element wrapper.
- `func (n Node) ReplaceObjectField(name string, value any) error` replaces an
  existing field value.
- `func (n Node) InsertObjectField(name string, value any) error` appends a new
  field.
- `func (n Node) RemoveObjectField(name string) error` removes a field.
- `func (n Node) RenameObjectField(oldName, newName string) error` replaces a
  field key while preserving the field value and surrounding trivia.

## Array editing

Array methods require a `NodeTypeArray` receiver unless noted.

- `func (n Node) ArrayValue(index int) (Node, bool)` returns an element value.
- `func (n Node) ArrayElement(index int) (Node, bool)` returns the
  `NodeTypeArrayElement` wrapper.
- `func (n Node) ReplaceArrayValue(index int, value any) error` replaces an
  existing element value.
- `func (n Node) InsertArrayValue(index int, value any) error` inserts before
  `index`; `index == len(array)` appends.
- `func (n Node) RemoveArrayValue(index int) error` removes an element.

## Path and JSON Pointer access

Path methods navigate with Go path segments:

- `string` selects an object field value.
- `int` selects an array value.
- `json.Append` may be used only by insert operations to append to an array.

```go
node, err := root.At("compilerOptions", "paths", 0)
err = root.InsertAt("new", "items", json.Append)
```

- `type AppendSegment struct{}`
- `var Append AppendSegment`
- `func (n Node) At(path ...any) (Node, error)` reads a path.
- `func (n Node) ReplaceAt(value any, path ...any) error` replaces a path.
- `func (n Node) InsertAt(value any, path ...any) error` inserts at a path.
- `func (n Node) RemoveAt(path ...any) error` removes a path.

JSON Pointer methods use RFC 6901 pointer strings:

- `func (n Node) JSONPointer(pointer string) (Node, error)` reads a pointer.
- `func (n Node) ReplaceJSONPointer(pointer string, value any) error` replaces a
  pointer target. The empty pointer replaces the receiver.
- `func (n Node) InsertJSONPointer(pointer string, value any) error` inserts at
  a pointer target. `/-` appends to arrays.
- `func (n Node) RemoveJSONPointer(pointer string) error` removes a pointer
  target.

## Comments

Comments are available when the document was parsed with JSONC or JSON5 comment
syntax enabled.

- `type Comment` represents one source comment.
- `func (c Comment) Text() string` returns the comment body without delimiters.
- `func (c Comment) ReplaceText(text string) error` replaces the body while
  preserving the original comment style.
- `type Comments []Comment`
- `func (cs Comments) First() (Comment, bool)` returns the first comment.
- `func (cs Comments) Text() string` joins comment text with newlines.
- `type CommentSet struct { Leading, Trailing, Dangling Comments }`
- `func (cs CommentSet) First() (Comment, bool)` returns the first leading,
  dangling, or trailing comment.
- `func (cs CommentSet) Text() string` joins all comment text in source order.
- `type CommentError struct { Err error; Token token; Text string }` wraps
  comment replacement validation errors.

## JSON Patch and Merge Patch

JSON Patch implements RFC 6902-style operations. Merge Patch implements RFC
7396-style object merging.

```go
patches, err := json.DecodePatch(r)
err = meta.Patch(patches...)
```

- `type Patch struct { Op, Path, From string; Value any }`
- `func DecodePatch(r io.Reader) ([]Patch, error)` parses a JSON5 patch
  document.
- `func (target Node) Patch(patches ...Patch) error` applies patches atomically
  to a target node.
- `func (m *Meta) Patch(patches ...Patch) error` applies patches atomically to
  document root.
- `func (target Node) Merge(patch Node) error` applies a merge patch atomically
  to a target node.
- `func (m *Meta) Merge(patch *Meta) error` applies a merge patch atomically to
  the document root.

Supported patch operations are `add`, `remove`, `replace`, `move`, `copy`, and
`test`. Patch `Value` may be a Go value, a `Node`, or a `*Meta`.

## Formatting helpers

- `func Valid(data []byte) bool` reports whether `data` is strict JSON.
- `func Compact(dst *bytes.Buffer, src []byte) error` removes whitespace from
  JSON5 input while preserving comments.
- `func Indent(dst *bytes.Buffer, src []byte, prefix, indent string) error`
  reformats JSON5 input while preserving comments and trailing whitespace.
- `func HTMLEscape(dst *bytes.Buffer, src []byte)` escapes `<`, `>`, `&`,
  U+2028, and U+2029 inside JSON string literals.

## Tokens

The syntax tree exposes token kinds through `Meta.SST`.

- `TokenAnchor`
- `TokenIllegal`
- `TokenEOF`
- `TokenIdentifier`
- `TokenNumber`
- `TokenString`
- `TokenColon`
- `TokenComma`
- `TokenLeftBrace`
- `TokenRightBrace`
- `TokenLeftBracket`
- `TokenRightBracket`
- `TokenWhitespace`
- `TokenNewline`
- `TokenComment`

`func (t TokenType) String() string` returns the symbolic token kind.

## Errors

Parsing sentinel errors:

- `ErrUnexpectedEOF`
- `ErrUnexpectedToken`
- `ErrInvalidString`
- `ErrInvalidNumber`
- `ErrInvalidSpace`

Tree editing sentinel errors:

- `ErrWrongNodeType`
- `ErrObjectFieldNotFound`
- `ErrObjectFieldExists`
- `ErrArrayIndexOutOfRange`

Path sentinel errors:

- `ErrEmptyPath`
- `ErrInvalidPathSegment`
- `ErrInvalidJSONPointer`
- `ErrInvalidAppend`

Comment sentinel errors:

- `ErrInvalidComment`

Patch sentinel errors:

- `ErrInvalidPatch`
- `ErrInvalidPatchOperation`
- `ErrPatchTestFailed`

Structured error types:

- `type ParseError struct { Err error; Token token }` reports syntax failures
  with source position and supports `errors.Is` through `Unwrap`.
- `type PathError struct { Op string; Index int; Segment any; Err error }`
  reports path, pointer, and patch operation failures and supports
  `errors.Is` through `Unwrap`.
- `type CommentError struct { Err error; Token token; Text string }` reports
  invalid comment replacement text and supports `errors.Is` through `Unwrap`.
- `type SyntaxError struct { Offset int64 }` matches the standard library error
  shape.
- `type UnmarshalTypeError struct { Value string; Type reflect.Type; Offset int64; Struct string; Field string }`
  reports incompatible decode targets.
- `type InvalidUnmarshalError struct { Type reflect.Type }` reports invalid
  decode destinations.
- `type UnsupportedTypeError struct { Type reflect.Type }`
- `type UnsupportedValueError struct { Value reflect.Value; Str string }`
- `type MarshalerError struct { Type reflect.Type; Err error }` wraps errors
  returned by `MarshalJSON`.

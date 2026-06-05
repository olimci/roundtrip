// Package json encodes, decodes, formats, and edits JSON while preserving the
// source document's syntax tree, whitespace, and comments.
//
// Decoding returns a *Meta alongside the decoded Go value. Meta and the Node
// values derived from it are live handles into the parsed document. Mutating a
// Node mutates its owning Meta. Node values are not detached snapshots; use
// MarshalMeta or EncodeMeta to observe the current document bytes.
//
// Unless a function states otherwise, pointer parameters must be non-nil and
// Node or Comment receivers must have been obtained from this package. A zero
// Node or Comment is not a valid receiver.
package json

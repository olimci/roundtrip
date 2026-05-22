package json

import (
	"errors"
	"io"
	"math/big"
	"strconv"
	"strings"

	"github.com/olimci/roundtrip/internal/sst"
)

var (
	ErrInvalidPatch          = errors.New("invalid JSON patch")
	ErrInvalidPatchOperation = errors.New("invalid JSON patch operation")
	ErrPatchTestFailed       = errors.New("JSON patch test failed")
)

type Patch struct {
	Op    string
	Path  string
	From  string
	Value any
}

func DecodePatch(r io.Reader) ([]Patch, error) {
	d := NewJSON5Decoder(r)
	m, err := d.DecodeMeta()
	if err != nil {
		return nil, err
	}
	return decodePatch(m)
}

func decodePatch(m *Meta) ([]Patch, error) {
	root := m.Root()
	if root.node.Type != NodeTypeArray {
		return nil, ErrInvalidPatch
	}

	patches := make([]Patch, 0, len(root.node.Children))
	for i, child := range root.node.Children {
		value := Node{meta: m, node: arrayElementValue(child)}
		if value.node.Type != NodeTypeObject {
			return nil, &PathError{Op: "patch", Index: i, Err: ErrInvalidPatchOperation}
		}
		patch, err := unmarshalPatch(value)
		if err != nil {
			return nil, &PathError{Op: "patch", Index: i, Err: err}
		}
		patches = append(patches, patch)
	}
	return patches, nil
}

func (target Node) Patch(patches ...Patch) error {
	working := cloneMetaFromNode(target)
	root := working.Root()
	if err := applyPatch(root, patches); err != nil {
		return err
	}
	return target.replaceWithNode(root)
}

func (m *Meta) Patch(patches ...Patch) error {
	working := cloneMetaFromNode(m.Root())
	root := working.Root()
	if err := applyPatch(root, patches); err != nil {
		return err
	}
	m.SST = working.SST
	m.Indent = working.Indent
	m.syntax = working.syntax
	return nil
}

func unmarshalPatch(n Node) (Patch, error) {
	var patch Patch
	var hasOp, hasPath, hasFrom, hasValue bool
	for name, field := range n.ObjectFields() {
		value, _ := field.Value()
		switch name {
		case "op":
			if err := value.Decode(&patch.Op); err != nil {
				return Patch{}, err
			}
			hasOp = true
		case "path":
			if err := value.Decode(&patch.Path); err != nil {
				return Patch{}, err
			}
			hasPath = true
		case "from":
			if err := value.Decode(&patch.From); err != nil {
				return Patch{}, err
			}
			hasFrom = true
		case "value":
			patch.Value = value
			hasValue = true
		}
	}
	if !hasOp || !hasPath {
		return Patch{}, ErrInvalidPatchOperation
	}
	switch patch.Op {
	case "add", "replace", "test":
		if !hasValue {
			return Patch{}, ErrInvalidPatchOperation
		}
	case "move", "copy":
		if !hasFrom {
			return Patch{}, ErrInvalidPatchOperation
		}
	}
	if err := validatePatch(patch); err != nil {
		return Patch{}, err
	}
	return patch, nil
}

func validatePatch(patch Patch) error {
	if patch.Op == "" {
		return ErrInvalidPatchOperation
	}
	if _, err := parseJSONPointer(patch.Path); err != nil {
		return err
	}
	switch patch.Op {
	case "add", "replace", "test":
	case "remove":
	case "move", "copy":
		if _, err := parseJSONPointer(patch.From); err != nil {
			return err
		}
	default:
		return ErrInvalidPatchOperation
	}
	return nil
}

func applyPatch(root Node, patches []Patch) error {
	for i, patch := range patches {
		if err := validatePatch(patch); err != nil {
			return &PathError{Op: "patch", Index: i, Segment: patch.Op, Err: err}
		}
		if err := applyPatchOperation(root, patch); err != nil {
			return &PathError{Op: "patch", Index: i, Segment: patch.Op, Err: err}
		}
	}
	return nil
}

func applyPatchOperation(root Node, patch Patch) error {
	switch patch.Op {
	case "add":
		return patchAdd(root, patch.Path, patch.Value)
	case "remove":
		return root.RemoveJSONPointer(patch.Path)
	case "replace":
		return root.ReplaceJSONPointer(patch.Path, patch.Value)
	case "move":
		return patchMove(root, patch.From, patch.Path)
	case "copy":
		value, err := root.JSONPointer(patch.From)
		if err != nil {
			return err
		}
		return patchAdd(root, patch.Path, cloneMetaFromNode(value).Root())
	case "test":
		value, err := root.JSONPointer(patch.Path)
		if err != nil {
			return err
		}
		testValue, err := patchValueNode(root, patch.Value)
		if err != nil {
			return err
		}
		if !jsonNodesEqual(value, testValue) {
			return ErrPatchTestFailed
		}
		return nil
	}
	return ErrInvalidPatchOperation
}

func patchAdd(root Node, pointer string, value any) error {
	tokens, err := parseJSONPointer(pointer)
	if err != nil {
		return &PathError{Op: "add", Index: -1, Segment: pointer, Err: err}
	}
	if len(tokens) == 0 {
		return root.Replace(value)
	}
	parent, err := root.pointerParent("add", tokens)
	if err != nil {
		return err
	}
	index := len(tokens) - 1
	token := tokens[index]
	switch parent.node.Type {
	case NodeTypeObject:
		if _, exists := parent.ObjectField(token); exists {
			return parent.ReplaceObjectField(token, value)
		}
		return parent.InsertObjectField(token, value)
	case NodeTypeArray:
		if token == "-" {
			return parent.InsertArrayValue(len(parent.node.Children), value)
		}
		arrayIndex, err := pointerArrayIndex(token, false)
		if err != nil {
			return &PathError{Op: "add", Index: index, Segment: token, Err: err}
		}
		return parent.InsertArrayValue(arrayIndex, value)
	default:
		return &PathError{Op: "add", Index: index, Segment: token, Err: ErrWrongNodeType}
	}
}

func patchMove(root Node, from, path string) error {
	if from == path {
		return nil
	}
	if strings.HasPrefix(path, from+"/") {
		return ErrInvalidPatchOperation
	}
	value, err := root.JSONPointer(from)
	if err != nil {
		return err
	}
	clone := cloneMetaFromNode(value).Root()
	if err := root.RemoveJSONPointer(from); err != nil {
		return err
	}
	return patchAdd(root, path, clone)
}

func patchValueNode(root Node, value any) (Node, error) {
	if n, ok := nodeValue(value); ok {
		return n, nil
	}
	m, err := encodeMeta(value, root.meta.Indent, root.meta.syntax)
	if err != nil {
		return Node{}, err
	}
	return m.Root(), nil
}

func cloneMetaFromNode(n Node) *Meta {
	root, tokens := n.node.Clone()
	return &Meta{
		SST:    sst.SST[TokenType, NodeType]{Tokens: tokens, Root: root},
		Indent: n.meta.Indent,
		syntax: n.meta.syntax,
	}
}

func encodeMeta(value any, indent string, syntax SyntaxOptions) (*Meta, error) {
	n, tokens, err := encode(value, indent, 0)
	if err != nil {
		return nil, err
	}
	return &Meta{
		SST:    sst.SST[TokenType, NodeType]{Tokens: tokens, Root: n},
		Indent: indent,
		syntax: syntax,
	}, nil
}

func jsonNodesEqual(a, b Node) bool {
	if a.node.Type != b.node.Type {
		if a.node.Type == NodeTypeNumber && b.node.Type == NodeTypeNumber {
			return jsonNumbersEqual(a.node.Start.Value.Literal, b.node.Start.Value.Literal)
		}
		return false
	}
	switch a.node.Type {
	case NodeTypeObject:
		if len(a.node.Children) != len(b.node.Children) {
			return false
		}
		for name, aField := range a.ObjectFields() {
			bField, ok := b.ObjectField(name)
			if !ok {
				return false
			}
			aValue, _ := aField.Value()
			if !jsonNodesEqual(aValue, bField) {
				return false
			}
		}
		return true
	case NodeTypeArray:
		if len(a.node.Children) != len(b.node.Children) {
			return false
		}
		for i := range a.node.Children {
			aValue := Node{meta: a.meta, node: arrayElementValue(a.node.Children[i])}
			bValue := Node{meta: b.meta, node: arrayElementValue(b.node.Children[i])}
			if !jsonNodesEqual(aValue, bValue) {
				return false
			}
		}
		return true
	case NodeTypeString:
		var aString, bString string
		if err := a.Decode(&aString); err != nil {
			return false
		}
		if err := b.Decode(&bString); err != nil {
			return false
		}
		return aString == bString
	case NodeTypeNumber:
		return jsonNumbersEqual(a.node.Start.Value.Literal, b.node.Start.Value.Literal)
	default:
		return string(a.Bytes()) == string(b.Bytes())
	}
}

func jsonNumbersEqual(a, b string) bool {
	aRat, ok := jsonNumberRat(a)
	if !ok {
		return a == b
	}
	bRat, ok := jsonNumberRat(b)
	if !ok {
		return a == b
	}
	return aRat.Cmp(bRat) == 0
}

func jsonNumberRat(s string) (*big.Rat, bool) {
	sign := 1
	if after, ok := strings.CutPrefix(s, "-"); ok {
		sign = -1
		s = after
	} else if after, ok := strings.CutPrefix(s, "+"); ok {
		s = after
	}

	mantissa, exponentText, hasExponent := strings.Cut(s, "e")
	if !hasExponent {
		mantissa, exponentText, hasExponent = strings.Cut(s, "E")
	}
	exponent := 0
	if hasExponent {
		parsed, err := strconv.Atoi(exponentText)
		if err != nil {
			return nil, false
		}
		exponent = parsed
	}

	whole, fraction, hasFraction := strings.Cut(mantissa, ".")
	digits := whole
	scale := 0
	if hasFraction {
		digits += fraction
		scale = len(fraction)
	}
	if digits == "" {
		return nil, false
	}

	numerator := new(big.Int)
	if _, ok := numerator.SetString(digits, 10); !ok {
		return nil, false
	}
	if sign < 0 {
		numerator.Neg(numerator)
	}

	decimalPlaces := scale - exponent
	if decimalPlaces <= 0 {
		multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-decimalPlaces)), nil)
		numerator.Mul(numerator, multiplier)
		return new(big.Rat).SetInt(numerator), true
	}

	denominator := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimalPlaces)), nil)
	return new(big.Rat).SetFrac(numerator, denominator), true
}

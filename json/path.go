package json

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

var (
	ErrEmptyPath          = errors.New("empty path")
	ErrInvalidPathSegment = errors.New("invalid path segment")
	ErrInvalidJSONPointer = errors.New("invalid JSON pointer")
	ErrInvalidAppend      = errors.New("invalid append segment")
)

type AppendSegment struct{}

var Append AppendSegment

type PathError struct {
	Op      string
	Index   int
	Segment any
	Err     error
}

func (e *PathError) Error() string {
	if e.Index < 0 {
		return fmt.Sprintf("json: %s path: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("json: %s path segment %d (%v): %v", e.Op, e.Index, e.Segment, e.Err)
}

func (e *PathError) Unwrap() error {
	return e.Err
}

func (n Node) At(path ...any) (Node, error) {
	current := n
	for i, segment := range path {
		switch segment := segment.(type) {
		case string:
			child, ok := current.ObjectField(segment)
			if !ok {
				if current.node.Type != NodeTypeObject {
					return Node{}, &PathError{Op: "access", Index: i, Segment: segment, Err: ErrWrongNodeType}
				}
				return Node{}, &PathError{Op: "access", Index: i, Segment: segment, Err: ErrObjectFieldNotFound}
			}
			current = child
		case int:
			child, ok := current.ArrayValue(segment)
			if !ok {
				if current.node.Type != NodeTypeArray {
					return Node{}, &PathError{Op: "access", Index: i, Segment: segment, Err: ErrWrongNodeType}
				}
				return Node{}, &PathError{Op: "access", Index: i, Segment: segment, Err: ErrArrayIndexOutOfRange}
			}
			current = child
		case AppendSegment:
			return Node{}, &PathError{Op: "access", Index: i, Segment: segment, Err: ErrInvalidAppend}
		default:
			return Node{}, &PathError{Op: "access", Index: i, Segment: segment, Err: ErrInvalidPathSegment}
		}
	}
	return current, nil
}

func (n Node) ReplaceAt(value any, path ...any) error {
	if len(path) == 0 {
		return n.Replace(value)
	}
	parent, err := n.At(path[:len(path)-1]...)
	if err != nil {
		return err
	}
	index := len(path) - 1
	segment := path[index]
	var pathErr error
	switch segment := segment.(type) {
	case string:
		pathErr = parent.ReplaceObjectField(segment, value)
	case int:
		pathErr = parent.ReplaceArrayValue(segment, value)
	case AppendSegment:
		pathErr = ErrInvalidAppend
	default:
		pathErr = ErrInvalidPathSegment
	}
	if pathErr != nil {
		return &PathError{Op: "replace", Index: index, Segment: segment, Err: pathErr}
	}
	return nil
}

func (n Node) InsertAt(value any, path ...any) error {
	if len(path) == 0 {
		return &PathError{Op: "insert", Index: -1, Err: ErrEmptyPath}
	}
	parent, err := n.At(path[:len(path)-1]...)
	if err != nil {
		return err
	}
	index := len(path) - 1
	segment := path[index]
	var pathErr error
	switch segment := segment.(type) {
	case string:
		if parent.node.Type != NodeTypeObject {
			pathErr = ErrWrongNodeType
			break
		}
		if _, exists := parent.ObjectField(segment); exists {
			pathErr = ErrObjectFieldExists
			break
		}
		pathErr = parent.InsertObjectField(segment, value)
	case int:
		pathErr = parent.InsertArrayValue(segment, value)
	case AppendSegment:
		pathErr = parent.InsertArrayValue(len(parent.node.Children), value)
	default:
		pathErr = ErrInvalidPathSegment
	}
	if pathErr != nil {
		return &PathError{Op: "insert", Index: index, Segment: segment, Err: pathErr}
	}
	return nil
}

func (n Node) RemoveAt(path ...any) error {
	if len(path) == 0 {
		return &PathError{Op: "remove", Index: -1, Err: ErrEmptyPath}
	}
	parent, err := n.At(path[:len(path)-1]...)
	if err != nil {
		return err
	}
	index := len(path) - 1
	segment := path[index]
	var pathErr error
	switch segment := segment.(type) {
	case string:
		pathErr = parent.RemoveObjectField(segment)
	case int:
		pathErr = parent.RemoveArrayValue(segment)
	case AppendSegment:
		pathErr = ErrInvalidAppend
	default:
		pathErr = ErrInvalidPathSegment
	}
	if pathErr != nil {
		return &PathError{Op: "remove", Index: index, Segment: segment, Err: pathErr}
	}
	return nil
}

func (n Node) JSONPointer(pointer string) (Node, error) {
	tokens, err := parseJSONPointer(pointer)
	if err != nil {
		return Node{}, &PathError{Op: "access", Index: -1, Segment: pointer, Err: err}
	}
	current := n
	for i, token := range tokens {
		next, err := current.pointerToken(token, false)
		if err != nil {
			return Node{}, &PathError{Op: "access", Index: i, Segment: token, Err: err}
		}
		current = next
	}
	return current, nil
}

func (n Node) ReplaceJSONPointer(pointer string, value any) error {
	tokens, err := parseJSONPointer(pointer)
	if err != nil {
		return &PathError{Op: "replace", Index: -1, Segment: pointer, Err: err}
	}
	if len(tokens) == 0 {
		return n.Replace(value)
	}
	parent, err := n.pointerParent("replace", tokens)
	if err != nil {
		return err
	}
	index := len(tokens) - 1
	token := tokens[index]
	switch parent.node.Type {
	case NodeTypeObject:
		if err := parent.ReplaceObjectField(token, value); err != nil {
			return &PathError{Op: "replace", Index: index, Segment: token, Err: err}
		}
	case NodeTypeArray:
		arrayIndex, err := pointerArrayIndex(token, false)
		if err != nil {
			return &PathError{Op: "replace", Index: index, Segment: token, Err: err}
		}
		if err := parent.ReplaceArrayValue(arrayIndex, value); err != nil {
			return &PathError{Op: "replace", Index: index, Segment: token, Err: err}
		}
	default:
		return &PathError{Op: "replace", Index: index, Segment: token, Err: ErrWrongNodeType}
	}
	return nil
}

func (n Node) InsertJSONPointer(pointer string, value any) error {
	tokens, err := parseJSONPointer(pointer)
	if err != nil {
		return &PathError{Op: "insert", Index: -1, Segment: pointer, Err: err}
	}
	if len(tokens) == 0 {
		return &PathError{Op: "insert", Index: -1, Err: ErrEmptyPath}
	}
	parent, err := n.pointerParent("insert", tokens)
	if err != nil {
		return err
	}
	index := len(tokens) - 1
	token := tokens[index]
	switch parent.node.Type {
	case NodeTypeObject:
		if _, exists := parent.ObjectField(token); exists {
			return &PathError{Op: "insert", Index: index, Segment: token, Err: ErrObjectFieldExists}
		}
		if err := parent.InsertObjectField(token, value); err != nil {
			return &PathError{Op: "insert", Index: index, Segment: token, Err: err}
		}
	case NodeTypeArray:
		if token == "-" {
			if err := parent.InsertArrayValue(len(parent.node.Children), value); err != nil {
				return &PathError{Op: "insert", Index: index, Segment: token, Err: err}
			}
			return nil
		}
		arrayIndex, err := pointerArrayIndex(token, false)
		if err != nil {
			return &PathError{Op: "insert", Index: index, Segment: token, Err: err}
		}
		if err := parent.InsertArrayValue(arrayIndex, value); err != nil {
			return &PathError{Op: "insert", Index: index, Segment: token, Err: err}
		}
	default:
		return &PathError{Op: "insert", Index: index, Segment: token, Err: ErrWrongNodeType}
	}
	return nil
}

func (n Node) RemoveJSONPointer(pointer string) error {
	tokens, err := parseJSONPointer(pointer)
	if err != nil {
		return &PathError{Op: "remove", Index: -1, Segment: pointer, Err: err}
	}
	if len(tokens) == 0 {
		return &PathError{Op: "remove", Index: -1, Err: ErrEmptyPath}
	}
	parent, err := n.pointerParent("remove", tokens)
	if err != nil {
		return err
	}
	index := len(tokens) - 1
	token := tokens[index]
	switch parent.node.Type {
	case NodeTypeObject:
		if err := parent.RemoveObjectField(token); err != nil {
			return &PathError{Op: "remove", Index: index, Segment: token, Err: err}
		}
	case NodeTypeArray:
		arrayIndex, err := pointerArrayIndex(token, false)
		if err != nil {
			return &PathError{Op: "remove", Index: index, Segment: token, Err: err}
		}
		if err := parent.RemoveArrayValue(arrayIndex); err != nil {
			return &PathError{Op: "remove", Index: index, Segment: token, Err: err}
		}
	default:
		return &PathError{Op: "remove", Index: index, Segment: token, Err: ErrWrongNodeType}
	}
	return nil
}

func (n Node) pointerParent(op string, tokens []string) (Node, error) {
	current := n
	for i, token := range tokens[:len(tokens)-1] {
		next, err := current.pointerToken(token, false)
		if err != nil {
			return Node{}, &PathError{Op: op, Index: i, Segment: token, Err: err}
		}
		current = next
	}
	return current, nil
}

func (n Node) pointerToken(token string, allowAppend bool) (Node, error) {
	switch n.node.Type {
	case NodeTypeObject:
		child, ok := n.ObjectField(token)
		if !ok {
			return Node{}, ErrObjectFieldNotFound
		}
		return child, nil
	case NodeTypeArray:
		index, err := pointerArrayIndex(token, allowAppend)
		if err != nil {
			return Node{}, err
		}
		child, ok := n.ArrayValue(index)
		if !ok {
			return Node{}, ErrArrayIndexOutOfRange
		}
		return child, nil
	default:
		return Node{}, ErrWrongNodeType
	}
}

func parseJSONPointer(pointer string) ([]string, error) {
	if pointer == "" {
		return nil, nil
	}
	if !strings.HasPrefix(pointer, "/") {
		return nil, ErrInvalidJSONPointer
	}
	raw := strings.Split(pointer[1:], "/")
	tokens := make([]string, len(raw))
	for i, token := range raw {
		unescaped, err := unescapeJSONPointerToken(token)
		if err != nil {
			return nil, err
		}
		tokens[i] = unescaped
	}
	return tokens, nil
}

func unescapeJSONPointerToken(token string) (string, error) {
	var b strings.Builder
	for i := 0; i < len(token); i++ {
		if token[i] != '~' {
			b.WriteByte(token[i])
			continue
		}
		if i+1 >= len(token) {
			return "", ErrInvalidJSONPointer
		}
		switch token[i+1] {
		case '0':
			b.WriteByte('~')
		case '1':
			b.WriteByte('/')
		default:
			return "", ErrInvalidJSONPointer
		}
		i++
	}
	return b.String(), nil
}

func pointerArrayIndex(token string, allowAppend bool) (int, error) {
	if token == "-" {
		if allowAppend {
			return 0, nil
		}
		return 0, ErrInvalidAppend
	}
	if token == "" {
		return 0, ErrInvalidPathSegment
	}
	index, err := strconv.Atoi(token)
	if err != nil || index < 0 {
		return 0, ErrInvalidPathSegment
	}
	return index, nil
}

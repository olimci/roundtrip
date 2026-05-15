package json

import (
	"strings"

	"github.com/olimci/roundtrip/internal/list"
)

func detectIndent(root *node) string {
	var indent string
	var ok bool
	walkIndent(root, 0, func(n *node, depth int) bool {
		if depth == 0 {
			return true
		}
		prefix, found := lineIndent(n.Start)
		if !found || prefix == "" {
			return true
		}
		if len(prefix)%depth != 0 {
			ok = false
			return false
		}
		candidate := prefix[:len(prefix)/depth]
		if strings.Repeat(candidate, depth) != prefix {
			ok = false
			return false
		}
		if indent == "" {
			indent = candidate
			ok = true
			return true
		}
		if candidate != indent {
			ok = false
			return false
		}
		return true
	})
	if !ok {
		return ""
	}
	return indent
}

func lineIndent(start *list.Elem[token]) (string, bool) {
	e := start.Prev
	indent := ""
	if e != nil && e.Value.Type == TokenWhitespace {
		indent = e.Value.Literal
		e = e.Prev
	}
	return indent, e != nil && e.Value.Type == TokenNewline
}

func nodeDepth(root, target *node, depth int) int {
	if root == target {
		return depth
	}
	for _, child := range root.Children {
		if d := nodeDepth(child, target, depth+1); d >= 0 {
			return d
		}
	}
	return -1
}

func walkIndent(n *node, depth int, yield func(*node, int) bool) bool {
	if !yield(n, depth) {
		return false
	}
	for _, child := range n.Children {
		if !walkIndent(child, depth+1, yield) {
			return false
		}
	}
	return true
}

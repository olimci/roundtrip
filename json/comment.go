package json

import (
	"errors"
	"fmt"
	"strings"

	"github.com/olimci/roundtrip/internal/list"
)

// ErrInvalidComment reports replacement text that cannot fit the existing
// comment form.
var ErrInvalidComment = errors.New("invalid comment")

// Comment is a live handle to one source comment token.
//
// Copying a Comment copies the handle. Mutating a Comment updates its owning
// Meta. A zero Comment is not a valid receiver.
type Comment struct {
	elem *list.Elem[token]
}

// Comments is an ordered group of comment handles.
type Comments []Comment

// CommentSet groups comments by their relationship to a node.
type CommentSet struct {
	Leading  Comments
	Trailing Comments
	Dangling Comments
}

// CommentError describes an invalid comment replacement.
type CommentError struct {
	Err   error
	Token token
	Text  string
}

// Error returns the formatted comment error.
func (e CommentError) Error() string {
	return fmt.Sprintf("%v at %v: %q", e.Err, e.Token, e.Text)
}

// Unwrap returns the underlying comment error.
func (e CommentError) Unwrap() error {
	return e.Err
}

// Text returns the comment text without the comment delimiters.
func (c Comment) Text() string {
	return commentText(c.elem.Value.Literal)
}

// ReplaceText replaces the comment text while preserving the existing comment
// form.
func (c Comment) ReplaceText(text string) error {
	lit, err := commentLiteralText(c.elem.Value.Literal, text)
	if err != nil {
		return CommentError{Err: err, Token: c.elem.Value, Text: text}
	}
	c.elem.Value.Literal = lit
	return nil
}

// First returns the first comment in cs.
func (cs Comments) First() (Comment, bool) {
	if len(cs) == 0 {
		return Comment{}, false
	}
	return cs[0], true
}

// Text joins the text of each comment in source order with newlines.
func (cs Comments) Text() string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = c.Text()
	}
	return strings.Join(parts, "\n")
}

// First returns the first leading, dangling, or trailing comment in that order.
func (cs CommentSet) First() (Comment, bool) {
	if c, ok := cs.Leading.First(); ok {
		return c, true
	}
	if c, ok := cs.Dangling.First(); ok {
		return c, true
	}
	return cs.Trailing.First()
}

// Text joins all comments in leading, dangling, then trailing order.
func (cs CommentSet) Text() string {
	comments := make(Comments, 0, len(cs.Leading)+len(cs.Trailing)+len(cs.Dangling))
	comments = append(comments, cs.Leading...)
	comments = append(comments, cs.Dangling...)
	comments = append(comments, cs.Trailing...)
	return comments.Text()
}

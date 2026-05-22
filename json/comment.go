package json

import (
	"errors"
	"fmt"
	"strings"

	"github.com/olimci/roundtrip/internal/list"
)

var ErrInvalidComment = errors.New("invalid comment")

type Comment struct {
	elem *list.Elem[token]
}

type Comments []Comment

type CommentSet struct {
	Leading  Comments
	Trailing Comments
	Dangling Comments
}

type CommentError struct {
	Err   error
	Token token
	Text  string
}

func (e CommentError) Error() string {
	return fmt.Sprintf("%v at %v: %q", e.Err, e.Token, e.Text)
}

func (e CommentError) Unwrap() error {
	return e.Err
}

func (c Comment) Text() string {
	return commentText(c.elem.Value.Literal)
}

func (c Comment) ReplaceText(text string) error {
	lit, err := commentLiteralText(c.elem.Value.Literal, text)
	if err != nil {
		return CommentError{Err: err, Token: c.elem.Value, Text: text}
	}
	c.elem.Value.Literal = lit
	return nil
}

func (cs Comments) First() (Comment, bool) {
	if len(cs) == 0 {
		return Comment{}, false
	}
	return cs[0], true
}

func (cs Comments) Text() string {
	parts := make([]string, len(cs))
	for i, c := range cs {
		parts[i] = c.Text()
	}
	return strings.Join(parts, "\n")
}

func (cs CommentSet) First() (Comment, bool) {
	if c, ok := cs.Leading.First(); ok {
		return c, true
	}
	if c, ok := cs.Dangling.First(); ok {
		return c, true
	}
	return cs.Trailing.First()
}

func (cs CommentSet) Text() string {
	comments := make(Comments, 0, len(cs.Leading)+len(cs.Trailing)+len(cs.Dangling))
	comments = append(comments, cs.Leading...)
	comments = append(comments, cs.Dangling...)
	comments = append(comments, cs.Trailing...)
	return comments.Text()
}

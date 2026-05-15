package json

import (
	"errors"
	"fmt"

	"github.com/olimci/roundtrip/internal/list"
)

var ErrInvalidComment = errors.New("invalid comment")

type Comment struct {
	elem *list.Elem[token]
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

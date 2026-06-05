package json

import "github.com/olimci/roundtrip/internal/sst"

type token = sst.Token[TokenType]

type node = sst.Node[TokenType, NodeType]

// TokenType identifies a lexical token in preserved source text.
type TokenType int8

const (
	// TokenAnchor is an internal zero-width marker used to delimit nodes.
	TokenAnchor TokenType = -1
	// TokenIllegal identifies an invalid token.
	TokenIllegal TokenType = iota
	// TokenEOF identifies the end of input.
	TokenEOF
	// TokenIdentifier identifies JSON identifiers such as true, false, null, or JSON5 names.
	TokenIdentifier
	// TokenNumber identifies a number literal.
	TokenNumber
	// TokenString identifies a string literal.
	TokenString
	// TokenColon identifies a colon.
	TokenColon
	// TokenComma identifies a comma.
	TokenComma
	// TokenDelim identifies a brace or bracket delimiter.
	TokenDelim
	// TokenWhitespace identifies horizontal whitespace.
	TokenWhitespace
	// TokenNewline identifies a newline.
	TokenNewline
	// TokenComment identifies a line or block comment.
	TokenComment
)

// String returns the display name for n.
func (n NodeType) String() string {
	return nodeSymbols[n]
}

// NodeType identifies a node in the parsed JSON syntax tree.
type NodeType int8

const (
	// NodeTypeIllegal identifies an invalid node.
	NodeTypeIllegal NodeType = iota
	// NodeTypeObject identifies a JSON object.
	NodeTypeObject
	// NodeTypeObjectField identifies an object field wrapper node.
	NodeTypeObjectField
	// NodeTypeArray identifies a JSON array.
	NodeTypeArray
	// NodeTypeArrayElement identifies an array element wrapper node.
	NodeTypeArrayElement
	// NodeTypeString identifies a string value.
	NodeTypeString
	// NodeTypeNumber identifies a number value.
	NodeTypeNumber
	// NodeTypeBool identifies a boolean value.
	NodeTypeBool
	// NodeTypeNull identifies null.
	NodeTypeNull
)

// String returns the display name for t.
func (t TokenType) String() string {
	if t == TokenAnchor {
		return "ANCHOR"
	}
	if int(t) < 0 || int(t) >= len(tokenSymbols) {
		return "ILLEGAL"
	}
	return tokenSymbols[t]
}

var nodeSymbols = []string{
	"ILLEGAL", "OBJECT", "OBJECT_FIELD", "ARRAY", "ARRAY_ELEMENT", "STRING", "NUMBER", "BOOL", "NULL",
}

var tokenSymbols = []string{
	"ILLEGAL", "EOF", "IDENTIFIER", "NUMBER", "STRING", "COLON", "COMMA", "DELIM", "WHITESPACE", "NEWLINE", "COMMENT",
}

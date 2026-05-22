package json

import "github.com/olimci/roundtrip/internal/sst"

type token = sst.Token[TokenType]

type node = sst.Node[TokenType, NodeType]

type TokenType int8

const (
	TokenAnchor  TokenType = -1
	TokenIllegal TokenType = iota
	TokenEOF
	TokenIdentifier
	TokenNumber
	TokenString
	TokenColon
	TokenComma
	TokenLeftBrace
	TokenRightBrace
	TokenLeftBracket
	TokenRightBracket
	TokenWhitespace
	TokenNewline
	TokenComment
)

func (n NodeType) String() string {
	return nodeSymbols[n]
}

type NodeType int8

const (
	NodeTypeIllegal NodeType = iota
	NodeTypeObject
	NodeTypeObjectField
	NodeTypeArray
	NodeTypeArrayElement
	NodeTypeString
	NodeTypeNumber
	NodeTypeBool
	NodeTypeNull
)

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
	"ILLEGAL", "EOF", "IDENTIFIER", "NUMBER", "STRING", "COLON", "COMMA", "LEFT_BRACE", "RIGHT_BRACE", "LEFT_BRACKET", "RIGHT_BRACKET", "WHITESPACE", "NEWLINE", "COMMENT",
}

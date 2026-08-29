package lexer

// TokenKind identifies the lexical category of a token.
type TokenKind int

const (
	EOF TokenKind = iota
	Ident
	IntLit
	StringLit
	True
	False

	// Keywords
	Fn
	Let
	If
	Else
	While
	Match
	Return
	Struct
	Enum
	Trait
	Impl
	Self
	Pub
	Mod
	Use
	As
	For
	Lifetime
	I32
	Bool
	Unsafe

	// Operators
	Plus
	Minus
	Star
	Slash
	Eq
	EqEq
	NotEq
	Lt
	Gt
	Bang
	And
	Or
	Pipe
	Percent

	// Punctuation
	Hash
	Dollar
	Question
	LParen
	RParen
	LBrace
	RBrace
	LBracket
	RBracket
	Semi
	Comma
	Colon
	ColonColon
	Dot
	Arrow    // ->
	FatArrow // =>
)

// Token represents a single lexical token.
type Token struct {
	Kind TokenKind
	Text string
	Pos  int // byte offset in source
}

var keywords = map[string]TokenKind{
	"fn":     Fn,
	"let":    Let,
	"if":     If,
	"else":   Else,
	"while":  While,
	"match":  Match,
	"return": Return,
	"struct": Struct,
	"enum":   Enum,
	"trait":  Trait,
	"impl":   Impl,
	"self":   Self,
	"Self":   Self,
	"pub":    Pub,
	"mod":    Mod,
	"use":    Use,
	"as":     As,
	"for":    For,
	"i32":    I32,
	"bool":   Bool,
	"unsafe": Unsafe,
	"true":   True,
	"false":  False,
}

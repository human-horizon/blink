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
	Return
	Struct
	Enum
	I32
	Bool

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

	// Punctuation
	LParen
	RParen
	LBrace
	RBrace
	LBracket
	RBracket
	Semi
	Comma
	Colon
	Dot
	Arrow // ->
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
	"return": Return,
	"struct": Struct,
	"enum":   Enum,
	"i32":    I32,
	"bool":   Bool,
	"true":   True,
	"false":  False,
}

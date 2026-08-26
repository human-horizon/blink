package lexer

import (
	"fmt"
	"unicode"
	"unicode/utf8"
)

// Lexer tokenizes Rust-like source.
type Lexer struct {
	src  []byte
	pos  int
	tok  Token
	err  error
}

// New creates a new lexer for the given source.
func New(src []byte) *Lexer {
	return &Lexer{src: src}
}

// Next returns the next token. At EOF, repeated calls return EOF tokens.
func (l *Lexer) Next() Token {
	if l.err != nil {
		return Token{Kind: EOF, Pos: l.pos}
	}
	l.skipWhitespace()
	start := l.pos
	if l.pos >= len(l.src) {
		return Token{Kind: EOF, Pos: start}
	}
	ch := l.src[l.pos]
	switch ch {
	case '+':
		l.pos++
		return Token{Kind: Plus, Text: "+", Pos: start}
	case '-':
		l.pos++
		if l.peek() == '>' {
			l.pos++
			return Token{Kind: Arrow, Text: "->", Pos: start}
		}
		return Token{Kind: Minus, Text: "-", Pos: start}
	case '*':
		l.pos++
		return Token{Kind: Star, Text: "*", Pos: start}
	case '/':
		l.pos++
		return Token{Kind: Slash, Text: "/", Pos: start}
	case '(':
		l.pos++
		return Token{Kind: LParen, Text: "(", Pos: start}
	case ')':
		l.pos++
		return Token{Kind: RParen, Text: ")", Pos: start}
	case '{':
		l.pos++
		return Token{Kind: LBrace, Text: "{", Pos: start}
	case '}':
		l.pos++
		return Token{Kind: RBrace, Text: "}", Pos: start}
	case '[':
		l.pos++
		return Token{Kind: LBracket, Text: "[", Pos: start}
	case ']':
		l.pos++
		return Token{Kind: RBracket, Text: "]", Pos: start}
	case ';':
		l.pos++
		return Token{Kind: Semi, Text: ";", Pos: start}
	case ',':
		l.pos++
		return Token{Kind: Comma, Text: ",", Pos: start}
	case ':':
		l.pos++
		return Token{Kind: Colon, Text: ":", Pos: start}
	case '.':
		l.pos++
		return Token{Kind: Dot, Text: ".", Pos: start}
	case '=':
		l.pos++
		if l.peek() == '=' {
			l.pos++
			return Token{Kind: EqEq, Text: "==", Pos: start}
		}
		return Token{Kind: Eq, Text: "=", Pos: start}
	case '!':
		l.pos++
		if l.peek() == '=' {
			l.pos++
			return Token{Kind: NotEq, Text: "!=", Pos: start}
		}
		return Token{Kind: Bang, Text: "!", Pos: start}
	case '<':
		l.pos++
		return Token{Kind: Lt, Text: "<", Pos: start}
	case '>':
		l.pos++
		return Token{Kind: Gt, Text: ">", Pos: start}
	case '|':
		l.pos++
		if l.peek() == '|' {
			l.pos++
			return Token{Kind: Or, Text: "||", Pos: start}
		}
		l.err = fmt.Errorf("unexpected character %q at offset %d", ch, start)
		return Token{Kind: EOF, Pos: start}
	case '&':
		l.pos++
		if l.peek() == '&' {
			l.pos++
			return Token{Kind: And, Text: "&&", Pos: start}
		}
		return Token{Kind: And, Text: "&", Pos: start}
	case '"':
		return l.readString(start)
	default:
		if unicode.IsDigit(rune(ch)) {
			return l.readNumber(start)
		}
		if unicode.IsLetter(rune(ch)) || ch == '_' {
			return l.readIdent(start)
		}
		l.err = fmt.Errorf("unexpected character %q at offset %d", ch, start)
		return Token{Kind: EOF, Pos: start}
	}
}

// Err returns the first lexing error, if any.
func (l *Lexer) Err() error { return l.err }

func (l *Lexer) peek() byte {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) skipWhitespace() {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == ' ' || ch == '\t' || ch == '\n' || ch == '\r' {
			l.pos++
		} else if ch == '/' && l.peekAt(l.pos+1) == '/' {
			// line comment
			l.pos += 2
			for l.pos < len(l.src) && l.src[l.pos] != '\n' {
				l.pos++
			}
		} else {
			break
		}
	}
}

func (l *Lexer) peekAt(i int) byte {
	if i >= len(l.src) {
		return 0
	}
	return l.src[i]
}

func (l *Lexer) readNumber(start int) Token {
	for l.pos < len(l.src) && unicode.IsDigit(rune(l.src[l.pos])) {
		l.pos++
	}
	return Token{Kind: IntLit, Text: string(l.src[start:l.pos]), Pos: start}
}

func (l *Lexer) readIdent(start int) Token {
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch)) || ch == '_' {
			l.pos++
		} else {
			break
		}
	}
	text := string(l.src[start:l.pos])
	kind, ok := keywords[text]
	if !ok {
		kind = Ident
	}
	return Token{Kind: kind, Text: text, Pos: start}
}

func (l *Lexer) readString(start int) Token {
	l.pos++ // opening "
	for l.pos < len(l.src) {
		ch := l.src[l.pos]
		if ch == '"' {
			l.pos++
			return Token{Kind: StringLit, Text: string(l.src[start:l.pos]), Pos: start}
		}
		if ch == '\\' {
			l.pos += 2
			continue
		}
		_, size := utf8.DecodeRune(l.src[l.pos:])
		l.pos += size
	}
	l.err = fmt.Errorf("unterminated string at offset %d", start)
	return Token{Kind: EOF, Pos: start}
}

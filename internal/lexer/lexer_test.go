package lexer

import (
	"testing"
)

func TestBasicTokens(t *testing.T) {
	src := []byte("fn main() -> i32 { let x = 42; }")
	l := New(src)
	var tokens []Token
	for {
		tok := l.Next()
		tokens = append(tokens, tok)
		if tok.Kind == EOF {
			break
		}
	}
	if l.Err() != nil {
		t.Fatalf("lex error: %v", l.Err())
	}
	want := []TokenKind{Fn, Ident, LParen, RParen, Arrow, I32, LBrace, Let, Ident, Eq, IntLit, Semi, RBrace, EOF}
	if len(tokens) != len(want) {
		t.Fatalf("got %d tokens, want %d", len(tokens), len(want))
	}
	for i, w := range want {
		if tokens[i].Kind != w {
			t.Fatalf("token %d: got %v, want %v", i, tokens[i].Kind, w)
		}
	}
}

func TestStringLit(t *testing.T) {
	src := []byte(`let s = "hello";`)
	l := New(src)
	var kinds []TokenKind
	for {
		tok := l.Next()
		kinds = append(kinds, tok.Kind)
		if tok.Kind == EOF {
			break
		}
	}
	if l.Err() != nil {
		t.Fatalf("lex error: %v", l.Err())
	}
	want := []TokenKind{Let, Ident, Eq, StringLit, Semi, EOF}
	if len(kinds) != len(want) {
		t.Fatalf("got %v, want %v", kinds, want)
	}
	for i, w := range want {
		if kinds[i] != w {
			t.Fatalf("token %d: got %v, want %v", i, kinds[i], w)
		}
	}
}

func TestCommentSkipped(t *testing.T) {
	src := []byte("// comment\nlet x = 1;")
	l := New(src)
	tok := l.Next()
	if tok.Kind != Let {
		t.Fatalf("expected let, got %v", tok.Kind)
	}
}

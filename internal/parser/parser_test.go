package parser

import (
	"testing"

	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/lexer"
)

func parse(src string) (*ast.File, error) {
	l := lexer.New([]byte(src))
	p := New(l, []byte(src))
	return p.ParseFile()
}

func TestParseFn(t *testing.T) {
	f, err := parse("fn add(a: i32, b: i32) -> i32 { a + b }")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(f.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(f.Decls))
	}
}

func TestParseStruct(t *testing.T) {
	_, err := parse("struct Point { x: i32, y: i32 }")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseGenericFn(t *testing.T) {
	_, err := parse("fn identity<T>(x: T) -> T { x }")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseGenericStruct(t *testing.T) {
	_, err := parse("struct Pair<T, U> { first: T, second: U }")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseTypeArgs(t *testing.T) {
	_, err := parse("fn main() { let p: Pair<i32, bool> = Pair { first: 1, second: true }; }")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseInvalid(t *testing.T) {
	_, err := parse("fn foo { }")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

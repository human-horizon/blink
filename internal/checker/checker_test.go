package checker

import (
	"testing"

	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/diag"
	"github.com/humanhorizon/blink/internal/lexer"
	"github.com/humanhorizon/blink/internal/parser"
)

func parse(src string) *ast.File {
	l := lexer.New([]byte(src))
	p := parser.New(l, []byte(src))
	f, err := p.ParseFile()
	if err != nil {
		panic(err)
	}
	return f
}

func TestValidArithmetic(t *testing.T) {
	src := `
fn main() {
    let x: i32 = 1 + 2;
    let y = x * 3;
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestInvalidReturnType(t *testing.T) {
	src := `
fn foo() -> bool {
    42
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected type error")
	}
}

func TestUnknownVariable(t *testing.T) {
	src := `
fn main() {
    let x = y;
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected type error")
	}
}

func TestStructLit(t *testing.T) {
	src := `
struct Point { x: i32, y: i32 }
fn origin() -> Point {
    Point { x: 0, y: 0 }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestArrayIndex(t *testing.T) {
	src := `
fn first() -> i32 {
    let a = [10, 20, 30];
    a[0]
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

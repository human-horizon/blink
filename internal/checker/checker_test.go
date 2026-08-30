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

func TestGenericFunction(t *testing.T) {
	src := `
fn identity<T>(x: T) -> T {
    x
}

fn main() -> i32 {
    identity(42)
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestGenericStruct(t *testing.T) {
	src := `
struct Pair<T, U> {
    first: T,
    second: U,
}

fn main() -> i32 {
    let p: Pair<i32, bool> = Pair { first: 1, second: true };
    p.first
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestGenericFunctionReturn(t *testing.T) {
	src := `
fn make_pair<T, U>(a: T, b: U) -> Pair<T, U> {
    Pair { first: a, second: b }
}

struct Pair<T, U> {
    first: T,
    second: U,
}

fn main() -> i32 {
    let p = make_pair(1, true);
    p.first
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestGenericMismatch(t *testing.T) {
	src := `
fn identity<T>(x: T) -> T {
    x
}

fn main() -> bool {
    identity(42)
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected type error")
	}
}

func TestSharedBorrow(t *testing.T) {
	src := `
fn main() -> i32 {
    let mut x: i32 = 1;
    let a = &x;
    let b = &x;
    *a + *b
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestMutableBorrowBlocksShared(t *testing.T) {
	src := `
fn main() {
    let mut x: i32 = 1;
    let a = &mut x;
    let b = &x;
    *a = *b;
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected borrow error")
	}
}

func TestUseAfterMove(t *testing.T) {
	src := `
struct Point { x: i32, y: i32 }

fn main() -> i32 {
    let p = Point { x: 1, y: 2 };
    let q = p;
    p.x
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected borrow error")
	}
}

func TestAssignment(t *testing.T) {
	src := `
fn main() -> i32 {
    let mut x: i32 = 1;
    x = 5;
    x
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestBuiltinIndexTraitImpl(t *testing.T) {
	src := `
struct Table { value: i32 }

impl Index<i32> for Table {
    fn index(&self, index: i32) -> i32 {
        self.value
    }
}

fn main() -> i32 {
    let table = Table { value: 7 };
    table.index(0)
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestMutSelfBorrow(t *testing.T) {
	src := `
struct Counter { value: i32 }

impl Counter {
    fn bump(&mut self) {
        self.value = self.value + 1;
    }
    fn get(&self) -> i32 {
        self.value
    }
}

fn main() -> i32 {
    let mut c = Counter { value: 0 };
    c.bump();
    c.get()
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}
func TestImplTraitReturn(t *testing.T) {
	src := `
fn main() -> i32 {
    let x = [1, 2, 3];
    let it = x.iter();
    let n = it.count();
    n
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestForLoopPatternBinding(t *testing.T) {
	src := `
fn main() -> i32 {
    let x = [1, 2, 3];
    let mut total = 0;
    for item in x {
        total = total + item;
    }
    for (idx, _v) in x.enumerate() {
        total = total + idx;
    }
    total
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

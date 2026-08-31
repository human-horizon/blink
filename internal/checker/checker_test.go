package checker

import (
	"testing"

	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/diag"
	"github.com/humanhorizon/blink/internal/lexer"
	"github.com/humanhorizon/blink/internal/parser"
	"github.com/humanhorizon/blink/internal/types"
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

func TestSequentialMutBorrows(t *testing.T) {
	src := `
fn main() -> i32 {
    let mut flags = 0;
    let mut i = 0;
    while i < 2 {
        update(&mut flags);
        update(&mut flags);
        i = i + 1;
    }
    flags
}

fn update(f: &mut i32) {
    *f = *f + 1;
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

func TestBuiltinStubReturnsPlaceholderForNilRet(t *testing.T) {
	// Builtin stub without declared return must not produce a Go-nil interface.
	if got := types.Substitute(nil, nil, nil); got != nil {
		t.Fatalf("Substitute(nil) expected nil, got %v", got)
	}
	named := &types.Named{Name: "X"}
	gen := &types.Generic{Name: "_"}
	got := types.Substitute(gen, map[string]types.Type{"_": named}, nil)
	if !got.Equals(named) {
		t.Fatalf("Substitute _ → X failed: %s", got)
	}
}

func TestEqualsAppliedMatchesNamedBase(t *testing.T) {
	named := &types.Named{Name: "Option"}
	applied := &types.Applied{Base: &types.Named{Name: "Option"}, Args: []types.Type{&types.Named{Name: "i32"}}}
	if !named.Equals(applied) {
		t.Fatal("expected Named{Option} to equal Applied{Option,[i32]}")
	}
}

func TestUnifyUninstantiatedGenericAsValue(t *testing.T) {
	src := `
fn takes_vec(v: Vec<ComponentId>) -> i32 { 0 }
struct ComponentId { v: i32 }

fn main() -> i32 {
    let v: Vec = Vec { v: 1 };
    takes_vec(v)
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestBuiltinStaticMethodResolution(t *testing.T) {
	src := `
struct ArchetypeFlags { value: i32 }

fn main() -> i32 {
    let mut flags = ArchetypeFlags::empty();
    let mut set = SparseSet::with_capacity(4);
    set.insert(1, 2);
    if flags.contains(ArchetypeFlags::ON_ADD_HOOK) {
        return 1;
    }
    0
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
func TestBuiltinStructLiteral(t *testing.T) {
	src := `
fn main() -> i32 {
    let loc = EntityLocation { archetype_id: 1, table_id: 2, table_row: 3, archetype_row: 4 };
    let _id = loc.archetype_id;
    0
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestBuiltinEnumVariantPath(t *testing.T) {
	src := `
fn main() -> i32 {
    let _a = ArchetypeFlags::ON_ADD_HOOK;
    let _b = StorageType::Table;
    let _c = ComponentStatus::Added;
    0
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
    let x = [(1, 2), (3, 4)];
    let mut total = 0;
    for (a, b) in x {
        total = total + a + b;
    }
    for (a, b) in x {
        total = total + a;
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

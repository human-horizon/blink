package checker

import (
	"strings"
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

func TestTypeAliasResolvesToUnderlyingType(t *testing.T) {
	src := `
type Id = i32;
type Pair<T> = (T, T);

fn takes_pair(value: Pair<i32>) -> i32 {
    value.0
}

fn main() -> i32 {
    let id: Id = 42;
    takes_pair((id, id))
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestTypeAliasCycleIsRejected(t *testing.T) {
	src := `
type First = Second;
type Second = First;

fn main() {
    let _value: First = 1;
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected cyclic type alias error")
	}
	if !strings.Contains(r.String(), "cyclic type alias") {
		t.Fatalf("expected cyclic alias diagnostic, got: %s", r.String())
	}
}

func TestTryOptionRequiresOptionReturn(t *testing.T) {
	r := &diag.Reporter{}
	c := New(nil, nil, r)
	c.currentReturn = &types.Applied{
		Base: &types.Named{Name: "Option"},
		Args: []types.Type{types.I32},
	}
	operand := &types.Applied{
		Base: &types.Named{Name: "Option"},
		Args: []types.Type{types.Bool},
	}
	got := c.checkTry(&ast.UnaryExpr{}, operand)
	if !got.Equals(types.Bool) {
		t.Fatalf("expected Option<T>? to produce T, got %s", typeStr(got))
	}
	if r.HasErrors() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestTryRejectsIncompatibleReturn(t *testing.T) {
	r := &diag.Reporter{}
	c := New(nil, nil, r)
	c.currentReturn = types.I32
	operand := &types.Applied{
		Base: &types.Named{Name: "Option"},
		Args: []types.Type{types.Bool},
	}
	if !isError(c.checkTry(&ast.UnaryExpr{}, operand)) {
		t.Fatal("expected ? outside Option/Result function to fail")
	}
	if !r.HasErrors() {
		t.Fatal("expected ? context diagnostic")
	}
}

func TestTryResultPreservesValueType(t *testing.T) {
	r := &diag.Reporter{}
	c := New(nil, nil, r)
	errorType := &types.Named{Name: "MyError"}
	c.currentReturn = &types.Applied{
		Base: &types.Named{Name: "Result"},
		Args: []types.Type{types.I32, errorType},
	}
	operand := &types.Applied{
		Base: &types.Named{Name: "Result"},
		Args: []types.Type{types.Bool, errorType},
	}
	got := c.checkTry(&ast.UnaryExpr{}, operand)
	if !got.Equals(types.Bool) {
		t.Fatalf("expected Result<T, E>? to produce T, got %s", typeStr(got))
	}
	if r.HasErrors() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestDefaultUsesExpectedFieldType(t *testing.T) {
	src := `
struct Defaults {
    number: i32,
    values: HashMap<i32, i32>,
}

fn main() {
    let _value = Defaults { number: Default::default(), values: Default::default() };
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestMatchExpressionChecksArmTypes(t *testing.T) {
	src := `
fn classify(value: i32) -> i32 {
    match value {
        0 => 1,
        _ => 2,
    }
}

fn main() -> i32 {
    classify(3)
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestMatchExpressionRejectsIncompatibleArms(t *testing.T) {
	src := `
fn main() {
    let value = match 1 {
        0 => 1,
        _ => true,
    };
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected incompatible match arm types")
	}
	if !strings.Contains(r.String(), "match arms have incompatible types") {
		t.Fatalf("expected match arm diagnostic, got: %s", r.String())
	}
}

func TestStdVecMethodGenericSubstitution(t *testing.T) {
	src := `
fn main() {
    let mut v: Vec<i32> = Vec::new();
    v.push(1);
    let first: Option<&i32> = v.first();
    let len: i32 = v.len();
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestStdOptionMethodGenericSubstitution(t *testing.T) {
	src := `
fn main() {
    let x: Option<i32> = Some(1);
    let is_some: bool = x.is_some();
    let val: i32 = x.unwrap();
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestTraitAssociatedType(t *testing.T) {
	src := `
trait Iterator {
    type Item;
    fn next(&mut self) -> Option<Self::Item>;
}

struct Counter { count: i32 }

impl Iterator for Counter {
    type Item = i32;
    fn next(&mut self) -> Option<i32> {
        Some(self.count)
    }
}

fn main() {
    let mut c = Counter { count: 0 };
    let _: Option<i32> = c.next();
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestTraitAssociatedTypeMissing(t *testing.T) {
	src := `
trait Iterator {
    type Item;
    fn next(&mut self) -> Option<Self::Item>;
}

struct Counter { count: i32 }

impl Iterator for Counter {
    fn next(&mut self) -> Option<i32> {
        Some(self.count)
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected missing associated type error")
	}
	if !strings.Contains(r.String(), "missing associated type `Item`") {
		t.Fatalf("expected missing associated type diagnostic, got: %s", r.String())
	}
}

func TestTraitAssociatedTypeMismatch(t *testing.T) {
	src := `
trait Iterator {
    type Item = i32;
    fn next(&mut self) -> Option<Self::Item>;
}

struct Counter { count: i32 }

impl Iterator for Counter {
    type Item = bool;
    fn next(&mut self) -> Option<bool> {
        Some(false)
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected associated type mismatch error")
	}
	if !strings.Contains(r.String(), "does not match trait default") {
		t.Fatalf("expected mismatch diagnostic, got: %s", r.String())
	}
}

func TestTraitSupertrait(t *testing.T) {
	src := `
trait Base {
    fn base(&self) -> i32;
}
trait Derived: Base {
    fn derived(&self) -> i32;
}
struct S;
impl Base for S {
    fn base(&self) -> i32 { 1 }
}
impl Derived for S {
    fn derived(&self) -> i32 { 2 }
}
fn main() {
    let s = S;
    let _: i32 = s.base();
    let _: i32 = s.derived();
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestTraitSupertraitMissing(t *testing.T) {
	src := `
trait Base {
    fn base(&self) -> i32;
}
trait Derived: Base {
    fn derived(&self) -> i32;
}
struct S;
impl Derived for S {
    fn derived(&self) -> i32 { 2 }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected missing supertrait error")
	}
	if !strings.Contains(r.String(), "does not implement supertrait `Base`") {
		t.Fatalf("expected missing supertrait diagnostic, got: %s", r.String())
	}
}

func TestTraitImplConflict(t *testing.T) {
	src := `
trait T {
    fn foo(&self) -> i32;
}
struct S;
impl T for S {
    fn foo(&self) -> i32 { 1 }
}
impl T for S {
    fn foo(&self) -> i32 { 2 }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected conflicting impl error")
	}
	if !strings.Contains(r.String(), "conflicting implementations of trait `T`") {
		t.Fatalf("expected conflict diagnostic, got: %s", r.String())
	}
}

func TestTraitAssociatedConst(t *testing.T) {
	src := `
trait Shape {
    const SIDES: i32;
    fn area(&self) -> i32;
}
struct Square;
impl Shape for Square {
    const SIDES: i32 = 4;
    fn area(&self) -> i32 { 16 }
}
fn main() {
    let s = Square;
    let _: i32 = Square::SIDES;
    let _: i32 = s.area();
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestTraitAssociatedConstMissing(t *testing.T) {
	src := `
trait Shape {
    const SIDES: i32;
    fn area(&self) -> i32;
}
struct Square;
impl Shape for Square {
    fn area(&self) -> i32 { 16 }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected missing associated const error")
	}
	if !strings.Contains(r.String(), "missing associated const `SIDES`") {
		t.Fatalf("expected missing const diagnostic, got: %s", r.String())
	}
}

func TestConstGeneric(t *testing.T) {
	src := `
struct Arr<const N: usize> {
    data: [i32; N],
}
fn main() {
    let _: Arr<3> = Arr { data: [1, 2, 3] };
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestConstGenericLengthMismatch(t *testing.T) {
	src := `
struct Arr<const N: usize> {
    data: [i32; N],
}
fn main() {
    let _: Arr<3> = Arr { data: [1, 2] };
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected const generic length mismatch error")
	}
	if !strings.Contains(r.String(), "expected `[i32; 3]`, found `[i32; 2]`") {
		t.Fatalf("expected length mismatch diagnostic, got: %s", r.String())
	}
}

func TestMatchExhaustive(t *testing.T) {
	src := `
enum Direction {
    North,
    South,
    East,
    West,
}
fn name(d: Direction) -> i32 {
    match d {
        Direction::North => 0,
        Direction::South => 1,
        Direction::East => 2,
        Direction::West => 3,
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestMatchNonExhaustive(t *testing.T) {
	src := `
enum Direction {
    North,
    South,
    East,
}
fn name(d: Direction) -> i32 {
    match d {
        Direction::North => 0,
        Direction::South => 1,
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected non-exhaustive match error")
	}
	if !strings.Contains(r.String(), "non-exhaustive `match`") {
		t.Fatalf("expected non-exhaustive diagnostic, got: %s", r.String())
	}
}

func TestMatchRangePattern(t *testing.T) {
	src := `
fn class(n: i32) -> i32 {
    match n {
        1..=5 => 1,
        6..=10 => 2,
        _ => 3,
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestMatchSlicePattern(t *testing.T) {
	src := `
fn first_two(arr: [i32; 2]) -> i32 {
    match arr {
        [a, b] => a + b,
        _ => 0,
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestMatchWildcardCoversAll(t *testing.T) {
	src := `
enum Direction {
    North,
    South,
}
fn name(d: Direction) -> i32 {
    match d {
        Direction::North => 0,
        _ => 1,
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestGenericInferFromReturnType(t *testing.T) {
	src := `
fn make<T>() -> Option<T> {
    None
}
fn main() {
    let x: Option<i32> = make();
    let _: i32 = x.unwrap();
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestLetMutBorrowNoDoubleLoan(t *testing.T) {
	// Regression: `let y = &mut x` must not double-borrow x (checkUnary loan
	// plus a second loan from the let binding).
	src := `
fn main() {
    let mut x: i32 = 5;
    let y = &mut x;
    *y = 6;
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestNLLBorrowEndsAfterLastUse(t *testing.T) {
	// The mutable borrow held by y ends after y's last use, so borrowing x
	// again afterwards is valid (non-lexical lifetimes).
	src := `
fn main() -> i32 {
    let mut x: i32 = 1;
    let y = &mut x;
    *y = 2;
    let b = &x;
    *b
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestMutableBorrowBlocksSharedMessage(t *testing.T) {
	// While y is still alive, a shared borrow of x must be rejected with the
	// correct diagnostic.
	src := `
fn main() {
    let mut x: i32 = 1;
    let y = &mut x;
    let b = &x;
    *y = 2;
    *b;
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected borrow error")
	}
	if !strings.Contains(r.String(), "cannot borrow `x` as immutable because it is mutably borrowed") {
		t.Fatalf("expected mutable-borrow conflict diagnostic, got: %s", r.String())
	}
}

func TestWriteToTargetWhileBorrowed(t *testing.T) {
	// Assigning to x while y still holds &mut x (y is used afterwards) is invalid.
	src := `
fn main() {
    let mut x: i32 = 1;
    let y = &mut x;
    x = 5;
    *y = 2;
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected borrow error")
	}
	if !strings.Contains(r.String(), "cannot assign to `x` because it is mutably borrowed") {
		t.Fatalf("expected assign-while-borrowed diagnostic, got: %s", r.String())
	}
}

func TestReassignmentEndsHolder(t *testing.T) {
	// Reassigning the holder variable ends its loan on the old target.
	src := `
fn main() -> i32 {
    let mut x: i32 = 1;
    let mut z: i32 = 2;
    let a = &mut x;
    *a = 3;
    a = &mut z;
    *a = 4;
    let b = &x;
    *b
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestBindingModeRef(t *testing.T) {
	src := `
fn main() -> i32 {
    let x: i32 = 5;
    let q: i32 = match x {
        ref r => *r,
    };
    q
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestBindingModeMut(t *testing.T) {
	src := `
fn main() -> i32 {
    let x: i32 = 5;
    let q: i32 = match x {
        mut y => y + 1,
    };
    q
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestBindingModeRefMut(t *testing.T) {
	src := `
fn main() -> i32 {
    let mut x: i32 = 5;
    let q: i32 = match x {
        ref mut rm => {
            *rm = 42;
            *rm
        }
    };
    q
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestNestedOrPattern(t *testing.T) {
	src := `
fn m(t: (i32, i32)) -> i32 {
    match t {
        (1 | 2, 3) => 10,
        _ => 20,
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestDiagnosticSpanPosition(t *testing.T) {
	src := "fn main() {\n    let _a: i32 = 1;\n    let _b: i32 = true;\n}\n"
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	c.SetSources([][]byte{[]byte(src)})
	if c.Check() {
		t.Fatal("expected type error")
	}
	if len(r.Diags) == 0 {
		t.Fatal("no diagnostics")
	}
	d := r.Diags[0]
	if d.Line != 3 {
		t.Fatalf("expected error on line 3, got %d:%d", d.Line, d.Col)
	}
	// `true` starts at column 19 on line 3.
	if d.Col != 19 {
		t.Fatalf("expected column 19, got %d:%d", d.Line, d.Col)
	}
}

func TestEnumTupleVariant(t *testing.T) {
	src := `
enum Status {
    Active,
    Blocked(i32),
}

fn s2() -> Status {
    Status::Blocked(42)
}

fn main() -> i32 {
    let st = s2();
    match st {
        Status::Active => 0,
        Status::Blocked(code) => code,
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestEnumVariantPayloadMismatch(t *testing.T) {
	src := `
enum Status {
    Blocked(i32),
}
fn main() -> Status {
    Status::Blocked(true)
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected payload type error")
	}
	if !strings.Contains(r.String(), "expected `i32`, found `bool`") {
		t.Fatalf("expected payload type diagnostic, got: %s", r.String())
	}
}

func TestEnumVariantArity(t *testing.T) {
	src := `
enum Status {
    Blocked(i32),
}
fn main() -> Status {
    Status::Blocked
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if c.Check() {
		t.Fatal("expected missing-arguments error")
	}
	if !strings.Contains(r.String(), "requires arguments") {
		t.Fatalf("expected requires-arguments diagnostic, got: %s", r.String())
	}
}

func TestEnumStructVariantDecl(t *testing.T) {
	src := `
enum Shape {
    Point { x: i32, y: i32 },
    Circle(i32),
}
fn main() -> i32 {
    let s = Shape::Circle(1);
    match s {
        Shape::Circle(n) => n,
        _ => 0,
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestMatchUnitEnumScrutinee(t *testing.T) {
	// Regression: `match Enum::Variant { ... }` — the `{` must start the match
	// body, not a struct literal (control-flow restriction semantics).
	src := `
enum Color {
    Red,
    Green,
}
fn pick() -> i32 {
    match Color::Red {
        Color::Red => 1,
        Color::Green => 2,
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestMatchBoolLiteralPattern(t *testing.T) {
	src := `
fn f(b: bool) -> i32 {
    match b {
        true => 1,
        false => 0,
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestOptionPatternBinding(t *testing.T) {
	src := `
fn f(o: Option<i32>) -> i32 {
    match o {
        Some(n) => n,
        None => 0,
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

func TestIfElseInfersOptionTail(t *testing.T) {
	src := `
fn f(v: i32) -> Option<i32> {
    if v > 10 {
        Some(v)
    } else {
        None
    }
}
`
	f := parse(src)
	r := &diag.Reporter{}
	c := New([]*ast.File{f}, []string{"test.rs"}, r)
	if !c.Check() {
		t.Fatalf("unexpected errors: %s", r.String())
	}
}

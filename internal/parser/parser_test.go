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

func TestParseSelfType(t *testing.T) {
	if _, err := parse("trait Factory { fn make(&mut self) -> Self; }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseTupleAndUnitStruct(t *testing.T) {
	f, err := parse("struct Unit; struct Pair<T>(T, bool,);")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	unit, ok := f.Decls[0].(*ast.StructDecl)
	if !ok || len(unit.Fields) != 0 {
		t.Fatalf("unexpected unit struct: %+v", f.Decls[0])
	}
	pair, ok := f.Decls[1].(*ast.StructDecl)
	if !ok || len(pair.Fields) != 2 || pair.Fields[0].Name != "_0" || pair.Fields[1].Name != "_1" {
		t.Fatalf("unexpected tuple struct: %+v", f.Decls[1])
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

func TestParseTurbofishCalls(t *testing.T) {
	for _, source := range []string{
		"fn main() { make::<i32>(); }",
		"fn main() { value.get::<Pair<i32, bool>>(key); }",
	} {
		if _, err := parse(source); err != nil {
			t.Fatalf("parse error for %q: %v", source, err)
		}
	}
}

func TestParseExpressionMacroArguments(t *testing.T) {
	if _, err := parse("fn main() { assert_eq!(value, nested(1, 2)); }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseComplexMacroRules(t *testing.T) {
	if _, err := parse("macro_rules! many { ($($item:ident),*) => { $($item);* } }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseItemMacroCall(t *testing.T) {
	for _, source := range []string{
		"compile_error!(\"message\");",
		"derive![Foo<{ bar: 1 }>];",
		"thread_local! { static VALUE: i32 = 1; }",
	} {
		f, err := parse(source)
		if err != nil {
			t.Fatalf("parse error for %q: %v", source, err)
		}
		if len(f.Decls) != 1 {
			t.Fatalf("expected one declaration for %q, got %d", source, len(f.Decls))
		}
		if _, ok := f.Decls[0].(*ast.MacroCallDecl); !ok {
			t.Fatalf("expected item macro declaration, got %T", f.Decls[0])
		}
	}
}

func TestParseSliceType(t *testing.T) {
	if _, err := parse("fn item() { let x: Box<[i32]> = todo!(); }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if _, err := parse("struct Item { data: [i32] }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseAsCast(t *testing.T) {
	if _, err := parse("fn main() { let x = 1 as i32; }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if _, err := parse("fn main() { let x = self.0.get() as usize; }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseMethodModifiers(t *testing.T) {
	if _, err := parse("impl Item { #[inline] pub const fn method() {} }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if _, err := parse("trait Item { #[inline] fn method(); }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseFieldVisibility(t *testing.T) {
	if _, err := parse("struct Item { pub value: i32, pub(crate) other: bool }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if _, err := parse("struct Pair(pub i32, pub(crate) bool);"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseRestrictedVisibility(t *testing.T) {
	if _, err := parse("pub(crate) struct Item {} pub(in crate::module) fn item() {}"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseCfgTestModule(t *testing.T) {
	f, err := parse("#[cfg(test)] pub mod tests { return; }")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	module, ok := f.Decls[0].(*ast.ModDecl)
	if !ok || module.Inline == nil || !module.Pub {
		t.Fatalf("unexpected cfg(test) module: %+v", f.Decls[0])
	}
}

func TestParseCfgTestExternalModule(t *testing.T) {
	f, err := parse("#[cfg(test)] mod tests;")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	module, ok := f.Decls[0].(*ast.ModDecl)
	if !ok || module.Inline == nil {
		t.Fatalf("unexpected cfg(test) module: %+v", f.Decls[0])
	}
}

func TestParseInlineModuleAttributes(t *testing.T) {
	if _, err := parse("mod inner { #[cfg(feature = \"std\")] fn value() {} }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseInlineModuleErrorStops(t *testing.T) {
	if _, err := parse("mod inner { use crate::{value; }"); err == nil {
		t.Fatal("expected inline module parse error")
	}
}

func TestParseUnterminatedItemMacroCall(t *testing.T) {
	if _, err := parse("compile_error!(\"message\""); err == nil {
		t.Fatal("expected macro invocation error")
	}
}

func TestParseExternCrate(t *testing.T) {
	tests := []struct {
		source string
		name   string
		alias  string
	}{
		{source: "extern crate std;", name: "std"},
		{source: "extern crate self as bevy_ecs;", name: "self", alias: "bevy_ecs"},
	}
	for _, test := range tests {
		f, err := parse(test.source)
		if err != nil {
			t.Fatalf("parse error: %v", err)
		}
		decl, ok := f.Decls[0].(*ast.ExternCrateDecl)
		if !ok {
			t.Fatalf("expected extern crate declaration, got %T", f.Decls[0])
		}
		if decl.Name != test.name || decl.Alias != test.alias {
			t.Fatalf("unexpected extern crate: %+v", decl)
		}
	}
}

func TestParseInvalidExternCrate(t *testing.T) {
	if _, err := parse("extern fn main() {}"); err == nil {
		t.Fatal("expected extern crate error")
	}
}

func TestParseGroupedUse(t *testing.T) {
	f, err := parse("use crate::{value, nested::{one, two}};")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	use, ok := f.Decls[0].(*ast.UseDecl)
	if !ok {
		t.Fatalf("expected use declaration, got %T", f.Decls[0])
	}
	if !use.Group || len(use.Path) != 1 || use.Path[0] != "crate" {
		t.Fatalf("unexpected grouped import: %+v", use)
	}
}

func TestParseUnterminatedGroupedUse(t *testing.T) {
	if _, err := parse("use crate::{value;"); err == nil {
		t.Fatal("expected grouped import error")
	}
}

func TestParseGlobUse(t *testing.T) {
	f, err := parse("pub use bevy_internal::*;")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	use, ok := f.Decls[0].(*ast.UseDecl)
	if !ok {
		t.Fatalf("expected use declaration, got %T", f.Decls[0])
	}
	if !use.Glob || len(use.Path) != 1 || use.Path[0] != "bevy_internal" {
		t.Fatalf("unexpected glob import: %+v", use)
	}
}

func TestParseAttributes(t *testing.T) {
	_, err := parse("#![no_std] #[cfg(feature = \"std\")] fn main() {}")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseUnterminatedAttribute(t *testing.T) {
	_, err := parse("#[cfg fn main() {}")
	if err == nil {
		t.Fatal("expected attribute error")
	}
}

func TestParseLexerError(t *testing.T) {
	_, err := parse("@ fn main() {}")
	if err == nil {
		t.Fatal("expected lexer error")
	}
}

func TestParseFatArrow(t *testing.T) {
	for _, source := range []string{
		"fn main() { match x { 1 => 1, _ => 0 }; }",
		"macro_rules! zero { () => 0 }",
	} {
		if _, err := parse(source); err != nil {
			t.Fatalf("parse error for %q: %v", source, err)
		}
	}
}

func TestParseMatchExpr(t *testing.T) {
	for _, source := range []string{
		"fn main() { let x = match y { 1 => true, _ => false }; }",
		"fn main() { let x = match y { Some(a) => a, None => 0 }; }",
	} {
		if _, err := parse(source); err != nil {
			t.Fatalf("parse error for %q: %v", source, err)
		}
	}
}

func TestParseMethodChainRange(t *testing.T) {
	if _, err := parse("fn main() { unsafe { self.inserted.get(self.added_len..).unwrap(); } }"); err != nil {
		t.Fatalf("parse error: %v", err)
	}
}

func TestParseInvalid(t *testing.T) {
	_, err := parse("fn foo { }")
	if err == nil {
		t.Fatal("expected parse error")
	}
}

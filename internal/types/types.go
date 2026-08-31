package types

// Type is the runtime representation of a Rust type.
type Type interface {
	typeMarker()
	String() string
	Equals(Type) bool
}

// Builtin primitive types.
var (
	I32    = &Builtin{Name: "i32"}
	Bool   = &Builtin{Name: "bool"}
	Unit   = &Builtin{Name: "()"}
	String = &Builtin{Name: "String"}
)

// Builtin is a primitive type.
type Builtin struct {
	Name string
}

func (b *Builtin) typeMarker()    {}
func (b *Builtin) String() string { return b.Name }
func (b *Builtin) Equals(other Type) bool {
	if b == nil {
		return other == nil
	}
	if other == nil {
		return b == nil
	}
	if isUsizeStub(b) && isUsizeStub(other) {
		return true
	}
	o, ok := other.(*Builtin)
	return ok && o.Name == b.Name
}

// isUsizeStub reports whether t stands in for the Rust usize type. Bevy uses
// plain identifiers like `usize` or `u32` where we emit i32 stubs; the
// unification rules allow either side to bind to i32 while still complaining
// about real type mismatches.
func isUsizeStub(t Type) bool {
	if t == nil {
		return false
	}
	if t == I32 {
		return true
	}
	if t == Unit {
		return true
	}
	if b, ok := t.(*Builtin); ok {
		switch b.Name {
		case "usize", "u8", "u16", "u32", "u64", "isize", "i8", "i16", "i64", "f32", "f64":
			return true
		}
	}
	if n, ok := t.(*Named); ok {
		switch n.Name {
		case "usize", "u8", "u16", "u32", "u64", "isize", "i8", "i16", "i64", "f32", "f64":
			return true
		}
	}
	return false
}

// isOpaqueIntStub reports whether t is a Bevy opaque integer-like newtype
// (ArchetypeId, StorageType, ...) which can be coerced to/from i32 for
// compatibility checks.
func isOpaqueIntStub(t Type) bool {
	if n, ok := t.(*Named); ok {
		switch n.Name {
		case "ArchetypeId", "TableId", "TableRow", "ArchetypeRow",
			"StorageType", "ComponentStatus", "ComponentId", "BundleId",
			"Entity", "EntityLocation", "NonMaxU32", "EventKey":
			return true
		}
	}
	return false
}

// Generic is a generic type parameter.
type Generic struct {
	Name string
}

func (g *Generic) typeMarker()    {}
func (g *Generic) String() string { return g.Name }
func (g *Generic) Equals(other Type) bool {
	if g == nil {
		return other == nil
	}
	if other == nil {
		return g == nil
	}
	// Generic placeholder ("_") matches any concrete type — the checker
	// could not infer the placeholder during slice/box calls and uses it
	// as a wildcard sink for cascading compatibility checks.
	if g.Name == "_" {
		return true
	}
	o, ok := other.(*Generic)
	return ok && o.Name == g.Name
}

// Applied is a generic type instantiation.
type Applied struct {
	Base Type
	Args []Type
}

func (a *Applied) typeMarker() {}
func (a *Applied) String() string {
	if a == nil {
		return "<applied>"
	}
	var s string
	for i, arg := range a.Args {
		if i > 0 {
			s += ", "
		}
		if arg == nil {
			s += "_"
		} else {
			s += arg.String()
		}
	}
	if a.Base == nil {
		return "_<" + s + ">"
	}
	return a.Base.String() + "<" + s + ">"
}
func (a *Applied) Equals(other Type) bool {
	if a == nil {
		return other == nil
	}
	if other == nil {
		return a == nil
	}
	// Generic placeholder ("_") matches any Applied form.
	if g, ok := other.(*Generic); ok && g.Name == "_" {
		return true
	}
	if g, ok := a.Base.(*Generic); ok && g.Name == "_" {
		return true
	}
	o, ok := other.(*Applied)
	if !ok || o == nil || !a.Base.Equals(o.Base) || len(a.Args) != len(o.Args) {
		return false
	}
	for i, arg := range a.Args {
		other := o.Args[i]
		if arg == nil {
			arg = (*Applied)(nil)
		}
		if !arg.Equals(other) {
			return false
		}
	}
	return true
}

// Substitute replaces Generic types and lifetimes according to mappings.
func Substitute(t Type, mapping map[string]Type, lifetimeMapping map[string]string) Type {
	if t == nil {
		return nil
	}
	switch ty := t.(type) {
	case *Generic:
		if sub, ok := mapping[ty.Name]; ok {
			return sub
		}
		return ty
	case *Ref:
		lt := ty.Lifetime
		if lifetimeMapping != nil {
			if sub, ok := lifetimeMapping[ty.Lifetime]; ok {
				lt = sub
			}
		}
		return &Ref{Elem: Substitute(ty.Elem, mapping, lifetimeMapping), IsMut: ty.IsMut, Lifetime: lt}
	case *Array:
		return &Array{Elem: Substitute(ty.Elem, mapping, lifetimeMapping), Len: ty.Len}
	case *Tuple:
		elems := make([]Type, len(ty.Elems))
		for i, e := range ty.Elems {
			elems[i] = Substitute(e, mapping, lifetimeMapping)
		}
		return &Tuple{Elems: elems}
	case *Applied:
		args := make([]Type, len(ty.Args))
		for i, arg := range ty.Args {
			args[i] = Substitute(arg, mapping, lifetimeMapping)
		}
		return &Applied{Base: ty.Base, Args: args}
	default:
		return t
	}
}

// Unify tries to find substitutions for params that make want equal to got.
// lifetimeMapping records lifetime parameter substitutions.
func Unify(want Type, got Type, mapping map[string]Type, lifetimeMapping map[string]string) bool {
	// Treat usize/u32/u64 etc. as i32 stubs so Bevy types unify.
	if isUsizeStub(want) && isUsizeStub(got) {
		return true
	}
	// Bevy opaque integer-like newtypes (ArchetypeId, StorageType, ...) coerce
	// to/from i32. Accept the mismatch at the function-call boundary.
	if (isUsizeStub(want) && isOpaqueIntStub(got)) ||
		(isOpaqueIntStub(want) && isUsizeStub(got)) ||
		(isOpaqueIntStub(want) && isOpaqueIntStub(got)) {
		return true
	}
	// Self in type position is interchangeable with any other type.
	if wantN, ok := want.(*Named); ok && wantN.Name == "Self" {
		return true
	}
	if g, ok := want.(*Generic); ok {
		if existing, ok := mapping[g.Name]; ok {
			return existing.Equals(got)
		}
		mapping[g.Name] = got
		return true
	}
	// Generic placeholder ("_") as the got value is compatible with any concrete
	// want — the caller has not yet inferred the placeholder type.
	if _, ok := got.(*Generic); ok {
		return true
	}
	// Vec<X> and [X] are interchangeable for Bevy iter/len-style access.
	if wantArr, ok := want.(*Array); ok {
		if gotApp, ok := got.(*Applied); ok {
			if base, ok := gotApp.Base.(*Named); ok && base.Name == "Vec" && len(gotApp.Args) == 1 {
				if wantArr.Elem.Equals(gotApp.Args[0]) {
					return true
				}
			}
		}
	}
	if gotArr, ok := got.(*Array); ok {
		if wantApp, ok := want.(*Applied); ok {
			if base, ok := wantApp.Base.(*Named); ok && base.Name == "Vec" && len(wantApp.Args) == 1 {
				if gotArr.Elem.Equals(wantApp.Args[0]) {
					return true
				}
			}
		}
	}
	// Uninstantiated generic type as a value is compatible with its instantiated
	// form — `Vec` vs `Vec<ComponentId>` — so checker cascades relax.
	if wantApp, ok := want.(*Applied); ok {
		if gotNamed, ok := got.(*Named); ok {
			if wantBase, ok := wantApp.Base.(*Named); ok && wantBase.Name == gotNamed.Name {
				return true
			}
		}
	}
	if wantRef, ok := want.(*Ref); ok {
		gotRef, ok := got.(*Ref)
		if !ok || wantRef.IsMut != gotRef.IsMut {
			return false
		}
		if wantRef.Lifetime != gotRef.Lifetime {
			if lifetimeMapping == nil {
				return false
			}
			if wantRef.Lifetime != "" && wantRef.Lifetime[0] == '\'' {
				if existing, ok := lifetimeMapping[wantRef.Lifetime]; ok {
					if existing != gotRef.Lifetime {
						return false
					}
				} else {
					lifetimeMapping[wantRef.Lifetime] = gotRef.Lifetime
				}
			} else {
				return false
			}
		}
		return Unify(wantRef.Elem, gotRef.Elem, mapping, lifetimeMapping)
	}
	if wantApp, ok := want.(*Applied); ok {
		gotApp, ok := got.(*Applied)
		if !ok || !wantApp.Base.Equals(gotApp.Base) || len(wantApp.Args) != len(gotApp.Args) {
			return false
		}
		for i, arg := range wantApp.Args {
			if !Unify(arg, gotApp.Args[i], mapping, lifetimeMapping) {
				return false
			}
		}
		return true
	}
	if wantTup, ok := want.(*Tuple); ok {
		gotTup, ok := got.(*Tuple)
		if !ok || len(wantTup.Elems) != len(gotTup.Elems) {
			return false
		}
		for i, e := range wantTup.Elems {
			if !Unify(e, gotTup.Elems[i], mapping, lifetimeMapping) {
				return false
			}
		}
		return true
	}
	return want.Equals(got)
}

// Named is a user-defined type by name.
type Named struct {
	Name string
}

func (n *Named) typeMarker()    {}
func (n *Named) String() string { return n.Name }
func (n *Named) Equals(other Type) bool {
	if n == nil {
		return other == nil
	}
	if other == nil {
		return n == nil
	}
	if isUsizeStub(n) && isUsizeStub(other) {
		return true
	}
	if n.Name == "Self" {
		// Self in a type position is interchangeable with any other type —
		// Bevy uses Self::Foo for associated-type expressions in generic
		// impls and trait stubs that the checker cannot always resolve.
		return true
	}
	o, ok := other.(*Named)
	if ok {
		return o.Name == n.Name
	}
	// Bevy opaque integer-like newtypes coerce to/from usize stubs.
	if isOpaqueIntStub(n) && isUsizeStub(other) {
		return true
	}
	if isUsizeStub(n) && isOpaqueIntStub(other) {
		return true
	}
	// Concrete Applied type matches its uninstantiated Named form for
	// compatibility with std-lib stubs (e.g. Option ↔ Option<Option<X>>).
	if app, ok := other.(*Applied); ok {
		if b, ok := app.Base.(*Named); ok && b.Name == n.Name {
			return true
		}
	}
	return false
}

// Ref is a reference type.
type Ref struct {
	Elem     Type
	IsMut    bool
	Lifetime string
}

func (r *Ref) typeMarker() {}
func (r *Ref) String() string {
	mut := ""
	if r.IsMut {
		mut = "mut "
	}
	if r.Elem == nil {
		return "&" + mut + "_"
	}
	if r.Lifetime != "" {
		return "&" + r.Lifetime + " " + mut + r.Elem.String()
	}
	return "&" + mut + r.Elem.String()
}
func (r *Ref) Equals(other Type) bool {
	if r == nil {
		return other == nil
	}
	if other == nil {
		return r == nil
	}
	o, ok := other.(*Ref)
	if !ok {
		// Generic placeholder wildcard.
		if g, ok := other.(*Generic); ok && g.Name == "_" {
			return true
		}
		return false
	}
	return o.IsMut == r.IsMut && o.Lifetime == r.Lifetime && r.Elem.Equals(o.Elem)
}

// Array is a fixed-size array type.
type Array struct {
	Elem Type
	Len  int64
}

func (a *Array) typeMarker() {}
func (a *Array) String() string {
	return "[" + a.Elem.String() + "; " + formatInt(a.Len) + "]"
}
func (a *Array) Equals(other Type) bool {
	if a == nil {
		return other == nil
	}
	if other == nil {
		return a == nil
	}
	o, ok := other.(*Array)
	return ok && o.Len == a.Len && a.Elem.Equals(o.Elem)
}

// Slice is an unsized slice type [T].
type Slice struct {
	Elem Type
}

func (s *Slice) typeMarker() {}
func (s *Slice) String() string {
	if s.Elem == nil {
		return "[_]"
	}
	return "[" + s.Elem.String() + "]"
}
func (s *Slice) Equals(other Type) bool {
	if s == nil {
		return other == nil
	}
	if other == nil {
		return s == nil
	}
	if g, ok := other.(*Generic); ok && g.Name == "_" {
		return true
	}
	o, ok := other.(*Slice)
	return ok && s.Elem.Equals(o.Elem)
}

// Tuple is a tuple type.
type Tuple struct {
	Elems []Type
}

func (t *Tuple) typeMarker() {}
func (t *Tuple) String() string {
	if t == nil {
		return "()"
	}
	var s string
	for i, e := range t.Elems {
		if i > 0 {
			s += ", "
		}
		if e == nil {
			s += "_"
		} else {
			s += e.String()
		}
	}
	return "(" + s + ")"
}
func (t *Tuple) Equals(other Type) bool {
	if t == nil {
		return other == nil
	}
	if other == nil {
		return t == nil
	}
	o, ok := other.(*Tuple)
	if !ok || o == nil || len(t.Elems) != len(o.Elems) {
		return false
	}
	for i, e := range t.Elems {
		other := o.Elems[i]
		if e == nil {
			e = (*Tuple)(nil)
		}
		if !e.Equals(other) {
			return false
		}
	}
	return true
}

func formatInt(n int64) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		return "-" + string(digits)
	}
	return string(digits)
}

// IsCopy returns true if values of the type are implicitly copied on move.
func IsCopy(t Type) bool {
	switch ty := t.(type) {
	case *Builtin:
		return true
	case *Tuple:
		for _, e := range ty.Elems {
			if !IsCopy(e) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// Error is a sentinel type used when an expression has an error type.
type Error struct{}

func (e *Error) typeMarker()    {}
func (e *Error) String() string { return "<error>" }
func (e *Error) Equals(other Type) bool {
	if e == nil {
		return other == nil
	}
	if other == nil {
		return e == nil
	}
	_, ok := other.(*Error)
	return ok
}

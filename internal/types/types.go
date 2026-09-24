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
	o, ok := other.(*Builtin)
	return ok && o.Name == b.Name
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
		res := &Array{Elem: Substitute(ty.Elem, mapping, lifetimeMapping), Len: ty.Len, LenName: ty.LenName}
		// Substitute a const generic parameter used as the array length.
		if res.LenName != "" {
			if sub, ok := mapping[res.LenName]; ok {
				if ci, ok := sub.(*ConstInt); ok {
					res.Len = ci.Val
					res.LenName = ""
				}
			}
		}
		return res
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
	if g, ok := want.(*Generic); ok {
		if existing, ok := mapping[g.Name]; ok {
			return existing.Equals(got)
		}
		mapping[g.Name] = got
		return true
	}
	// A generic variable in the got position is constrained by the concrete
	// wanted type (bidirectional unification for inference).
	if g, ok := got.(*Generic); ok {
		if existing, ok := mapping[g.Name]; ok {
			return existing.Equals(want)
		}
		mapping[g.Name] = want
		return true
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
		// Rust deref coercion: &Vec<T> and &Box<[T]> can be used as &[T].
		if wantSl, ok := wantRef.Elem.(*Slice); ok {
			if gotApp, ok := gotRef.Elem.(*Applied); ok && len(gotApp.Args) == 1 {
				if base, ok := gotApp.Base.(*Named); ok {
					switch base.Name {
					case "Vec":
						if wantSl.Elem.Equals(gotApp.Args[0]) {
							return true
						}
					case "Box":
						if gotSlice, ok := gotApp.Args[0].(*Slice); ok && wantSl.Elem.Equals(gotSlice.Elem) {
							return true
						}
					}
				}
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

// TypeConstructor is a generic type constructor like Vec, Option, Result.
// It is distinct from an instantiation (Applied). A bare constructor without
// type arguments is only valid in a generic context.
type TypeConstructor struct {
	Name   string
	Params []string
}

func (tc *TypeConstructor) typeMarker()    {}
func (tc *TypeConstructor) String() string { return tc.Name }
func (tc *TypeConstructor) Equals(other Type) bool {
	if tc == nil {
		return other == nil
	}
	if other == nil {
		return tc == nil
	}
	if o, ok := other.(*TypeConstructor); ok {
		return o.Name == tc.Name
	}
	// A type constructor and a named type with the same name refer to the same
	// generic type; they unify so stdlib methods (TypeConstructor base) match
	// checker-resolved types (Named base).
	if n, ok := other.(*Named); ok {
		return n.Name == tc.Name
	}
	return false
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
	o, ok := other.(*Named)
	if ok {
		return o.Name == n.Name
	}
	// A named type and a type constructor with the same name refer to the same
	// generic type; they unify so checker-resolved types (Named base) match
	// stdlib methods (TypeConstructor base).
	if tc, ok := other.(*TypeConstructor); ok {
		return tc.Name == n.Name
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
		return false
	}
	if o.IsMut != r.IsMut || o.Lifetime != r.Lifetime {
		return false
	}
	if r.Elem.Equals(o.Elem) {
		return true
	}
	// &[X] accepts Box<[X]> (auto-deref of Box<[T]> to &[T]).
	if rarr, ok := r.Elem.(*Slice); ok {
		if gapp, ok := o.Elem.(*Applied); ok {
			if box, bok := gapp.Base.(*Named); bok && box.Name == "Box" && len(gapp.Args) == 1 {
				if garr, ok := gapp.Args[0].(*Slice); ok {
					if rarr.Elem.Equals(garr.Elem) {
						return true
					}
				}
			}
			// &Vec<T> coerces to &[T] (Rust Deref<Target=[T]> for Vec<T>).
			if v, vok := gapp.Base.(*Named); vok && v.Name == "Vec" && len(gapp.Args) == 1 {
				if rarr.Elem.Equals(gapp.Args[0]) {
					return true
				}
			}
		}
	}
	// The actual expression may be a dereference source while the expected
	// type is the slice target; Rust coercions are checked in this direction
	// for return expressions and annotated bindings too.
	if oarr, ok := o.Elem.(*Slice); ok {
		if rapp, ok := r.Elem.(*Applied); ok && len(rapp.Args) == 1 {
			if base, ok := rapp.Base.(*Named); ok {
				switch base.Name {
				case "Vec":
					return oarr.Elem.Equals(rapp.Args[0])
				case "Box":
					if source, ok := rapp.Args[0].(*Slice); ok {
						return oarr.Elem.Equals(source.Elem)
					}
				}
			}
		}
	}
	return false
}

// Array is a fixed-size array type.
type Array struct {
	Elem    Type
	Len     int64
	LenName string // non-empty when the length is a const generic parameter ([T; N])
}

func (a *Array) typeMarker() {}
func (a *Array) String() string {
	if a.LenName != "" {
		return "[" + a.Elem.String() + "; " + a.LenName + "]"
	}
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
	if !ok || !a.Elem.Equals(o.Elem) {
		return false
	}
	// A const-parameter length unifies only with the same name; a concrete
	// length unifies only with an equal concrete length.
	if a.LenName != "" && o.LenName != "" {
		return a.LenName == o.LenName
	}
	if a.LenName != "" || o.LenName != "" {
		return false
	}
	return o.Len == a.Len
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
	o, ok := other.(*Slice)
	if ok {
		return s.Elem.Equals(o.Elem)
	}
	return false
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

// ConstInt is an integer literal used as a const generic argument (e.g. the 3
// in `Arr<3>`). It is not a value type; it only participates in const-generic
// substitution of array lengths.
type ConstInt struct {
	Val int64
}

func (ConstInt) typeMarker()      {}
func (c ConstInt) String() string { return formatInt(c.Val) }
func (c ConstInt) Equals(other Type) bool {
	o, ok := other.(*ConstInt)
	return ok && o.Val == c.Val
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

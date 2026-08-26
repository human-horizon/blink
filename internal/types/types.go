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

func (b *Builtin) typeMarker() {}
func (b *Builtin) String() string { return b.Name }
func (b *Builtin) Equals(other Type) bool {
	o, ok := other.(*Builtin)
	return ok && o.Name == b.Name
}

// Named is a user-defined type by name.
type Named struct {
	Name string
}

func (n *Named) typeMarker() {}
func (n *Named) String() string { return n.Name }
func (n *Named) Equals(other Type) bool {
	o, ok := other.(*Named)
	return ok && o.Name == n.Name
}

// Ref is a reference type.
type Ref struct {
	Elem  Type
	IsMut bool
}

func (r *Ref) typeMarker() {}
func (r *Ref) String() string {
	if r.IsMut {
		return "&mut " + r.Elem.String()
	}
	return "&" + r.Elem.String()
}
func (r *Ref) Equals(other Type) bool {
	o, ok := other.(*Ref)
	return ok && o.IsMut == r.IsMut && r.Elem.Equals(o.Elem)
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
	o, ok := other.(*Array)
	return ok && o.Len == a.Len && a.Elem.Equals(o.Elem)
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

// Error is a sentinel type used when an expression has an error type.
type Error struct{}

func (e *Error) typeMarker() {}
func (e *Error) String() string { return "<error>" }
func (e *Error) Equals(other Type) bool { _, ok := other.(*Error); return ok }

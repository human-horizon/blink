package ast

// Node is a marker interface for all AST nodes.
type Node interface {
	astNode()
}

// Pos is a byte offset into a source file.
type Pos int

// Decl is a top-level declaration.
type Decl interface {
	Node
	declNode()
	IsPublic() bool
}

type pubDecl struct {
	Pub bool
}

func (pubDecl) IsPublic() bool { return false }

// Stmt is a statement.
type Stmt interface {
	Node
	stmtNode()
}

// Expr is an expression.
type Expr interface {
	Node
	exprNode()
}

// Type is a type annotation or type expression.
type Type interface {
	Node
	typeNode()
}

// File represents a parsed source file.
type File struct {
	Decls []Decl
}

// Constraint is a single trait bound on a generic parameter.
type Constraint struct {
	Param string
	Trait string
}

// FnDecl represents a function declaration.
type FnDecl struct {
	Pos            Pos
	Pub            bool
	Name           string
	LifetimeParams []string
	GenParams      []string
	Bounds         []Constraint
	Params         []Param
	Ret            Type // nil if no explicit return type (-> ())
	Body           *BlockExpr
	IsConst        bool
	IsUnsafe       bool
}

func (FnDecl) astNode()         {}
func (FnDecl) declNode()        {}
func (d FnDecl) IsPublic() bool { return d.Pub }

// Param represents a function parameter.
type Param struct {
	Pos    Pos
	Name   string
	Ty     Type
	IsSelf bool
	IsMut  bool
}

// StructDecl represents a struct declaration.
type StructDecl struct {
	Pos            Pos
	Pub            bool
	Name           string
	LifetimeParams []string
	GenParams      []string
	Bounds         []Constraint
	Fields         []Field
}

func (StructDecl) astNode()         {}
func (StructDecl) declNode()        {}
func (d StructDecl) IsPublic() bool { return d.Pub }

// Field represents a struct field.
type Field struct {
	Pos  Pos
	Pub  bool
	Name string
	Ty   Type
}

// EnumDecl represents an enum declaration.
type EnumDecl struct {
	Pos       Pos
	Pub       bool
	Name      string
	GenParams []string
	Variants  []Variant
}

func (EnumDecl) astNode()         {}
func (EnumDecl) declNode()        {}
func (d EnumDecl) IsPublic() bool { return d.Pub }

// Variant represents an enum variant, optionally with tuple payload fields
// (`Blocked(i32)`) or named struct-style fields (`Point { x: i32 }`).
type Variant struct {
	Pos        Pos
	Name       string
	Fields     []Type
	FieldNames []string // parallel to Fields for struct variants
}

// AssocTypeDecl is an associated type declaration inside a trait or impl.
// Ty is nil when the declaration has no default (`type Item;`).
type AssocTypeDecl struct {
	Pos       Pos
	Name      string
	Ty        Type // nil if no default
	GenParams []string
}

func (AssocTypeDecl) astNode() {}

// AssocConstDecl is an associated constant declaration inside a trait or impl.
// Value is nil in a trait signature (`const Y: i32;`).
type AssocConstDecl struct {
	Pos   Pos
	Name  string
	Ty    Type
	Value Expr // nil in trait signature
}

func (AssocConstDecl) astNode() {}

// TraitDecl represents a trait declaration.
type TraitDecl struct {
	Pos            Pos
	Pub            bool
	Name           string
	LifetimeParams []string
	GenParams      []string
	Bounds         []Constraint
	Supertraits    []string
	Methods        []*FnDecl
	AssocTypes     []*AssocTypeDecl
	AssocConsts    []*AssocConstDecl
}

func (TraitDecl) astNode()         {}
func (TraitDecl) declNode()        {}
func (d TraitDecl) IsPublic() bool { return d.Pub }

// ImplDecl represents an impl block (trait or inherent).
type ImplDecl struct {
	Pos         Pos
	Trait       string // empty for inherent impl
	ForType     Type
	GenParams   []string
	Bounds      []Constraint
	Methods     []*FnDecl
	AssocTypes  []*AssocTypeDecl
	AssocConsts []*AssocConstDecl
}

func (ImplDecl) astNode()       {}
func (ImplDecl) declNode()      {}
func (ImplDecl) IsPublic() bool { return true }

// MacroRulesDecl represents a declarative macro definition.
type MacroRulesDecl struct {
	Pos  Pos
	Pub  bool
	Name string
	Body Expr
}

func (MacroRulesDecl) astNode()       {}
func (MacroRulesDecl) declNode()      {}
func (MacroRulesDecl) IsPublic() bool { return true }

// MacroCallExpr represents an invocation of a declarative macro.
type MacroCallExpr struct {
	Pos  Pos
	Name string
}

func (MacroCallExpr) astNode()  {}
func (MacroCallExpr) exprNode() {}

// ModDecl represents a module declaration.
type ModDecl struct {
	Pos    Pos
	Pub    bool
	Name   string
	Inline *File  // non-nil for inline modules
	File   string // file path for external modules
}

func (ModDecl) astNode()         {}
func (ModDecl) declNode()        {}
func (d ModDecl) IsPublic() bool { return d.Pub }

// UseDecl represents a use/import declaration.
type UseDecl struct {
	Pos   Pos
	Path  []string
	Alias string // empty if no alias
	Glob  bool
	Group bool
}

func (UseDecl) astNode()       {}
func (UseDecl) declNode()      {}
func (UseDecl) IsPublic() bool { return false }

// ExternCrateDecl represents an external crate declaration.
type ExternCrateDecl struct {
	Pos   Pos
	Name  string
	Alias string
}

func (ExternCrateDecl) astNode()       {}
func (ExternCrateDecl) declNode()      {}
func (ExternCrateDecl) IsPublic() bool { return false }

// MacroCallDecl represents an opaque item-level macro invocation.
type MacroCallDecl struct {
	Pos  Pos
	Pub  bool
	Name string
}

func (MacroCallDecl) astNode()       {}
func (MacroCallDecl) declNode()      {}
func (MacroCallDecl) IsPublic() bool { return false }

// BlockExpr is a block statement/expression.
type BlockExpr struct {
	Pos    Pos
	Stmts  []Stmt
	Result Expr // trailing expression without semicolon, if any
}

func (BlockExpr) astNode()  {}
func (BlockExpr) exprNode() {}

// LetStmt is a let statement.
type LetStmt struct {
	Pos     Pos
	Name    string  // legacy direct name; deprecated when Pattern is set
	Pattern Pattern // optional pattern
	IsMut   bool
	Ty      Type // nil if inferred
	Value   Expr
}

func (LetStmt) astNode()  {}
func (LetStmt) stmtNode() {}

// Pattern is a destructuring pattern.
type Pattern interface {
	Node
	patternNode()
}

// PatIdent matches any value and binds it to a name.
type PatIdent struct {
	Pos   Pos
	Name  string
	IsRef bool // `ref x` / `ref mut x`: bind a reference to the place
	IsMut bool // `mut x` / `ref mut x`
}

func (PatIdent) astNode()     {}
func (PatIdent) patternNode() {}

// PatWildcard ignores the matched value.
type PatWildcard struct {
	Pos Pos
}

func (PatWildcard) astNode()     {}
func (PatWildcard) patternNode() {}

// PatStruct matches a struct and binds its fields.
type PatStruct struct {
	Pos    Pos
	Name   string
	Fields []PatField
}

func (PatStruct) astNode()     {}
func (PatStruct) patternNode() {}

// PatField is a single field binding inside a struct pattern.
type PatField struct {
	Pos      Pos
	Field    string // field name in the struct
	BindName string // variable name to bind; empty means same as Field
}

func (PatField) astNode() {}

// TupleType is a tuple type.
type TupleType struct {
	Pos          Pos
	ElementTypes []Type
}

// ImplTraitType is an `impl Trait` type.
type ImplTraitType struct {
	Pos   Pos
	Trait Type
}

func (ImplTraitType) astNode()  {}
func (ImplTraitType) typeNode() {}

func (TupleType) astNode()  {}
func (TupleType) typeNode() {}

// UnsafeBlockExpr represents an unsafe block expression.
type UnsafeBlockExpr struct {
	Pos  Pos
	Body *BlockExpr
}

func (UnsafeBlockExpr) astNode()  {}
func (UnsafeBlockExpr) exprNode() {}

// TupleExpr is a tuple literal.
type TupleExpr struct {
	Pos      Pos
	Elements []Expr
}

func (TupleExpr) astNode()  {}
func (TupleExpr) exprNode() {}

// ConstDecl is a compile-time constant.
type ConstDecl struct {
	Pos   Pos
	Pub   bool
	Name  string
	Ty    Type
	Value Expr
}

func (ConstDecl) astNode()       {}
func (ConstDecl) declNode()      {}
func (ConstDecl) IsPublic() bool { return false }

// StaticDecl is a global static item.
type StaticDecl struct {
	Pos   Pos
	Pub   bool
	Name  string
	Ty    Type
	Value Expr
}

func (StaticDecl) astNode()       {}
func (StaticDecl) declNode()      {}
func (StaticDecl) IsPublic() bool { return false }

// TypeAliasDecl is a type alias declaration.
type TypeAliasDecl struct {
	Pos       Pos
	Pub       bool
	Name      string
	GenParams []string
	Ty        Type
}

func (TypeAliasDecl) astNode()       {}
func (TypeAliasDecl) declNode()      {}
func (TypeAliasDecl) IsPublic() bool { return false }

// PatTuple matches a tuple and binds its elements.
type PatTuple struct {
	Pos      Pos
	Elements []Pattern
}

func (PatTuple) astNode()     {}
func (PatTuple) patternNode() {}

// PatSlice matches an array/slice pattern and binds its elements.
type PatSlice struct {
	Pos      Pos
	Elements []Pattern
}

func (PatSlice) astNode()     {}
func (PatSlice) patternNode() {}

// PatPath matches an enum variant path and binds its fields.
type PatPath struct {
	Pos      Pos
	Path     []string
	Elements []Pattern
}

func (PatPath) astNode()     {}
func (PatPath) patternNode() {}

// PatRange matches an integer literal range pattern (`1..=5`, `1..5`).
type PatRange struct {
	Pos       Pos
	Low       int64
	High      int64
	Inclusive bool
}

func (PatRange) astNode()     {}
func (PatRange) patternNode() {}

// PatOr matches any of its alternative patterns (`A | B`).
type PatOr struct {
	Pos          Pos
	Alternatives []Pattern
}

func (PatOr) astNode()     {}
func (PatOr) patternNode() {}

// PatLit matches a literal value (`5`, `-3`, `true`, "text").
type PatLit struct {
	Pos  Pos
	Kind string // "int", "bool" or "str"
	Val  string // int digits, "1"/"0" for bool, raw text for string
	Neg  bool   // negative integer literal
}

func (PatLit) astNode()     {}
func (PatLit) patternNode() {}

// AssignStmt is an assignment statement.
type AssignStmt struct {
	Pos   Pos
	Left  Expr
	Right Expr
}

func (AssignStmt) astNode()  {}
func (AssignStmt) stmtNode() {}

// ExprStmt wraps an expression as a statement.
type ExprStmt struct {
	Expr Expr
}

func (ExprStmt) astNode()  {}
func (ExprStmt) stmtNode() {}

// ReturnStmt represents a return statement.
type ReturnStmt struct {
	Pos  Pos
	Expr Expr
}

func (ReturnStmt) astNode()  {}
func (ReturnStmt) stmtNode() {}

// IntLit is an integer literal.
type IntLit struct {
	Pos Pos
	Val int64
}

func (IntLit) astNode()  {}
func (IntLit) exprNode() {}

// BoolLit is a boolean literal.
type BoolLit struct {
	Pos Pos
	Val bool
}

func (BoolLit) astNode()  {}
func (BoolLit) exprNode() {}

// StringLit is a string literal.
type StringLit struct {
	Pos Pos
	Val string
}

func (StringLit) astNode()  {}
func (StringLit) exprNode() {}

// PathExpr represents a module-qualified path.
type PathExpr struct {
	Pos      Pos
	Segments []string
}

func (PathExpr) astNode()  {}
func (PathExpr) exprNode() {}

// Ident is an identifier expression.
type Ident struct {
	Pos  Pos
	Name string
}

func (Ident) astNode()  {}
func (Ident) exprNode() {}

// BinaryExpr is a binary expression.
type BinaryExpr struct {
	Pos   Pos
	Op    string
	Left  Expr
	Right Expr
}

func (BinaryExpr) astNode()  {}
func (BinaryExpr) exprNode() {}

// RangeExpr is a range expression (a..b, a.., ..b, ..).
type RangeExpr struct {
	Pos  Pos
	From Expr
	To   Expr
}

func (RangeExpr) astNode()  {}
func (RangeExpr) exprNode() {}

// ClosureExpr is a closure expression `|args| body`.
type ClosureExpr struct {
	Pos  Pos
	Args []string
	Body Expr
}

func (ClosureExpr) astNode()  {}
func (ClosureExpr) exprNode() {}

// UnaryExpr is a unary expression.
type UnaryExpr struct {
	Pos     Pos
	Op      string
	Operand Expr
}

func (UnaryExpr) astNode()  {}
func (UnaryExpr) exprNode() {}

// CastExpr is a numeric `as` cast.
type CastExpr struct {
	Pos  Pos
	Expr Expr
	Ty   Type
}

func (CastExpr) astNode()  {}
func (CastExpr) exprNode() {}

// CallExpr is a function call.
type CallExpr struct {
	Pos  Pos
	Func Expr
	Args []Expr
}

func (CallExpr) astNode()  {}
func (CallExpr) exprNode() {}

// IfExpr is an if/else expression.
type IfExpr struct {
	Pos       Pos
	Cond      Expr
	Pattern   Pattern
	ThenBlock *BlockExpr
	ElseBlock *BlockExpr
}

func (IfExpr) astNode()  {}
func (IfExpr) exprNode() {}

// MatchExpr is a Rust match expression.
type MatchExpr struct {
	Pos       Pos
	Scrutinee Expr
	Arms      []MatchArm
}

func (MatchExpr) astNode()  {}
func (MatchExpr) exprNode() {}

// MatchArm is one pattern group and its result expression.
type MatchArm struct {
	Patterns []Pattern
	Body     Expr
}

// WhileStmt is a while loop statement.
type WhileStmt struct {
	Pos  Pos
	Cond Expr
	Body *BlockExpr
}

func (WhileStmt) astNode()  {}
func (WhileStmt) stmtNode() {}

// ForStmt is a for-in loop: `for pattern in expr { body }`.
type ForStmt struct {
	Pos  Pos
	Pat  Pattern
	Iter Expr
	Body *BlockExpr
}

func (ForStmt) astNode()  {}
func (ForStmt) stmtNode() {}

// FieldExpr accesses a field on a struct.
type FieldExpr struct {
	Pos   Pos
	Expr  Expr
	Field string
}

func (FieldExpr) astNode()  {}
func (FieldExpr) exprNode() {}

// IndexExpr indexes into an array.
type IndexExpr struct {
	Pos   Pos
	Expr  Expr
	Index Expr
}

func (IndexExpr) astNode()  {}
func (IndexExpr) exprNode() {}

// StructLit creates a struct instance.
type StructLit struct {
	Pos    Pos
	Name   string
	Fields []FieldInit
}

func (StructLit) astNode()  {}
func (StructLit) exprNode() {}

// FieldInit is a field initializer.
type FieldInit struct {
	Pos   Pos
	Name  string
	Value Expr
}

// ArrayLit creates an array.
type ArrayLit struct {
	Pos   Pos
	Elems []Expr
}

func (ArrayLit) astNode()  {}
func (ArrayLit) exprNode() {}

// NamedType is a named type reference, optionally instantiated with type arguments.
type NamedType struct {
	Pos  Pos
	Name string
	Args []Type
}

func (NamedType) astNode()  {}
func (NamedType) typeNode() {}

// RefType is a reference type.
type RefType struct {
	Pos      Pos
	Lifetime string // empty if anonymous
	Elem     Type
	IsMut    bool
}

func (RefType) astNode()  {}
func (RefType) typeNode() {}

// ArrayType is an array type.
type ArrayType struct {
	Pos     Pos
	Elem    Type
	Len     int64
	LenName string // non-empty when the length is a const generic parameter ([T; N])
}

func (ArrayType) astNode()  {}
func (ArrayType) typeNode() {}

// ConstIntLitType represents an integer literal used as a const generic
// argument, e.g. `Arr<3>`. It is only produced in a type-argument position.
type ConstIntLitType struct {
	Pos Pos
	Val int64
}

func (ConstIntLitType) astNode()  {}
func (ConstIntLitType) typeNode() {}

// SliceType is a slice/unsized array type [T].
type SliceType struct {
	Pos  Pos
	Elem Type
}

func (SliceType) astNode()  {}
func (SliceType) typeNode() {}

// PosOf returns the source position of any node that implements GetPos.
func PosOf(n Node) Pos {
	if p, ok := n.(interface{ GetPos() Pos }); ok {
		return p.GetPos()
	}
	return 0
}

// Position accessors so diagnostics report accurate spans instead of 1:1.

func (n FnDecl) GetPos() Pos          { return n.Pos }
func (n StructDecl) GetPos() Pos      { return n.Pos }
func (n EnumDecl) GetPos() Pos        { return n.Pos }
func (n AssocTypeDecl) GetPos() Pos   { return n.Pos }
func (n AssocConstDecl) GetPos() Pos  { return n.Pos }
func (n TraitDecl) GetPos() Pos       { return n.Pos }
func (n ImplDecl) GetPos() Pos        { return n.Pos }
func (n MacroRulesDecl) GetPos() Pos  { return n.Pos }
func (n ModDecl) GetPos() Pos         { return n.Pos }
func (n UseDecl) GetPos() Pos         { return n.Pos }
func (n ExternCrateDecl) GetPos() Pos { return n.Pos }
func (n MacroCallDecl) GetPos() Pos   { return n.Pos }
func (n ConstDecl) GetPos() Pos       { return n.Pos }
func (n StaticDecl) GetPos() Pos      { return n.Pos }
func (n TypeAliasDecl) GetPos() Pos   { return n.Pos }
func (n BlockExpr) GetPos() Pos       { return n.Pos }
func (n UnsafeBlockExpr) GetPos() Pos { return n.Pos }
func (n TupleExpr) GetPos() Pos       { return n.Pos }
func (n IntLit) GetPos() Pos          { return n.Pos }
func (n BoolLit) GetPos() Pos         { return n.Pos }
func (n StringLit) GetPos() Pos       { return n.Pos }
func (n PathExpr) GetPos() Pos        { return n.Pos }
func (n Ident) GetPos() Pos           { return n.Pos }
func (n BinaryExpr) GetPos() Pos      { return n.Pos }
func (n RangeExpr) GetPos() Pos       { return n.Pos }
func (n ClosureExpr) GetPos() Pos     { return n.Pos }
func (n UnaryExpr) GetPos() Pos       { return n.Pos }
func (n CastExpr) GetPos() Pos        { return n.Pos }
func (n CallExpr) GetPos() Pos        { return n.Pos }
func (n IfExpr) GetPos() Pos          { return n.Pos }
func (n MatchExpr) GetPos() Pos       { return n.Pos }
func (n FieldExpr) GetPos() Pos       { return n.Pos }
func (n IndexExpr) GetPos() Pos       { return n.Pos }
func (n StructLit) GetPos() Pos       { return n.Pos }
func (n ArrayLit) GetPos() Pos        { return n.Pos }
func (n MacroCallExpr) GetPos() Pos   { return n.Pos }
func (n LetStmt) GetPos() Pos         { return n.Pos }
func (n AssignStmt) GetPos() Pos      { return n.Pos }
func (n ReturnStmt) GetPos() Pos      { return n.Pos }
func (n WhileStmt) GetPos() Pos       { return n.Pos }
func (n ForStmt) GetPos() Pos         { return n.Pos }
func (n PatIdent) GetPos() Pos        { return n.Pos }
func (n PatWildcard) GetPos() Pos     { return n.Pos }
func (n PatStruct) GetPos() Pos       { return n.Pos }
func (n PatField) GetPos() Pos        { return n.Pos }
func (n PatTuple) GetPos() Pos        { return n.Pos }
func (n PatSlice) GetPos() Pos        { return n.Pos }
func (n PatPath) GetPos() Pos         { return n.Pos }
func (n PatRange) GetPos() Pos        { return n.Pos }
func (n PatOr) GetPos() Pos           { return n.Pos }
func (n PatLit) GetPos() Pos          { return n.Pos }
func (n TupleType) GetPos() Pos       { return n.Pos }
func (n ImplTraitType) GetPos() Pos   { return n.Pos }
func (n NamedType) GetPos() Pos       { return n.Pos }
func (n RefType) GetPos() Pos         { return n.Pos }
func (n ArrayType) GetPos() Pos       { return n.Pos }
func (n ConstIntLitType) GetPos() Pos { return n.Pos }
func (n SliceType) GetPos() Pos       { return n.Pos }

// GetPos delegates to the wrapped expression (ExprStmt has no own position).
func (n ExprStmt) GetPos() Pos {
	if n.Expr != nil {
		return PosOf(n.Expr)
	}
	return 0
}

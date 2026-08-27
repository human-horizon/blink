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

// FnDecl represents a function declaration.
type FnDecl struct {
	Pos            Pos
	Pub            bool
	Name           string
	LifetimeParams []string
	GenParams      []string
	Params         []Param
	Ret            Type // nil if no explicit return type (-> ())
	Body           *BlockExpr
}

func (FnDecl) astNode()     {}
func (FnDecl) declNode()    {}
func (d FnDecl) IsPublic() bool { return d.Pub }


// Param represents a function parameter.
type Param struct {
	Pos    Pos
	Name   string
	Ty     Type
	IsSelf bool
}

// StructDecl represents a struct declaration.
type StructDecl struct {
	Pos            Pos
	Pub            bool
	Name           string
	LifetimeParams []string
	GenParams      []string
	Fields         []Field
}

func (StructDecl) astNode()  {}
func (StructDecl) declNode() {}
func (d StructDecl) IsPublic() bool { return d.Pub }


// Field represents a struct field.
type Field struct {
	Pos  Pos
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

func (EnumDecl) astNode()  {}
func (EnumDecl) declNode() {}
func (d EnumDecl) IsPublic() bool { return d.Pub }


// Variant represents an enum variant.
type Variant struct {
	Pos  Pos
	Name string
}

// TraitDecl represents a trait declaration.
type TraitDecl struct {
	Pos            Pos
	Pub            bool
	Name           string
	LifetimeParams []string
	GenParams      []string
	Methods        []*FnDecl
}

func (TraitDecl) astNode()  {}
func (TraitDecl) declNode() {}
func (d TraitDecl) IsPublic() bool { return d.Pub }


// ImplDecl represents an impl block (trait or inherent).
type ImplDecl struct {
	Pos     Pos
	Trait   string // empty for inherent impl
	ForType Type
	Methods []*FnDecl
}

func (ImplDecl) astNode()  {}
func (ImplDecl) declNode() {}
func (ImplDecl) IsPublic() bool { return true }

// ModDecl represents a module declaration.
type ModDecl struct {
	Pos       Pos
	Pub       bool
	Name      string
	Inline    *File // non-nil for inline modules
	File      string // file path for external modules
}

func (ModDecl) astNode()  {}
func (ModDecl) declNode() {}
func (d ModDecl) IsPublic() bool { return d.Pub }

// UseDecl represents a use/import declaration.
type UseDecl struct {
	Pos    Pos
	Path   []string
	Alias  string // empty if no alias
}

func (UseDecl) astNode()  {}
func (UseDecl) declNode() {}
func (UseDecl) IsPublic() bool { return false }


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
	Pos  Pos
	Name string
}

func (PatIdent) astNode()    {}
func (PatIdent) patternNode() {}

// PatWildcard ignores the matched value.
type PatWildcard struct {
	Pos Pos
}

func (PatWildcard) astNode()    {}
func (PatWildcard) patternNode() {}

// PatStruct matches a struct and binds its fields.
type PatStruct struct {
	Pos    Pos
	Name   string
	Fields []PatField
}

func (PatStruct) astNode()    {}
func (PatStruct) patternNode() {}

// PatField is a single field binding inside a struct pattern.
type PatField struct {
	Pos      Pos
	Field    string // field name in the struct
	BindName string // variable name to bind; empty means same as Field
}

func (PatField) astNode() {}

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
	Pos  Pos
	Op   string
	Left Expr
	Right Expr
}

func (BinaryExpr) astNode()  {}
func (BinaryExpr) exprNode() {}

// UnaryExpr is a unary expression.
type UnaryExpr struct {
	Pos    Pos
	Op     string
	Operand Expr
}

func (UnaryExpr) astNode()  {}
func (UnaryExpr) exprNode() {}

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
	ThenBlock *BlockExpr
	ElseBlock *BlockExpr
}

func (IfExpr) astNode()  {}
func (IfExpr) exprNode() {}

// WhileStmt is a while loop statement.
type WhileStmt struct {
	Pos  Pos
	Cond Expr
	Body *BlockExpr
}

func (WhileStmt) astNode()  {}
func (WhileStmt) stmtNode() {}

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
	Pos  Pos
	Elem Type
	Len  int64
}

func (ArrayType) astNode()  {}
func (ArrayType) typeNode() {}

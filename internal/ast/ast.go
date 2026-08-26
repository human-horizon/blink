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
}

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
	Pos    Pos
	Name   string
	Params []Param
	Ret    Type // nil if no explicit return type (-> ())
	Body   *BlockExpr
}

func (FnDecl) astNode()  {}
func (FnDecl) declNode() {}

// Param represents a function parameter.
type Param struct {
	Pos  Pos
	Name string
	Ty   Type
}

// StructDecl represents a struct declaration.
type StructDecl struct {
	Pos    Pos
	Name   string
	Fields []Field
}

func (StructDecl) astNode()  {}
func (StructDecl) declNode() {}

// Field represents a struct field.
type Field struct {
	Pos  Pos
	Name string
	Ty   Type
}

// EnumDecl represents an enum declaration.
type EnumDecl struct {
	Pos     Pos
	Name    string
	Variants []Variant
}

func (EnumDecl) astNode()  {}
func (EnumDecl) declNode() {}

// Variant represents an enum variant.
type Variant struct {
	Pos  Pos
	Name string
}

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
	Pos   Pos
	Name  string
	Ty    Type // nil if inferred
	Value Expr
}

func (LetStmt) astNode()  {}
func (LetStmt) stmtNode() {}

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

// NamedType is a named type reference.
type NamedType struct {
	Pos  Pos
	Name string
}

func (NamedType) astNode()  {}
func (NamedType) typeNode() {}

// RefType is a reference type.
type RefType struct {
	Pos      Pos
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

package checker

import (
	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/diag"
	"github.com/humanhorizon/blink/internal/types"
)

// Checker performs type checking across files.
type Checker struct {
	Files       []*ast.File
	Paths       []string
	Reporter    *diag.Reporter
	currentPath string
	fns         map[string]*ast.FnDecl
	structs     map[string]*ast.StructDecl
	enums       map[string]*ast.EnumDecl
}

// New creates a checker for the given parsed files and their paths.
func New(files []*ast.File, paths []string, r *diag.Reporter) *Checker {
	return &Checker{
		Files:    files,
		Paths:    paths,
		Reporter: r,
		fns:      make(map[string]*ast.FnDecl),
		structs:  make(map[string]*ast.StructDecl),
		enums:    make(map[string]*ast.EnumDecl),
	}
}

// Check runs collection and type checking. Returns true if no errors.
func (c *Checker) Check() bool {
	c.collect()
	for i, f := range c.Files {
		path := c.Paths[i]
		c.checkFile(f, path)
	}
	return !c.Reporter.HasErrors()
}

func (c *Checker) collect() {
	for _, f := range c.Files {
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.FnDecl:
				if _, ok := c.fns[decl.Name]; ok {
					c.errorf(decl.Pos, "duplicate function `%s`", decl.Name)
				} else {
					c.fns[decl.Name] = decl
				}
			case *ast.StructDecl:
				if _, ok := c.structs[decl.Name]; ok {
					c.errorf(decl.Pos, "duplicate struct `%s`", decl.Name)
				} else {
					c.structs[decl.Name] = decl
				}
			case *ast.EnumDecl:
				if _, ok := c.enums[decl.Name]; ok {
					c.errorf(decl.Pos, "duplicate enum `%s`", decl.Name)
				} else {
					c.enums[decl.Name] = decl
				}
			}
		}
	}
}

func (c *Checker) checkFile(f *ast.File, path string) {
	c.currentPath = path
	for _, d := range f.Decls {
		if fn, ok := d.(*ast.FnDecl); ok {
			c.checkFn(fn, path)
		}
	}
}

func (c *Checker) checkFn(fn *ast.FnDecl, path string) {
	env := newEnv(nil)
	var paramTypes []types.Type
	for _, p := range fn.Params {
		ty := c.resolveType(p.Ty, path)
		paramTypes = append(paramTypes, ty)
		env.set(p.Name, ty)
	}
	var ret types.Type = types.Unit
	if fn.Ret != nil {
		ret = c.resolveType(fn.Ret, path)
	}
	bodyTy := c.checkBlock(fn.Body, env, ret, path)
	if !bodyTy.Equals(ret) && !isError(bodyTy) {
		c.errorf(PosOf(fn.Body), "expected `%s`, found `%s`", ret, bodyTy)
	}
	_ = paramTypes
}

func (c *Checker) checkBlock(block *ast.BlockExpr, env *environment, ret types.Type, path string) types.Type {
	local := newEnv(env)
	for _, s := range block.Stmts {
		c.checkStmt(s, local, ret, path)
	}
	if block.Result != nil {
		return c.checkExpr(block.Result, local, path)
	}
	return types.Unit
}

func (c *Checker) checkStmt(s ast.Stmt, env *environment, ret types.Type, path string) {
	switch st := s.(type) {
	case *ast.LetStmt:
		valTy := c.checkExpr(st.Value, env, path)
		if st.Ty != nil {
			annot := c.resolveType(st.Ty, path)
			if !annot.Equals(valTy) && !isError(valTy) {
				c.errorf(PosOf(st.Value), "expected `%s`, found `%s`", annot, valTy)
			}
			env.set(st.Name, annot)
		} else {
			env.set(st.Name, valTy)
		}
	case *ast.ReturnStmt:
		var ty types.Type = types.Unit
		if st.Expr != nil {
			ty = c.checkExpr(st.Expr, env, path)
		}
		if !ty.Equals(ret) && !isError(ty) {
			c.errorf(st.Pos, "expected `%s`, found `%s`", ret, ty)
		}
	case *ast.WhileStmt:
		cond := c.checkExpr(st.Cond, env, path)
		if !cond.Equals(types.Bool) && !isError(cond) {
			c.errorf(PosOf(st.Cond), "expected `bool`, found `%s`", cond)
		}
		c.checkBlock(st.Body, env, ret, path)
	case *ast.ExprStmt:
		c.checkExpr(st.Expr, env, path)
	}
}

func (c *Checker) checkExpr(expr ast.Expr, env *environment, path string) types.Type {
	switch e := expr.(type) {
	case *ast.IntLit:
		return types.I32
	case *ast.BoolLit:
		return types.Bool
	case *ast.StringLit:
		return types.String
	case *ast.Ident:
		ty, ok := env.get(e.Name)
		if !ok {
			c.errorf(e.Pos, "cannot find value `%s` in this scope", e.Name)
			return &types.Error{}
		}
		return ty
	case *ast.BinaryExpr:
		return c.checkBinary(e, env, path)
	case *ast.UnaryExpr:
		return c.checkUnary(e, env, path)
	case *ast.CallExpr:
		return c.checkCall(e, env, path)
	case *ast.BlockExpr:
		return c.checkBlock(e, env, types.Unit, path)
	case *ast.IfExpr:
		return c.checkIf(e, env, path)
	case *ast.FieldExpr:
		return c.checkField(e, env, path)
	case *ast.IndexExpr:
		return c.checkIndex(e, env, path)
	case *ast.StructLit:
		return c.checkStructLit(e, env, path)
	case *ast.ArrayLit:
		return c.checkArrayLit(e, env, path)
	default:
		c.errorf(PosOf(expr), "unsupported expression")
		return &types.Error{}
	}
}

func (c *Checker) checkBinary(e *ast.BinaryExpr, env *environment, path string) types.Type {
	left := c.checkExpr(e.Left, env, path)
	right := c.checkExpr(e.Right, env, path)
	switch e.Op {
	case "+", "-", "*", "/":
		if !left.Equals(types.I32) && !isError(left) {
			c.errorf(PosOf(e.Left), "expected `i32`, found `%s`", left)
		}
		if !right.Equals(types.I32) && !isError(right) {
			c.errorf(PosOf(e.Right), "expected `i32`, found `%s`", right)
		}
		return types.I32
	case "==", "!=", "<", ">":
		if !left.Equals(types.I32) && !isError(left) {
			c.errorf(PosOf(e.Left), "expected `i32`, found `%s`", left)
		}
		if !right.Equals(types.I32) && !isError(right) {
			c.errorf(PosOf(e.Right), "expected `i32`, found `%s`", right)
		}
		return types.Bool
	case "&&", "||":
		if !left.Equals(types.Bool) && !isError(left) {
			c.errorf(PosOf(e.Left), "expected `bool`, found `%s`", left)
		}
		if !right.Equals(types.Bool) && !isError(right) {
			c.errorf(PosOf(e.Right), "expected `bool`, found `%s`", right)
		}
		return types.Bool
	default:
		c.errorf(e.Pos, "unsupported binary operator `%s`", e.Op)
		return &types.Error{}
	}
}

func (c *Checker) checkUnary(e *ast.UnaryExpr, env *environment, path string) types.Type {
	ty := c.checkExpr(e.Operand, env, path)
	switch e.Op {
	case "-":
		if !ty.Equals(types.I32) && !isError(ty) {
			c.errorf(PosOf(e.Operand), "expected `i32`, found `%s`", ty)
		}
		return types.I32
	case "!":
		if !ty.Equals(types.Bool) && !isError(ty) {
			c.errorf(PosOf(e.Operand), "expected `bool`, found `%s`", ty)
		}
		return types.Bool
	default:
		c.errorf(e.Pos, "unsupported unary operator `%s`", e.Op)
		return &types.Error{}
	}
}

func (c *Checker) checkCall(e *ast.CallExpr, env *environment, path string) types.Type {
	ident, ok := e.Func.(*ast.Ident)
	if !ok {
		c.errorf(PosOf(e.Func), "only direct function calls are supported")
		return &types.Error{}
	}
	fn, ok := c.fns[ident.Name]
	if !ok {
		c.errorf(ident.Pos, "cannot find function `%s`", ident.Name)
		return &types.Error{}
	}
	if len(fn.Params) != len(e.Args) {
		c.errorf(e.Pos, "expected %d arguments, found %d", len(fn.Params), len(e.Args))
		return &types.Error{}
	}
	for i, arg := range e.Args {
		argTy := c.checkExpr(arg, env, path)
		paramTy := c.resolveType(fn.Params[i].Ty, path)
		if !argTy.Equals(paramTy) && !isError(argTy) {
			c.errorf(PosOf(arg), "expected `%s`, found `%s`", paramTy, argTy)
		}
	}
	if fn.Ret != nil {
		return c.resolveType(fn.Ret, path)
	}
	return types.Unit
}

func (c *Checker) checkIf(e *ast.IfExpr, env *environment, path string) types.Type {
	cond := c.checkExpr(e.Cond, env, path)
	if !cond.Equals(types.Bool) && !isError(cond) {
		c.errorf(PosOf(e.Cond), "expected `bool`, found `%s`", cond)
	}
	thenTy := c.checkBlock(e.ThenBlock, env, types.Unit, path)
	if e.ElseBlock != nil {
		elseTy := c.checkBlock(e.ElseBlock, env, types.Unit, path)
		if !thenTy.Equals(elseTy) && !isError(thenTy) && !isError(elseTy) {
			c.errorf(PosOf(e.ElseBlock), "expected `%s`, found `%s`", thenTy, elseTy)
		}
		return thenTy
	}
	return types.Unit
}

func (c *Checker) checkField(e *ast.FieldExpr, env *environment, path string) types.Type {
	base := c.checkExpr(e.Expr, env, path)
	named, ok := base.(*types.Named)
	if !ok {
		if !isError(base) {
			c.errorf(PosOf(e.Expr), "expected struct, found `%s`", base)
		}
		return &types.Error{}
	}
	st, ok := c.structs[named.Name]
	if !ok {
		c.errorf(PosOf(e.Expr), "unknown type `%s`", named.Name)
		return &types.Error{}
	}
	for _, f := range st.Fields {
		if f.Name == e.Field {
			return c.resolveType(f.Ty, path)
		}
	}
	c.errorf(e.Pos, "no field `%s` on struct `%s`", e.Field, named.Name)
	return &types.Error{}
}

func (c *Checker) checkIndex(e *ast.IndexExpr, env *environment, path string) types.Type {
	base := c.checkExpr(e.Expr, env, path)
	arr, ok := base.(*types.Array)
	if !ok {
		if !isError(base) {
			c.errorf(PosOf(e.Expr), "expected array, found `%s`", base)
		}
		return &types.Error{}
	}
	idx := c.checkExpr(e.Index, env, path)
	if !idx.Equals(types.I32) && !isError(idx) {
		c.errorf(PosOf(e.Index), "expected `i32`, found `%s`", idx)
	}
	return arr.Elem
}

func (c *Checker) checkStructLit(e *ast.StructLit, env *environment, path string) types.Type {
	st, ok := c.structs[e.Name]
	if !ok {
		c.errorf(e.Pos, "unknown struct `%s`", e.Name)
		return &types.Error{}
	}
	fieldMap := make(map[string]types.Type)
	for _, f := range st.Fields {
		fieldMap[f.Name] = c.resolveType(f.Ty, path)
	}
	provided := make(map[string]bool)
	for _, init := range e.Fields {
		expected, ok := fieldMap[init.Name]
		if !ok {
			c.errorf(init.Pos, "unknown field `%s` on struct `%s`", init.Name, e.Name)
			continue
		}
		valTy := c.checkExpr(init.Value, env, path)
		if !valTy.Equals(expected) && !isError(valTy) {
			c.errorf(PosOf(init.Value), "expected `%s`, found `%s`", expected, valTy)
		}
		provided[init.Name] = true
	}
	for name := range fieldMap {
		if !provided[name] {
			c.errorf(e.Pos, "missing field `%s` in initializer of struct `%s`", name, e.Name)
		}
	}
	return &types.Named{Name: e.Name}
}

func (c *Checker) checkArrayLit(e *ast.ArrayLit, env *environment, path string) types.Type {
	if len(e.Elems) == 0 {
		c.errorf(e.Pos, "cannot infer type of empty array")
		return &types.Error{}
	}
	elemTy := c.checkExpr(e.Elems[0], env, path)
	for _, elem := range e.Elems[1:] {
		ty := c.checkExpr(elem, env, path)
		if !ty.Equals(elemTy) && !isError(ty) {
			c.errorf(PosOf(elem), "expected `%s`, found `%s`", elemTy, ty)
		}
	}
	return &types.Array{Elem: elemTy, Len: int64(len(e.Elems))}
}

func (c *Checker) resolveType(t ast.Type, path string) types.Type {
	switch ty := t.(type) {
	case *ast.NamedType:
		switch ty.Name {
		case "i32":
			return types.I32
		case "bool":
			return types.Bool
		default:
			if _, ok := c.structs[ty.Name]; ok {
				return &types.Named{Name: ty.Name}
			}
			if _, ok := c.enums[ty.Name]; ok {
				return &types.Named{Name: ty.Name}
			}
			c.errorf(ty.Pos, "unknown type `%s`", ty.Name)
			return &types.Error{}
		}
	case *ast.RefType:
		elem := c.resolveType(ty.Elem, path)
		return &types.Ref{Elem: elem, IsMut: ty.IsMut}
	case *ast.ArrayType:
		elem := c.resolveType(ty.Elem, path)
		return &types.Array{Elem: elem, Len: ty.Len}
	default:
		c.errorf(PosOf(t), "unsupported type")
		return &types.Error{}
	}
}

func (c *Checker) errorf(pos ast.Pos, format string, args ...interface{}) {
	// Positions are byte offsets; line/col conversion is skipped for MVP speed.
	c.Reporter.Errorf(c.currentPath, 1, int(pos), format, args...)
}

func isError(t types.Type) bool {
	_, ok := t.(*types.Error)
	return ok
}

// PosOf returns the position of an expression node if available.
func PosOf(n ast.Node) ast.Pos {
	switch x := n.(type) {
	case *ast.IntLit:
		return x.Pos
	case *ast.BoolLit:
		return x.Pos
	case *ast.StringLit:
		return x.Pos
	case *ast.Ident:
		return x.Pos
	case *ast.BinaryExpr:
		return x.Pos
	case *ast.UnaryExpr:
		return x.Pos
	case *ast.CallExpr:
		return x.Pos
	case *ast.BlockExpr:
		return x.Pos
	case *ast.IfExpr:
		return x.Pos
	case *ast.FieldExpr:
		return x.Pos
	case *ast.IndexExpr:
		return x.Pos
	case *ast.StructLit:
		return x.Pos
	case *ast.ArrayLit:
		return x.Pos
	default:
		return 0
	}
}

// environment maps names to their types.
type environment struct {
	parent *environment
	vars   map[string]types.Type
}

func newEnv(parent *environment) *environment {
	return &environment{parent: parent, vars: make(map[string]types.Type)}
}

func (e *environment) set(name string, ty types.Type) {
	e.vars[name] = ty
}

func (e *environment) get(name string) (types.Type, bool) {
	if ty, ok := e.vars[name]; ok {
		return ty, true
	}
	if e.parent != nil {
		return e.parent.get(name)
	}
	return nil, false
}

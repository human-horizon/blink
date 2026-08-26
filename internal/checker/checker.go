package checker

import (
	"fmt"

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
	fns         map[string]*fnInfo
	structs     map[string]*structInfo
	enums       map[string]*enumInfo
	traits      map[string]*traitInfo
	inherent    map[string]map[string]*fnInfo
	traitImpls  map[string]map[string]*implInfo
}

type fnInfo struct {
	decl       *ast.FnDecl
	genParams  []string
	paramTypes []types.Type
	ret        types.Type
	selfType   types.Type
}

type structInfo struct {
	decl      *ast.StructDecl
	genParams []string
	fields    map[string]types.Type
}

type enumInfo struct {
	decl      *ast.EnumDecl
	genParams []string
}

type traitInfo struct {
	decl    *ast.TraitDecl
	methods map[string]*fnInfo
}

type implInfo struct {
	decl    *ast.ImplDecl
	trait   string
	forType types.Type
	methods map[string]*fnInfo
}

// New creates a checker for the given parsed files and their paths.
func New(files []*ast.File, paths []string, r *diag.Reporter) *Checker {
	return &Checker{
		Files:      files,
		Paths:      paths,
		Reporter:   r,
		fns:        make(map[string]*fnInfo),
		structs:    make(map[string]*structInfo),
		enums:      make(map[string]*enumInfo),
		traits:     make(map[string]*traitInfo),
		inherent:   make(map[string]map[string]*fnInfo),
		traitImpls: make(map[string]map[string]*implInfo),
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
	// First pass: register names so forward references work.
	for _, f := range c.Files {
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.FnDecl:
				if _, ok := c.fns[decl.Name]; ok {
					c.errorf(decl.Pos, "duplicate function `%s`", decl.Name)
				} else {
					c.fns[decl.Name] = &fnInfo{decl: decl, genParams: decl.GenParams}
				}
			case *ast.StructDecl:
				if _, ok := c.structs[decl.Name]; ok {
					c.errorf(decl.Pos, "duplicate struct `%s`", decl.Name)
				} else {
					c.structs[decl.Name] = &structInfo{decl: decl, genParams: decl.GenParams}
				}
			case *ast.EnumDecl:
				if _, ok := c.enums[decl.Name]; ok {
					c.errorf(decl.Pos, "duplicate enum `%s`", decl.Name)
				} else {
					c.enums[decl.Name] = &enumInfo{decl: decl, genParams: decl.GenParams}
				}
			case *ast.TraitDecl:
				if _, ok := c.traits[decl.Name]; ok {
					c.errorf(decl.Pos, "duplicate trait `%s`", decl.Name)
				} else {
					c.traits[decl.Name] = &traitInfo{decl: decl, methods: make(map[string]*fnInfo)}
				}
			}
		}
	}
	// Second pass: resolve free function, struct, and trait method types.
	for _, info := range c.fns {
		c.fillFnInfo(info, c.currentPath)
	}
	for _, info := range c.structs {
		info.fields = make(map[string]types.Type)
		for _, f := range info.decl.Fields {
			info.fields[f.Name] = c.resolveType(f.Ty, c.currentPath, info.decl.GenParams)
		}
	}
	for _, info := range c.traits {
		for _, m := range info.decl.Methods {
			minfo := &fnInfo{decl: m, genParams: m.GenParams}
			c.fillMethodInfo(minfo, m, []string{"Self"}, &types.Generic{Name: "Self"})
			info.methods[m.Name] = minfo
		}
	}
	// Third pass: collect impl blocks.
	for _, f := range c.Files {
		for _, d := range f.Decls {
			if impl, ok := d.(*ast.ImplDecl); ok {
				c.collectImpl(impl)
			}
		}
	}
}

func (c *Checker) fillFnInfo(info *fnInfo, path string) {
	for _, p := range info.decl.Params {
		info.paramTypes = append(info.paramTypes, c.resolveType(p.Ty, path, info.decl.GenParams))
	}
	info.ret = types.Unit
	if info.decl.Ret != nil {
		info.ret = c.resolveType(info.decl.Ret, path, info.decl.GenParams)
	}
}

func (c *Checker) fillMethodInfo(info *fnInfo, decl *ast.FnDecl, genParams []string, selfTy types.Type) {
	for _, p := range decl.Params {
		if p.IsSelf {
			info.paramTypes = append(info.paramTypes, selfTy)
			info.selfType = selfTy
		} else {
			info.paramTypes = append(info.paramTypes, c.resolveType(p.Ty, c.currentPath, genParams))
		}
	}
	info.ret = types.Unit
	if decl.Ret != nil {
		info.ret = c.resolveType(decl.Ret, c.currentPath, genParams)
	}
}

func (c *Checker) collectImpl(impl *ast.ImplDecl) {
	forType := c.resolveType(impl.ForType, c.currentPath, nil)
	typeName := c.typeName(forType)
	if typeName == "" {
		c.errorf(impl.Pos, "impl must be for a named type")
		return
	}
	selfRef := &types.Ref{Elem: forType, IsMut: false}
	methods := make(map[string]*fnInfo)
	for _, m := range impl.Methods {
		minfo := &fnInfo{decl: m, genParams: m.GenParams}
		c.fillMethodInfo(minfo, m, append([]string{"Self"}, m.GenParams...), selfRef)
		methods[m.Name] = minfo
	}
	if impl.Trait == "" {
		if _, ok := c.inherent[typeName]; !ok {
			c.inherent[typeName] = make(map[string]*fnInfo)
		}
		for name, minfo := range methods {
			if _, ok := c.inherent[typeName][name]; ok {
				c.errorf(minfo.decl.Pos, "duplicate inherent method `%s` for `%s`", name, typeName)
				continue
			}
			c.inherent[typeName][name] = minfo
		}
	} else {
		tr, ok := c.traits[impl.Trait]
		if !ok {
			c.errorf(impl.Pos, "unknown trait `%s`", impl.Trait)
			return
		}
		for name, expected := range tr.methods {
			provided, ok := methods[name]
			if !ok {
				c.errorf(impl.Pos, "missing method `%s` for trait `%s`", name, impl.Trait)
				continue
			}
			if !c.fnSigMatches(expected, provided) {
				c.errorf(provided.decl.Pos, "method `%s` has incompatible signature with trait `%s`", name, impl.Trait)
			}
		}
		for name := range methods {
			if _, ok := tr.methods[name]; !ok {
				c.errorf(methods[name].decl.Pos, "method `%s` is not part of trait `%s`", name, impl.Trait)
			}
		}
		if _, ok := c.traitImpls[impl.Trait]; !ok {
			c.traitImpls[impl.Trait] = make(map[string]*implInfo)
		}
		c.traitImpls[impl.Trait][typeName] = &implInfo{decl: impl, trait: impl.Trait, forType: forType, methods: methods}
	}
}

func (c *Checker) fnSigMatches(a, b *fnInfo) bool {
	if len(a.paramTypes) != len(b.paramTypes) {
		return false
	}
	for i, pa := range a.paramTypes {
		if !pa.Equals(b.paramTypes[i]) {
			return false
		}
	}
	return a.ret.Equals(b.ret)
}

func (c *Checker) checkFile(f *ast.File, path string) {
	c.currentPath = path
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FnDecl:
			c.checkFn(decl, path)
		case *ast.ImplDecl:
			c.checkImpl(decl, path)
		}
	}
}

func (c *Checker) checkFn(fn *ast.FnDecl, path string) {
	env := newEnv(nil)
	info := c.fns[fn.Name]
	if info == nil {
		c.errorf(fn.Pos, "internal error: missing function info for `%s`", fn.Name)
		return
	}
	for i, p := range fn.Params {
		env.set(p.Name, info.paramTypes[i], true)
	}
	loans := newBorrowCtx(nil)
	bodyTy := c.checkBlock(fn.Body, env, loans, info.ret, path)
	if !bodyTy.Equals(info.ret) && !isError(bodyTy) {
		c.errorf(PosOf(fn.Body), "expected `%s`, found `%s`", info.ret, bodyTy)
	}
}

func (c *Checker) checkImpl(impl *ast.ImplDecl, path string) {
	forType := c.resolveType(impl.ForType, path, nil)
	typeName := c.typeName(forType)
	var implMethods map[string]*fnInfo
	if impl.Trait == "" {
		implMethods = c.inherent[typeName]
	} else {
		if m, ok := c.traitImpls[impl.Trait]; ok {
			if info, ok := m[typeName]; ok {
				implMethods = info.methods
			}
		}
	}
	if implMethods == nil {
		return
	}
	for _, m := range impl.Methods {
		minfo, ok := implMethods[m.Name]
		if !ok {
			continue
		}
		env := newEnv(nil)
		env.set("self", &types.Ref{Elem: forType, IsMut: false}, false)
		for i, p := range m.Params {
			if !p.IsSelf {
				env.set(p.Name, minfo.paramTypes[i], true)
			}
		}
		loans := newBorrowCtx(nil)
		bodyTy := c.checkBlock(m.Body, env, loans, minfo.ret, path)
		if !bodyTy.Equals(minfo.ret) && !isError(bodyTy) {
			c.errorf(PosOf(m.Body), "expected `%s`, found `%s`", minfo.ret, bodyTy)
		}
	}
}

func (c *Checker) checkExpr(expr ast.Expr, env *environment, loans *borrowCtx, path string) types.Type {
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
			if _, ok := c.structs[e.Name]; ok {
				return &types.Named{Name: e.Name}
			}
			if _, ok := c.enums[e.Name]; ok {
				return &types.Named{Name: e.Name}
			}
			c.errorf(e.Pos, "cannot find value `%s` in this scope", e.Name)
			return &types.Error{}
		}
		if loans != nil {
			c.useRead(loans, e, ty)
		}
		return ty
	case *ast.BinaryExpr:
		return c.checkBinary(e, env, loans, path)
	case *ast.UnaryExpr:
		return c.checkUnary(e, env, loans, path)
	case *ast.CallExpr:
		return c.checkCall(e, env, loans, path)
	case *ast.BlockExpr:
		return c.checkBlock(e, env, loans, types.Unit, path)
	case *ast.IfExpr:
		return c.checkIf(e, env, loans, path)
	case *ast.FieldExpr:
		return c.checkField(e, env, loans, path)
	case *ast.IndexExpr:
		return c.checkIndex(e, env, loans, path)
	case *ast.StructLit:
		return c.checkStructLit(e, env, loans, path)
	case *ast.ArrayLit:
		return c.checkArrayLit(e, env, loans, path)
	default:
		c.errorf(PosOf(expr), "unsupported expression")
		return &types.Error{}
	}
}

func (c *Checker) checkBinary(e *ast.BinaryExpr, env *environment, loans *borrowCtx, path string) types.Type {
	left := c.checkExpr(e.Left, env, loans, path)
	right := c.checkExpr(e.Right, env, loans, path)
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

func (c *Checker) checkUnary(e *ast.UnaryExpr, env *environment, loans *borrowCtx, path string) types.Type {
	ty := c.checkExpr(e.Operand, env, loans, path)
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
	case "&", "&mut":
		isMut := e.Op == "&mut"
		ref := &types.Ref{Elem: ty, IsMut: isMut}
		if loans != nil {
			if isMut {
				c.borrowMut(loans, env, e.Operand, ty)
			} else {
				c.borrowShared(loans, e.Operand, ty)
			}
		}
		return ref
	case "*":
		ref, ok := ty.(*types.Ref)
		if !ok {
			if !isError(ty) {
				c.errorf(PosOf(e.Operand), "expected reference, found `%s`", ty)
			}
			return &types.Error{}
		}
		if loans != nil {
			if ref.IsMut {
				c.useWrite(loans, e.Operand, ty)
			} else {
				c.useRead(loans, e.Operand, ty)
			}
		}
		return ref.Elem
	default:
		c.errorf(e.Pos, "unsupported unary operator `%s`", e.Op)
		return &types.Error{}
	}
}

func (c *Checker) checkCall(e *ast.CallExpr, env *environment, loans *borrowCtx, path string) types.Type {
	switch fn := e.Func.(type) {
	case *ast.Ident:
		info, ok := c.fns[fn.Name]
		if !ok {
			c.errorf(fn.Pos, "cannot find function `%s`", fn.Name)
			return &types.Error{}
		}
		return c.checkFnCall(info, e.Args, nil, env, loans, path)
	case *ast.FieldExpr:
		return c.checkMethodCall(fn, e.Args, env, loans, path)
	default:
		c.errorf(PosOf(e.Func), "only direct function or method calls are supported")
		return &types.Error{}
	}
}

func (c *Checker) checkFnCall(info *fnInfo, args []ast.Expr, receiver types.Type, env *environment, loans *borrowCtx, path string) types.Type {
	if len(info.paramTypes) != len(args) {
		c.errorf(PosOf(args[0]), "expected %d arguments, found %d", len(info.paramTypes), len(args))
		return &types.Error{}
	}
	mapping := make(map[string]types.Type)
	if receiver != nil {
		mapping["Self"] = receiver
	}
	for i, arg := range args {
		argTy := c.checkExpr(arg, env, loans, path)
		paramTy := info.paramTypes[i]
		if !types.Unify(paramTy, argTy, mapping) && !isError(argTy) {
			c.errorf(PosOf(args[i]), "expected `%s`, found `%s`", paramTy, argTy)
		}
	}
	for _, name := range info.genParams {
		if _, ok := mapping[name]; !ok {
			c.errorf(PosOf(args[0]), "cannot infer type parameter `%s`", name)
		}
	}
	return types.Substitute(info.ret, mapping)
}

func (c *Checker) checkMethodCall(field *ast.FieldExpr, args []ast.Expr, env *environment, loans *borrowCtx, path string) types.Type {
	recvExpr := field.Expr
	recvTy := c.checkExpr(recvExpr, env, loans, path)
	methodName := field.Field
	static := c.isTypeName(recvExpr, env)
	if static {
		typeName := c.typeName(recvTy)
		if typeName == "" {
			c.errorf(PosOf(recvExpr), "expected type name for static call")
			return &types.Error{}
		}
		m := c.findInherentMethod(typeName, methodName)
		if m == nil {
			c.errorf(PosOf(field), "no static method `%s` found for type `%s`", methodName, typeName)
			return &types.Error{}
		}
		if m.selfType != nil {
			c.errorf(PosOf(field), "method `%s` requires an instance", methodName)
			return &types.Error{}
		}
		return c.checkFnCall(m, args, nil, env, loans, path)
	}
	baseTy := c.deref(recvTy)
	baseName := c.typeName(baseTy)
	if baseName == "" {
		c.errorf(PosOf(recvExpr), "method calls require a named type")
		return &types.Error{}
	}
	m := c.findInherentMethod(baseName, methodName)
	if m == nil {
		for _, impls := range c.traitImpls {
			if impl, ok := impls[baseName]; ok {
				if m2, ok := impl.methods[methodName]; ok {
					m = m2
					break
				}
			}
		}
	}
	if m == nil {
		c.errorf(PosOf(field), "no method `%s` found for type `%s`", methodName, baseName)
		return &types.Error{}
	}
	if m.selfType == nil {
		c.errorf(PosOf(field), "method `%s` is not an instance method", methodName)
		return &types.Error{}
	}
	expectedSelf := &types.Ref{Elem: baseTy, IsMut: false}
	if !m.selfType.Equals(expectedSelf) && !m.selfType.Equals(baseTy) {
		c.errorf(PosOf(recvExpr), "expected `%s`, found `%s`", m.selfType, recvTy)
		return &types.Error{}
	}
	if loans != nil {
		c.borrowShared(loans, recvExpr, recvTy)
	}
	minfo := *m
	minfo.paramTypes = minfo.paramTypes[1:]
	return c.checkFnCall(&minfo, args, baseTy, env, loans, path)
}

func (c *Checker) deref(t types.Type) types.Type {
	if r, ok := t.(*types.Ref); ok {
		return r.Elem
	}
	return t
}

func (c *Checker) isTypeName(expr ast.Expr, env *environment) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	if _, ok := env.vars[ident.Name]; ok {
		return false
	}
	_, isStruct := c.structs[ident.Name]
	_, isEnum := c.enums[ident.Name]
	return isStruct || isEnum
}

func (c *Checker) findInherentMethod(typeName, method string) *fnInfo {
	if m, ok := c.inherent[typeName]; ok {
		if info, ok := m[method]; ok {
			return info
		}
	}
	return nil
}

func (c *Checker) typeName(t types.Type) string {
	switch ty := t.(type) {
	case *types.Named:
		return ty.Name
	case *types.Applied:
		if named, ok := ty.Base.(*types.Named); ok {
			return named.Name
		}
	}
	return ""
}

func (c *Checker) checkIf(e *ast.IfExpr, env *environment, loans *borrowCtx, path string) types.Type {
	cond := c.checkExpr(e.Cond, env, loans, path)
	if !cond.Equals(types.Bool) && !isError(cond) {
		c.errorf(PosOf(e.Cond), "expected `bool`, found `%s`", cond)
	}
	thenTy := c.checkBlock(e.ThenBlock, env, loans, types.Unit, path)
	if e.ElseBlock != nil {
		elseTy := c.checkBlock(e.ElseBlock, env, loans, types.Unit, path)
		if !thenTy.Equals(elseTy) && !isError(thenTy) && !isError(elseTy) {
			c.errorf(PosOf(e.ElseBlock), "expected `%s`, found `%s`", thenTy, elseTy)
		}
		return thenTy
	}
	return types.Unit
}

func (c *Checker) checkField(e *ast.FieldExpr, env *environment, loans *borrowCtx, path string) types.Type {
	base := c.checkExpr(e.Expr, env, loans, path)
	base = c.deref(base)
	switch b := base.(type) {
	case *types.Named:
		return c.fieldType(b.Name, nil, e.Field, e)
	case *types.Applied:
		if named, ok := b.Base.(*types.Named); ok {
			return c.fieldType(named.Name, b.Args, e.Field, e)
		}
		if !isError(base) {
			c.errorf(PosOf(e.Expr), "expected struct, found `%s`", base)
		}
		return &types.Error{}
	default:
		if !isError(base) {
			c.errorf(PosOf(e.Expr), "expected struct, found `%s`", base)
		}
		return &types.Error{}
	}
}

func (c *Checker) fieldType(name string, args []types.Type, field string, e ast.Expr) types.Type {
	st, ok := c.structs[name]
	if !ok {
		c.errorf(PosOf(e), "unknown type `%s`", name)
		return &types.Error{}
	}
	fty, ok := st.fields[field]
	if !ok {
		c.errorf(PosOf(e), "no field `%s` on struct `%s`", field, name)
		return &types.Error{}
	}
	if len(args) > 0 {
		if len(st.genParams) != len(args) {
			c.errorf(PosOf(e), "expected %d generic arguments, found %d", len(st.genParams), len(args))
			return &types.Error{}
		}
		mapping := make(map[string]types.Type)
		for i, p := range st.genParams {
			mapping[p] = args[i]
		}
		return types.Substitute(fty, mapping)
	}
	if len(st.genParams) > 0 {
		c.errorf(PosOf(e), "struct `%s` requires generic arguments", name)
		return &types.Error{}
	}
	return fty
}

func (c *Checker) checkIndex(e *ast.IndexExpr, env *environment, loans *borrowCtx, path string) types.Type {
	base := c.checkExpr(e.Expr, env, loans, path)
	arr, ok := base.(*types.Array)
	if !ok {
		if !isError(base) {
			c.errorf(PosOf(e.Expr), "expected array, found `%s`", base)
		}
		return &types.Error{}
	}
	idx := c.checkExpr(e.Index, env, loans, path)
	if !idx.Equals(types.I32) && !isError(idx) {
		c.errorf(PosOf(e.Index), "expected `i32`, found `%s`", idx)
	}
	return arr.Elem
}

func (c *Checker) checkStructLit(e *ast.StructLit, env *environment, loans *borrowCtx, path string) types.Type {
	st, ok := c.structs[e.Name]
	if !ok {
		c.errorf(e.Pos, "unknown struct `%s`", e.Name)
		return &types.Error{}
	}
	if len(st.genParams) > 0 {
		c.errorf(e.Pos, "cannot infer generic arguments for struct `%s`; annotate with type", e.Name)
		return &types.Error{}
	}
	return c.checkStructLitFields(e, st, nil, env, loans, path)
}

func (c *Checker) checkStructLitWithAnnotation(e *ast.StructLit, annot types.Type, env *environment, loans *borrowCtx, path string) types.Type {
	applied, ok := annot.(*types.Applied)
	if !ok {
		return c.checkStructLit(e, env, loans, path)
	}
	named, ok := applied.Base.(*types.Named)
	if !ok {
		c.errorf(e.Pos, "expected struct type, found `%s`", applied.Base)
		return &types.Error{}
	}
	st, ok := c.structs[named.Name]
	if !ok {
		c.errorf(e.Pos, "unknown struct `%s`", named.Name)
		return &types.Error{}
	}
	if len(st.genParams) != len(applied.Args) {
		c.errorf(e.Pos, "expected %d generic arguments, found %d", len(st.genParams), len(applied.Args))
		return &types.Error{}
	}
	return c.checkStructLitFields(e, st, applied.Args, env, loans, path)
}

func (c *Checker) checkStructLitFields(e *ast.StructLit, st *structInfo, args []types.Type, env *environment, loans *borrowCtx, path string) types.Type {
	fieldMap := make(map[string]types.Type)
	for name, fty := range st.fields {
		fieldMap[name] = fty
	}
	if len(args) > 0 {
		mapping := make(map[string]types.Type)
		for i, p := range st.genParams {
			mapping[p] = args[i]
		}
		for name, fty := range fieldMap {
			fieldMap[name] = types.Substitute(fty, mapping)
		}
	}
	provided := make(map[string]bool)
	for _, init := range e.Fields {
		expected, ok := fieldMap[init.Name]
		if !ok {
			c.errorf(init.Pos, "unknown field `%s` on struct `%s`", init.Name, st.decl.Name)
			continue
		}
		valTy := c.checkExpr(init.Value, env, loans, path)
		if !valTy.Equals(expected) && !isError(valTy) {
			c.errorf(PosOf(init.Value), "expected `%s`, found `%s`", expected, valTy)
		}
		provided[init.Name] = true
	}
	for name := range fieldMap {
		if !provided[name] {
			c.errorf(e.Pos, "missing field `%s` in initializer of struct `%s`", name, st.decl.Name)
		}
	}
	if len(args) > 0 {
		return appliedOf(st.decl.Name, args)
	}
	return &types.Named{Name: st.decl.Name}
}

func appliedOf(name string, args []types.Type) types.Type {
	return &types.Applied{Base: &types.Named{Name: name}, Args: args}
}

func (c *Checker) checkArrayLit(e *ast.ArrayLit, env *environment, loans *borrowCtx, path string) types.Type {
	if len(e.Elems) == 0 {
		c.errorf(e.Pos, "cannot infer type of empty array")
		return &types.Error{}
	}
	elemTy := c.checkExpr(e.Elems[0], env, loans, path)
	for _, elem := range e.Elems[1:] {
		ty := c.checkExpr(elem, env, loans, path)
		if !ty.Equals(elemTy) && !isError(ty) {
			c.errorf(PosOf(elem), "expected `%s`, found `%s`", elemTy, ty)
		}
	}
	return &types.Array{Elem: elemTy, Size: len(e.Elems)}
}

func (c *Checker) resolveType(t ast.Type, path string, genParams []string) types.Type {
	if t == nil {
		return types.Unit
	}
	switch ty := t.(type) {
	case *ast.NamedType:
		if ty.Name == "Self" {
			for _, p := range genParams {
				if p == "Self" {
					return &types.Generic{Name: "Self"}
				}
			}
			c.errorf(ty.Pos, "`Self` is only valid inside a trait or impl")
			return &types.Error{}
		}
		return &types.Named{Name: ty.Name}
	case *ast.GenericType:
		base := c.resolveType(ty.Base, path, genParams)
		var args []types.Type
		for _, a := range ty.Args {
			args = append(args, c.resolveType(a, path, genParams))
		}
		return &types.Applied{Base: base, Args: args}
	case *ast.RefType:
		return &types.Ref{Elem: c.resolveType(ty.Elem, path, genParams), IsMut: ty.IsMut}
	case *ast.ArrayType:
		return &types.Array{Elem: c.resolveType(ty.Elem, path, genParams), Size: ty.Size}
	default:
		return &types.Error{}
	}
}

func (c *Checker) errorf(pos ast.Pos, format string, args ...interface{}) {
	c.Reporter.Add(diag.New(c.currentPath, pos, fmt.Sprintf(format, args...)))
}

func PosOf(n ast.Node) ast.Pos {
	if p, ok := n.(interface{ GetPos() ast.Pos }); ok {
		return p.GetPos()
	}
	return 0
}

func isError(t types.Type) bool {
	_, ok := t.(*types.Error)
	return ok
}

func (c *Checker) checkBlock(block *ast.BlockExpr, env *environment, loans *borrowCtx, ret types.Type, path string) types.Type {
	local := newEnv(env)
	localLoans := newBorrowCtx(loans)
	for _, s := range block.Stmts {
		c.checkStmt(s, local, localLoans, ret, path)
	}
	if block.Result != nil {
		if lit, ok := block.Result.(*ast.StructLit); ok {
			if applied, ok := ret.(*types.Applied); ok {
				return c.checkStructLitWithAnnotation(lit, applied, local, localLoans, path)
			}
		}
		return c.checkExpr(block.Result, local, localLoans, path)
	}
	return types.Unit
}

func (c *Checker) checkStmt(s ast.Stmt, env *environment, loans *borrowCtx, ret types.Type, path string) {
	switch st := s.(type) {
	case *ast.LetStmt:
		var annot types.Type
		if st.Ty != nil {
			annot = c.resolveType(st.Ty, path, nil)
		}
		var valTy types.Type
		if lit, ok := st.Value.(*ast.StructLit); ok && annot != nil {
			valTy = c.checkStructLitWithAnnotation(lit, annot, env, loans, path)
		} else {
			valTy = c.checkExpr(st.Value, env, loans, path)
			if loans != nil {
				c.move(loans, st.Value, valTy)
			}
		}
		if annot != nil {
			if !annot.Equals(valTy) && !isError(valTy) {
				c.errorf(PosOf(st.Value), "expected `%s`, found `%s`", annot, valTy)
			}
			env.set(st.Name, annot, st.IsMut)
		} else {
			env.set(st.Name, valTy, st.IsMut)
		}
	case *ast.AssignStmt:
		c.checkAssign(st, env, loans, path)
	case *ast.ReturnStmt:
		var ty types.Type = types.Unit
		if st.Expr != nil {
			if lit, ok := st.Expr.(*ast.StructLit); ok {
				if applied, ok := ret.(*types.Applied); ok {
					ty = c.checkStructLitWithAnnotation(lit, applied, env, loans, path)
				} else {
					ty = c.checkExpr(st.Expr, env, loans, path)
				}
			} else {
				ty = c.checkExpr(st.Expr, env, loans, path)
			}
		}
		if !ty.Equals(ret) && !isError(ty) {
			c.errorf(st.Pos, "expected `%s`, found `%s`", ret, ty)
		}
	case *ast.WhileStmt:
		cond := c.checkExpr(st.Cond, env, loans, path)
		if !cond.Equals(types.Bool) && !isError(cond) {
			c.errorf(PosOf(st.Cond), "expected `bool`, found `%s`", cond)
		}
		c.checkBlock(st.Body, env, loans, ret, path)
	case *ast.ExprStmt:
		c.checkExpr(st.Expr, env, loans, path)
	}
}

func (c *Checker) checkAssign(st *ast.AssignStmt, env *environment, loans *borrowCtx, path string) {
	rightTy := c.checkExpr(st.Right, env, loans, path)
	leftTy := c.checkExprNoBorrow(st.Left, env, path)
	if !leftTy.Equals(rightTy) && !isError(leftTy) && !isError(rightTy) {
		c.errorf(PosOf(st.Right), "expected `%s`, found `%s`", leftTy, rightTy)
	}
	c.useWrite(loans, st.Left, leftTy)
	c.move(loans, st.Right, rightTy)
}

func (c *Checker) checkExprNoBorrow(expr ast.Expr, env *environment, path string) types.Type {
	return c.checkExpr(expr, env, nil, path)
}

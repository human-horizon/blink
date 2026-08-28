package checker

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/types"
)

// CGen generates C code from a successfully type-checked program.
type CGen struct {
	c         *Checker
	out       strings.Builder
	indent    int
	monoInsts map[string][][]types.Type
	tupleDefs map[string]*types.Tuple
}

// GenerateC returns a complete C translation unit for the checked program.
func (c *Checker) GenerateC() string {
	g := &CGen{
		c:         c,
		monoInsts: make(map[string][][]types.Type),
		tupleDefs: make(map[string]*types.Tuple),
	}
	return g.generate()
}

func (g *CGen) generate() string {
	g.out.WriteString("#include <stdint.h>\n")
	g.out.WriteString("#include <stddef.h>\n")
	g.out.WriteString("#include <string.h>\n\n")

	g.collectTupleTypes()
	g.collectMonomorphizations()
	g.emitForwardDecls()
	g.emitStructDefs()
	g.emitEnumConstants()
	g.emitTupleDefs()
	g.emitConstants()
	g.emitGlobalVars()
	g.emitFunctions()
	g.emitMain()

	return g.out.String()
}

func (g *CGen) emitForwardDecls() {
	for key, info := range g.c.structs {
		name := g.mangleName(key, info.decl.GenParams)
		g.writeln("struct ", name, ";")
	}
	for key, info := range g.c.fns {
		if len(info.genParams) == 0 {
			g.emitMethodPrototype(g.mangleName(key, nil), info, info.paramTypes, info.ret)
			continue
		}
		for _, args := range g.monoInsts[key] {
			params, ret := specializeFn(info, args)
			g.emitMethodPrototype(g.monoName(key, args), info, params, ret)
		}
	}
	for typeName, methods := range g.c.inherent {
		for _, info := range methods {
			g.emitMethodPrototype(typeName+"__"+info.decl.Name, info, info.paramTypes, info.ret)
		}
	}
	for _, byType := range g.c.traitImpls {
		for typeName, impl := range byType {
			for _, info := range impl.methods {
				g.emitMethodPrototype(typeName+"__"+info.decl.Name, info, info.paramTypes, info.ret)
			}
		}
	}
	if len(g.c.structs) > 0 || len(g.c.fns) > 0 || len(g.c.inherent) > 0 || len(g.c.traitImpls) > 0 {
		g.writeln("")
	}
}

func (g *CGen) emitMethodPrototype(name string, info *fnInfo, paramTypes []types.Type, ret types.Type) {
	g.write(g.cType(ret), " ", name, "(")
	for i := range paramTypes {
		if i > 0 {
			g.write(", ")
		}
		g.write(g.cType(paramTypes[i]))
	}
	g.writeln(");")
}

func (g *CGen) emitStructDefs() {
	for key, info := range g.c.structs {
		name := g.mangleName(key, info.decl.GenParams)
		g.writeln("struct ", name, " {")
		g.indent++
		for _, f := range info.decl.Fields {
			fieldType := info.fields[f.Name]
			if fieldType == nil {
				fieldType = g.resolveType(f.Ty)
			}
			g.writei(g.cType(fieldType), " ", f.Name, ";")
			g.newline()
		}
		g.indent--
		g.writeln("};")
		g.writeln("")
	}
}

func (g *CGen) emitEnumConstants() {
	for key, info := range g.c.enums {
		name := g.mangleName(key, nil)
		for i, v := range info.decl.Variants {
			g.writeln("#define ", name, "__", v.Name, " ", strconv.Itoa(i))
		}
	}
	if len(g.c.enums) > 0 {
		g.writeln("")
	}
}

func (g *CGen) emitTupleDefs() {
	for _, tuple := range g.tupleDefs {
		name := g.tupleTypeName(tuple)
		g.writeln("struct ", name, " {")
		g.indent++
		for i, e := range tuple.Elems {
			g.writei(g.cType(e), " f", strconv.Itoa(i), ";")
			g.newline()
		}
		g.indent--
		g.writeln("};")
		g.writeln("")
	}
}

func (g *CGen) emitConstants() {
	for key, info := range g.c.consts {
		val := g.expr(info.decl.Value)
		g.writeln("#define ", g.mangleName(key, nil), " (", val, ")")
	}
	if len(g.c.consts) > 0 {
		g.writeln("")
	}
}

func (g *CGen) emitGlobalVars() {
	for key, info := range g.c.globals {
		ty := g.cType(info.ty)
		name := g.mangleName(key, nil)
		g.write(ty, " ", name)
		if info.decl.Value != nil {
			g.write(" = ", g.expr(info.decl.Value))
		}
		g.writeln(";")
	}
	if len(g.c.globals) > 0 {
		g.writeln("")
	}
}

func (g *CGen) emitFunctions() {
	for key, info := range g.c.fns {
		if len(info.genParams) == 0 {
			g.emitFn(key, info, info.paramTypes, info.ret)
			continue
		}
		for _, args := range g.monoInsts[key] {
			params, ret := specializeFn(info, args)
			g.emitFnBody(g.monoName(key, args), info.decl.Params, params, ret, info.decl.Body)
		}
	}
	for typeName, methods := range g.c.inherent {
		for _, info := range methods {
			g.emitMethod(typeName, info, info.paramTypes, info.ret)
		}
	}
	for _, byType := range g.c.traitImpls {
		for typeName, impl := range byType {
			for _, info := range impl.methods {
				g.emitMethod(typeName, info, info.paramTypes, info.ret)
			}
		}
	}
}

func (g *CGen) emitFn(key string, info *fnInfo, paramTypes []types.Type, ret types.Type) {
	name := g.mangleName(key, info.decl.GenParams)
	g.emitFnBody(name, info.decl.Params, paramTypes, ret, info.decl.Body)
}

func (g *CGen) emitMethod(typeName string, info *fnInfo, paramTypes []types.Type, ret types.Type) {
	name := typeName + "__" + info.decl.Name
	g.emitFnBody(name, info.decl.Params, paramTypes, ret, info.decl.Body)
}

func (g *CGen) emitFnBody(name string, params []ast.Param, paramTypes []types.Type, ret types.Type, body *ast.BlockExpr) {
	retTy := g.cType(ret)
	g.write(retTy, " ", name, "(")
	for i, p := range params {
		if i > 0 {
			g.write(", ")
		}
		ty := paramTypes[i]
		g.write(g.cType(ty), " ", g.ident(p.Name))
	}
	g.writeln(") {")
	g.indent++
	g.emitBlock(body, ret)
	g.indent--
	g.writeln("}")
	g.writeln("")
}

func (g *CGen) emitMain() {
	_, ok := g.c.fns["main"]
	if !ok {
		return
	}
	g.writeln("int main(void) {")
	g.indent++
	g.writeln("return (int)", g.mangleName("main", nil), "();")
	g.indent--
	g.writeln("}")
}

func (g *CGen) emitBlock(b *ast.BlockExpr, ret types.Type) {
	for _, s := range b.Stmts {
		g.stmt(s)
	}
	if b.Result != nil {
		g.writei("return ", g.expr(b.Result), ";")
		g.newline()
	} else if ret != nil {
		g.writei("return 0;")
		g.newline()
	}
}

func (g *CGen) stmt(s ast.Stmt) {
	switch st := s.(type) {
	case *ast.LetStmt:
		g.letStmt(st)
	case *ast.AssignStmt:
		g.writei(g.expr(st.Left), " = ", g.expr(st.Right), ";")
		g.newline()
	case *ast.ReturnStmt:
		if st.Expr != nil {
			g.writei("return ", g.expr(st.Expr), ";")
		} else {
			g.writei("return;")
		}
		g.newline()
	case *ast.ExprStmt:
		g.writei(g.expr(st.Expr), ";")
		g.newline()
	case *ast.WhileStmt:
		g.writei("while (", g.expr(st.Cond), ") {")
		g.newline()
		g.indent++
		g.emitBlock(st.Body, nil)
		g.indent--
		g.writei("}")
		g.newline()
	default:
		g.writei("/* unsupported stmt */;")
		g.newline()
	}
}

func (g *CGen) letStmt(st *ast.LetStmt) {
	valTy := g.c.ExprType(st.Value)
	ty := g.cType(g.resolveType(st.Ty))
	if st.Ty == nil && valTy != nil {
		ty = g.cType(valTy)
	}
	val := g.expr(st.Value)
	pat := st.Pattern
	if pat == nil {
		pat = &ast.PatIdent{Pos: st.Pos, Name: st.Name}
	}
	switch p := pat.(type) {
	case *ast.PatIdent:
		if p.Name == "_" {
			g.writei(val, ";")
			g.newline()
			return
		}
		g.writei(ty, " ", g.ident(p.Name), " = ", val, ";")
		g.newline()
	case *ast.PatTuple:
		tmp := g.fresh("tuple")
		g.writei(g.cType(valTy), " ", tmp, " = ", val, ";")
		g.newline()
		for i, elem := range p.Elements {
			if id, ok := elem.(*ast.PatIdent); ok && id.Name != "_" {
				fieldTy := g.cType(g.tupleFieldType(valTy, i))
				g.writei(fieldTy, " ", g.ident(id.Name), " = ", tmp, ".f", strconv.Itoa(i), ";")
				g.newline()
			}
		}
	case *ast.PatStruct:
		tmp := g.fresh("struct")
		g.writei(g.cType(valTy), " ", tmp, " = ", val, ";")
		g.newline()
		for _, f := range p.Fields {
			if f.BindName == "_" {
				continue
			}
			name := f.BindName
			if name == "" {
				name = f.Field
			}
			g.writei(g.cType(g.fieldType(valTy, f.Field)), " ", g.ident(name), " = ", tmp, ".", f.Field, ";")
			g.newline()
		}
	default:
		g.writei(ty, " ", g.ident(st.Name), " = ", val, ";")
		g.newline()
	}
}

func (g *CGen) expr(e ast.Expr) string {
	if e == nil {
		return "0"
	}
	switch ex := e.(type) {
	case *ast.IntLit:
		return strconv.FormatInt(ex.Val, 10)
	case *ast.BoolLit:
		if ex.Val {
			return "1"
		}
		return "0"
	case *ast.StringLit:
		return strconv.Quote(ex.Val)
	case *ast.Ident:
		if _, ok := g.c.structs[ex.Name]; ok {
			return "((struct " + g.mangleName(ex.Name, nil) + "){ })"
		}
		return g.ident(ex.Name)
	case *ast.PathExpr:
		return g.pathExpr(ex)
	case *ast.BinaryExpr:
		return fmt.Sprintf("(%s %s %s)", g.expr(ex.Left), g.binOp(ex.Op), g.expr(ex.Right))
	case *ast.UnaryExpr:
		return fmt.Sprintf("(%s%s)", g.unaryOp(ex.Op), g.expr(ex.Operand))
	case *ast.CallExpr:
		return g.callExpr(ex)
	case *ast.BlockExpr:
		return g.blockExpr(ex)
	case *ast.IfExpr:
		return g.ifExpr(ex)
	case *ast.FieldExpr:
		return g.fieldExpr(ex)
	case *ast.IndexExpr:
		return fmt.Sprintf("(%s[%s])", g.expr(ex.Expr), g.expr(ex.Index))
	case *ast.StructLit:
		return g.structLit(ex)
	case *ast.ArrayLit:
		return g.arrayLit(ex)
	case *ast.TupleExpr:
		return g.tupleExpr(ex)
	case *ast.UnsafeBlockExpr:
		return g.blockExpr(ex.Body)
	case *ast.MacroCallExpr:
		return g.macroCallExpr(ex)
	default:
		return "0 /* unsupported expr */"
	}
}

func (g *CGen) binOp(op string) string {
	switch op {
	case "&&", "||", "==", "!=", "<=", ">=":
		return op
	default:
		return op
	}
}

func (g *CGen) unaryOp(op string) string {
	switch op {
	case "&", "&mut":
		return "&"
	case "*":
		return "*"
	case "!":
		return "!"
	default:
		return op
	}
}

func (g *CGen) callExpr(e *ast.CallExpr) string {
	var recv string
	var methodName string
	if field, ok := e.Func.(*ast.FieldExpr); ok {
		recv = g.expr(field.Expr)
		methodName = field.Field
		typeName := g.typeName(g.c.ExprType(field.Expr))
		// strip pointer for dispatch
		if ref, ok := g.c.ExprType(field.Expr).(*types.Ref); ok {
			typeName = g.typeName(ref.Elem)
		}
		fnName := typeName + "__" + methodName
		var args []string
		if _, ok := g.c.ExprType(field.Expr).(*types.Ref); ok {
			args = append(args, recv)
		} else {
			args = append(args, "&("+recv+")")
		}
		for _, a := range e.Args {
			args = append(args, g.expr(a))
		}
		return fnName + "(" + strings.Join(args, ", ") + ")"
	}
	fn := g.expr(e.Func)
	key := callKey(e.Func)
	if info, ok := g.c.fns[key]; ok && len(info.genParams) > 0 {
		argTypes := make([]types.Type, len(e.Args))
		for i, arg := range e.Args {
			argTypes[i] = g.c.ExprType(arg)
			if argTypes[i] == nil {
				argTypes[i] = types.I32
			}
		}
		fn = g.monoName(key, argTypes)
	}
	var args []string
	for _, a := range e.Args {
		args = append(args, g.expr(a))
	}
	return fn + "(" + strings.Join(args, ", ") + ")"
}

func (g *CGen) blockExpr(e *ast.BlockExpr) string {
	if e == nil {
		return "0"
	}
	var b strings.Builder
	b.WriteString("({ ")
	for _, s := range e.Stmts {
		b.WriteString(g.stmtString(s))
		b.WriteString(" ")
	}
	if e.Result != nil {
		b.WriteString(g.expr(e.Result))
		b.WriteString(";")
	} else {
		b.WriteString("0;")
	}
	b.WriteString(" })")
	return b.String()
}

func (g *CGen) stmtString(s ast.Stmt) string {
	var b strings.Builder
	old := g.out
	g.out = b
	g.stmt(s)
	g.out = old
	return b.String()
}

func (g *CGen) ifExpr(e *ast.IfExpr) string {
	elseExpr := "0"
	if e.ElseBlock != nil {
		elseExpr = g.blockExpr(e.ElseBlock)
	}
	return fmt.Sprintf("((%s) ? %s : %s)", g.expr(e.Cond), g.blockExpr(e.ThenBlock), elseExpr)
}

func (g *CGen) fieldExpr(e *ast.FieldExpr) string {
	expr := g.expr(e.Expr)
	sep := "."
	if _, ok := g.c.ExprType(e.Expr).(*types.Ref); ok {
		sep = "->"
	}
	// tuple field access uses fN
	field := e.Field
	if _, ok := g.c.ExprType(e.Expr).(*types.Tuple); ok {
		return fmt.Sprintf("(%s%s%s)", expr, sep, "f"+field)
	}
	return fmt.Sprintf("(%s%s%s)", expr, sep, field)
}

func (g *CGen) structLit(e *ast.StructLit) string {
	name := g.mangleName(e.Name, nil)
	var init strings.Builder
	init.WriteString("((struct ")
	init.WriteString(name)
	init.WriteString("){ ")
	for i, f := range e.Fields {
		if i > 0 {
			init.WriteString(", ")
		}
		init.WriteString(".")
		init.WriteString(f.Name)
		init.WriteString(" = ")
		init.WriteString(g.expr(f.Value))
	}
	init.WriteString(" })")
	return init.String()
}

func (g *CGen) arrayLit(e *ast.ArrayLit) string {
	var elems []string
	for _, el := range e.Elems {
		elems = append(elems, g.expr(el))
	}
	return "{ " + strings.Join(elems, ", ") + " }"
}

func (g *CGen) tupleExpr(e *ast.TupleExpr) string {
	ty := g.c.ExprType(e)
	name := g.tupleTypeName(ty)
	var init strings.Builder
	init.WriteString("((struct ")
	init.WriteString(name)
	init.WriteString("){ ")
	for i, el := range e.Elements {
		if i > 0 {
			init.WriteString(", ")
		}
		init.WriteString(".f")
		init.WriteString(strconv.Itoa(i))
		init.WriteString(" = ")
		init.WriteString(g.expr(el))
	}
	init.WriteString(" })")
	return init.String()
}

func (g *CGen) macroCallExpr(e *ast.MacroCallExpr) string {
	m, ok := g.c.macros[e.Name]
	if !ok {
		return "0 /* unknown macro */"
	}
	return g.expr(m.Body)
}

func (g *CGen) pathExpr(e *ast.PathExpr) string {
	key := strings.Join(e.Segments, "::")
	return g.mangleName(key, nil)
}

func (g *CGen) cType(t types.Type) string {
	if t == nil {
		return "int"
	}
	switch ty := t.(type) {
	case *types.Builtin:
		switch ty.Name {
		case "i32":
			return "int32_t"
		case "bool":
			return "int"
		case "()":
			return "int"
		case "String":
			return "const char*"
		default:
			return ty.Name
		}
	case *types.Named:
		if _, ok := g.c.enums[ty.Name]; ok {
			return "int"
		}
		return "struct " + g.mangleName(ty.Name, nil)
	case *types.Ref:
		return g.cType(ty.Elem) + "*"
	case *types.Array:
		return g.cType(ty.Elem) + "[" + strconv.FormatInt(ty.Len, 10) + "]"
	case *types.Tuple:
		return "struct " + g.tupleTypeName(ty)
	case *types.Applied:
		return g.cType(ty.Base)
	case *types.Generic:
		return "int /* generic */"
	default:
		return "int"
	}
}

func (g *CGen) mangleName(key string, genParams []string) string {
	if key == "main" {
		return "blink_main"
	}
	parts := strings.Split(key, "::")
	base := parts[len(parts)-1]
	if len(parts) > 1 {
		prefix := strings.Join(parts[:len(parts)-1], "__")
		return prefix + "__" + base
	}
	return base
}

func (g *CGen) ident(name string) string {
	if name == "main" {
		return "blink_main"
	}
	return name
}

func (g *CGen) typeName(t types.Type) string {
	switch ty := t.(type) {
	case *types.Named:
		return ty.Name
	case *types.Ref:
		return g.typeName(ty.Elem)
	case *types.Applied:
		return g.typeName(ty.Base)
	default:
		return ""
	}
}

func (g *CGen) tupleTypeName(t types.Type) string {
	ty, ok := t.(*types.Tuple)
	if !ok {
		return "tuple"
	}
	g.tupleDefs[ty.String()] = ty
	var parts []string
	for _, e := range ty.Elems {
		parts = append(parts, g.cType(e))
	}
	name := "tuple_" + strings.Join(parts, "_")
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "*", "ptr")
	return name
}

func (g *CGen) tupleFieldType(t types.Type, i int) types.Type {
	ty, ok := t.(*types.Tuple)
	if !ok || i >= len(ty.Elems) {
		return types.I32
	}
	return ty.Elems[i]
}

func (g *CGen) fieldType(t types.Type, field string) types.Type {
	name, ok := t.(*types.Named)
	if !ok {
		if ref, ok := t.(*types.Ref); ok {
			name, ok = ref.Elem.(*types.Named)
		}
	}
	if !ok {
		return types.I32
	}
	info, ok := g.c.structs[name.Name]
	if !ok {
		return types.I32
	}
	fty, ok := info.fields[field]
	if !ok {
		return types.I32
	}
	return fty
}

func (g *CGen) resolveType(t ast.Type) types.Type {
	if t == nil {
		return nil
	}
	switch ty := t.(type) {
	case *ast.NamedType:
		if ty.Name == "i32" {
			return types.I32
		}
		if ty.Name == "bool" {
			return types.Bool
		}
		return &types.Named{Name: ty.Name}
	case *ast.RefType:
		return &types.Ref{Elem: g.resolveType(ty.Elem), IsMut: ty.IsMut, Lifetime: ty.Lifetime}
	case *ast.TupleType:
		var elems []types.Type
		for _, e := range ty.ElementTypes {
			elems = append(elems, g.resolveType(e))
		}
		return &types.Tuple{Elems: elems}
	case *ast.ArrayType:
		return &types.Array{Elem: g.resolveType(ty.Elem), Len: ty.Len}
	default:
		return nil
	}
}

func (g *CGen) write(parts ...string) {
	for _, p := range parts {
		g.out.WriteString(p)
	}
}

func (g *CGen) writeln(parts ...string) {
	g.write(parts...)
	g.newline()
}

func (g *CGen) writei(parts ...string) {
	for i := 0; i < g.indent; i++ {
		g.out.WriteString("\t")
	}
	g.write(parts...)
}

func (g *CGen) newline() {
	g.out.WriteString("\n")
}

var freshCounter int

func (g *CGen) fresh(prefix string) string {
	freshCounter++
	return prefix + "_" + strconv.Itoa(freshCounter)
}

func (g *CGen) collectTupleTypes() {
	for _, file := range g.c.Files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FnDecl:
				for _, p := range d.Params {
					g.collectType(p.Ty)
				}
				g.collectType(d.Ret)
				g.collectBlock(d.Body)
			case *ast.StructDecl:
				for _, f := range d.Fields {
					g.collectType(f.Ty)
				}
			case *ast.ConstDecl:
				g.collectType(d.Ty)
				g.collectExpr(d.Value)
			case *ast.StaticDecl:
				g.collectType(d.Ty)
				g.collectExpr(d.Value)
			case *ast.ImplDecl:
				g.collectType(d.ForType)
				for _, m := range d.Methods {
					for _, p := range m.Params {
						g.collectType(p.Ty)
					}
					g.collectType(m.Ret)
					g.collectBlock(m.Body)
				}
			}
		}
	}
}

func (g *CGen) collectType(t ast.Type) {
	switch ty := t.(type) {
	case *ast.TupleType:
		resolved := g.resolveType(ty)
		g.tupleTypeName(resolved)
		for _, elem := range ty.ElementTypes {
			g.collectType(elem)
		}
	case *ast.RefType:
		g.collectType(ty.Elem)
	case *ast.ArrayType:
		g.collectType(ty.Elem)
	}
}

func (g *CGen) collectBlock(block *ast.BlockExpr) {
	if block == nil {
		return
	}
	for _, stmt := range block.Stmts {
		switch s := stmt.(type) {
		case *ast.LetStmt:
			g.collectType(s.Ty)
			g.collectExpr(s.Value)
		case *ast.AssignStmt:
			g.collectExpr(s.Left)
			g.collectExpr(s.Right)
		case *ast.ReturnStmt:
			g.collectExpr(s.Expr)
		case *ast.ExprStmt:
			g.collectExpr(s.Expr)
		case *ast.WhileStmt:
			g.collectExpr(s.Cond)
			g.collectBlock(s.Body)
		}
	}
	g.collectExpr(block.Result)
}

func (g *CGen) collectExpr(expr ast.Expr) {
	switch e := expr.(type) {
	case *ast.TupleExpr:
		g.tupleTypeName(g.c.ExprType(e))
		for _, elem := range e.Elements {
			g.collectExpr(elem)
		}
	case *ast.BinaryExpr:
		g.collectExpr(e.Left)
		g.collectExpr(e.Right)
	case *ast.UnaryExpr:
		g.collectExpr(e.Operand)
	case *ast.CallExpr:
		g.collectCall(e)
		g.collectExpr(e.Func)
		for _, arg := range e.Args {
			g.collectExpr(arg)
		}
	case *ast.BlockExpr:
		g.collectBlock(e)
	case *ast.UnsafeBlockExpr:
		g.collectBlock(e.Body)
	case *ast.IfExpr:
		g.collectExpr(e.Cond)
		g.collectBlock(e.ThenBlock)
		g.collectBlock(e.ElseBlock)
	case *ast.FieldExpr:
		g.collectExpr(e.Expr)
	case *ast.IndexExpr:
		g.collectExpr(e.Expr)
		g.collectExpr(e.Index)
	case *ast.StructLit:
		for _, field := range e.Fields {
			g.collectExpr(field.Value)
		}
	case *ast.ArrayLit:
		for _, elem := range e.Elems {
			g.collectExpr(elem)
		}
	case *ast.MacroCallExpr:
		if macro, ok := g.c.macros[e.Name]; ok {
			g.collectExpr(macro.Body)
		}
	}
}

func (g *CGen) collectMonomorphizations() {
	for _, file := range g.c.Files {
		for _, decl := range file.Decls {
			switch d := decl.(type) {
			case *ast.FnDecl:
				g.collectBlock(d.Body)
			case *ast.ConstDecl:
				g.collectExpr(d.Value)
			case *ast.StaticDecl:
				g.collectExpr(d.Value)
			case *ast.ImplDecl:
				for _, method := range d.Methods {
					g.collectBlock(method.Body)
				}
			}
		}
	}
}

func (g *CGen) collectCall(call *ast.CallExpr) {
	key := callKey(call.Func)
	if key == "" {
		return
	}
	info, ok := g.c.fns[key]
	if !ok || len(info.genParams) == 0 {
		return
	}
	args := make([]types.Type, len(call.Args))
	for i, arg := range call.Args {
		args[i] = g.c.ExprType(arg)
		if args[i] == nil {
			args[i] = types.I32
		}
	}
	for _, existing := range g.monoInsts[key] {
		if sameTypes(existing, args) {
			return
		}
	}
	g.monoInsts[key] = append(g.monoInsts[key], args)
}

func callKey(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.PathExpr:
		return strings.Join(e.Segments, "::")
	default:
		return ""
	}
}

func sameTypes(left, right []types.Type) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !left[i].Equals(right[i]) {
			return false
		}
	}
	return true
}

func specializeFn(info *fnInfo, args []types.Type) ([]types.Type, types.Type) {
	mapping := make(map[string]types.Type, len(info.genParams))
	for i, name := range info.genParams {
		if i < len(args) {
			mapping[name] = args[i]
		}
	}
	params := make([]types.Type, len(info.paramTypes))
	for i, param := range info.paramTypes {
		params[i] = types.Substitute(param, mapping, nil)
	}
	return params, types.Substitute(info.ret, mapping, nil)
}

func (g *CGen) monoName(key string, args []types.Type) string {
	name := g.mangleName(key, nil)
	for _, arg := range args {
		name += "_" + typeMangle(arg)
	}
	return name
}

func typeMangle(t types.Type) string {
	name := t.String()
	name = strings.NewReplacer("&", "ref_", " ", "_", "::", "_", "<", "_", ">", "_", ",", "_", "[", "arr_", "]", "", ";", "_", "(", "tuple_", ")", "").Replace(name)
	if name == "" {
		return "unit"
	}
	return name
}

func isUnit(t types.Type) bool {
	b, ok := t.(*types.Builtin)
	return ok && b.Name == "()"
}

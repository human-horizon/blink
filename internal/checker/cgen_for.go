package checker

import (
	"strconv"

	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/types"
)

// C code generation for `for` loops. Supported iterables:
//   - arrays with known length (`for x in arr`, patterns may destructure);
//   - integer ranges (`for i in 0..n`).
// Anything else (Vec, slices, iterators) needs a std runtime and is emitted
// as an unsupported comment for now.

func (g *CGen) forStmt(st *ast.ForStmt) {
	switch iter := st.Iter.(type) {
	case *ast.RangeExpr:
		g.forRange(st, iter)
	case *ast.Ident:
		g.forArray(st, iter)
	default:
		g.writei("/* unsupported for iterable */;")
		g.newline()
	}
}

// forRange renders `for pat in from..to { body }` as a counted C loop.
func (g *CGen) forRange(st *ast.ForStmt, r *ast.RangeExpr) {
	from := "0"
	if r.From != nil {
		from = g.expr(r.From)
	}
	if r.To == nil {
		g.writei("/* unsupported open range in for */;")
		g.newline()
		return
	}
	to := g.expr(r.To)
	name := "__iter"
	if id, ok := st.Pat.(*ast.PatIdent); ok {
		name = g.ident(id.Name)
	}
	g.writei("for (int32_t ", name, " = ", from, "; ", name, " < ", to, "; ", name, "++) {")
	g.newline()
	g.indent++
	g.emitBlock(st.Body, nil)
	g.indent--
	g.writei("}")
	g.newline()
}

// forArray renders `for pat in arr { body }` over a fixed-size C array.
func (g *CGen) forArray(st *ast.ForStmt, ident *ast.Ident) {
	arr, ok := g.c.ExprType(ident).(*types.Array)
	if !ok || arr.LenName != "" || arr.Len <= 0 {
		g.writei("/* unsupported for iterable */;")
		g.newline()
		return
	}
	key := g.fresh("it")
	subject := g.ident(ident.Name) + "[" + key + "]"
	_, decls := g.patCond(st.Pat, subject, arr.Elem)
	g.writei("for (size_t ", key, " = 0; ", key, " < ", strconv.FormatInt(arr.Len, 10), "; ", key, "++) {")
	g.newline()
	g.indent++
	if decls != "" {
		g.write(decls)
		g.newline()
	}
	g.emitBlock(st.Body, nil)
	g.indent--
	g.writei("}")
	g.newline()
}

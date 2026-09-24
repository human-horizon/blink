package checker

import (
	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/types"
)

// borrowCtx tracks ownership and loans for local variables within a scope.
type borrowCtx struct {
	parent  *borrowCtx
	states  map[string]*varState
	holders map[string]*refHolder
}

type varState struct {
	moved       bool
	sharedLoans int
	mutableLoan bool
}

// refHolder records that a local variable holds a reference to target. The
// loan stays active while the holder variable is still live (NLL-style): it is
// re-established after each statement that still has a use of the holder ahead.
type refHolder struct {
	target string
	mut    bool
}

func newBorrowCtx(parent *borrowCtx) *borrowCtx {
	return &borrowCtx{
		parent:  parent,
		states:  make(map[string]*varState),
		holders: make(map[string]*refHolder),
	}
}

func (b *borrowCtx) state(name string) *varState {
	if s, ok := b.states[name]; ok {
		return s
	}
	if b.parent != nil {
		return b.parent.state(name)
	}
	return nil
}

func (b *borrowCtx) getOrCreate(name string) *varState {
	if s, ok := b.states[name]; ok {
		return s
	}
	// Look up in parent and copy current effective state into current scope.
	var base varState
	if b.parent != nil {
		if ps := b.parent.state(name); ps != nil {
			base = *ps
		}
	}
	s := &base
	b.states[name] = s
	return s
}

// holder returns the reference holder registered for name in this scope or any
// enclosing scope, copying it into this scope on the way.
func (b *borrowCtx) holder(name string) *refHolder {
	if h, ok := b.holders[name]; ok {
		return h
	}
	if b.parent != nil {
		if ph := b.parent.holder(name); ph != nil {
			h := *ph
			b.holders[name] = &h
			return b.holders[name]
		}
	}
	return nil
}

func (b *borrowCtx) addHolder(name, target string, mut bool) {
	b.holders[name] = &refHolder{target: target, mut: mut}
}

func (b *borrowCtx) removeHolder(name string) {
	delete(b.holders, name)
}

// endStatement clears statement-level loans and then re-establishes loans for
// reference holders that are still live, i.e. mentioned in one of the
// remaining statements or in the block's tail expression. Holders whose last
// use has passed drop their loan (non-lexical lifetimes).
func (b *borrowCtx) endStatement(remaining []ast.Stmt, tail ast.Expr) {
	for _, s := range b.states {
		s.mutableLoan = false
		s.sharedLoans = 0
	}
	for name, h := range b.holders {
		if !stmtsMention(remaining, tail, name) {
			delete(b.holders, name)
			continue
		}
		s := b.getOrCreate(h.target)
		if h.mut {
			s.mutableLoan = true
		} else {
			s.sharedLoans++
		}
	}
}

func (c *Checker) borrowError(pos ast.Pos, format string, args ...interface{}) {
	c.errorf(pos, format, args...)
}

func rootVar(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.FieldExpr:
		return rootVar(e.Expr)
	case *ast.IndexExpr:
		return rootVar(e.Expr)
	case *ast.UnaryExpr:
		if e.Op == "*" {
			return rootVar(e.Operand)
		}
		return ""
	default:
		return ""
	}
}

func (c *Checker) useRead(ctx *borrowCtx, expr ast.Expr, ty types.Type) {
	if types.IsCopy(ty) {
		return
	}
	name := rootVar(expr)
	if name == "" {
		return
	}
	s := ctx.state(name)
	if s == nil {
		return
	}
	if s.moved {
		c.borrowError(PosOf(expr), "use of moved value `%s`", name)
		return
	}
	if s.mutableLoan {
		c.borrowError(PosOf(expr), "cannot use `%s` because it is mutably borrowed", name)
	}
}

func (c *Checker) useWrite(ctx *borrowCtx, expr ast.Expr, ty types.Type) {
	name := rootVar(expr)
	if name == "" {
		return
	}
	s := ctx.getOrCreate(name)
	if s.mutableLoan {
		c.borrowError(PosOf(expr), "cannot assign to `%s` because it is mutably borrowed", name)
		return
	}
	if s.sharedLoans > 0 {
		c.borrowError(PosOf(expr), "cannot assign to `%s` because it is borrowed", name)
		return
	}
	s.moved = false
}

func (c *Checker) borrowShared(ctx *borrowCtx, expr ast.Expr, ty types.Type) {
	name := rootVar(expr)
	if name == "" {
		return
	}
	s := ctx.getOrCreate(name)
	if s.moved {
		c.borrowError(PosOf(expr), "cannot borrow `%s` as immutable because it is moved", name)
		return
	}
	if s.mutableLoan {
		c.borrowError(PosOf(expr), "cannot borrow `%s` as immutable because it is mutably borrowed", name)
		return
	}
	s.sharedLoans++
}

func (c *Checker) borrowMut(ctx *borrowCtx, env *environment, expr ast.Expr, ty types.Type) {
	name := rootVar(expr)
	if name == "" {
		return
	}
	if !env.isMut(name) {
		c.borrowError(PosOf(expr), "cannot borrow `%s` as mutable, as it is not declared mutable", name)
		return
	}
	s := ctx.getOrCreate(name)
	if s.moved {
		c.borrowError(PosOf(expr), "cannot borrow `%s` as mutable because it is moved", name)
		return
	}
	if s.sharedLoans > 0 {
		c.borrowError(PosOf(expr), "cannot borrow `%s` as mutable because it is also borrowed as immutable", name)
		return
	}
	if s.mutableLoan {
		c.borrowError(PosOf(expr), "cannot borrow `%s` as mutable more than once at a time", name)
		return
	}
	s.mutableLoan = true
}

func (c *Checker) move(ctx *borrowCtx, expr ast.Expr, ty types.Type) {
	if types.IsCopy(ty) {
		return
	}
	name := rootVar(expr)
	if name == "" {
		return
	}
	s := ctx.getOrCreate(name)
	if s.sharedLoans > 0 || s.mutableLoan {
		c.borrowError(PosOf(expr), "cannot move out of `%s` because it is borrowed", name)
		return
	}
	s.moved = true
}

// registerRefHolder records that a freshly bound variable holds a reference,
// so the loan survives statement-level release while the holder stays live.
// The checkUnary loan for the initializer is the single loan; it must not be
// taken twice.
func (c *Checker) registerRefHolder(ctx *borrowCtx, name string, value ast.Expr) {
	if ctx == nil || name == "" || name == "self" {
		return
	}
	u, ok := value.(*ast.UnaryExpr)
	if !ok || (u.Op != "&" && u.Op != "&mut") {
		return
	}
	target := rootVar(u.Operand)
	if target == "" || target == "self" {
		return
	}
	ctx.addHolder(name, target, u.Op == "&mut")
}

// stmtsMention reports whether name is referenced in any of the statements or
// in the tail expression.
func stmtsMention(stmts []ast.Stmt, tail ast.Expr, name string) bool {
	for _, s := range stmts {
		if stmtMentions(s, name) {
			return true
		}
	}
	return tail != nil && exprMentions(tail, name)
}

func stmtMentions(s ast.Stmt, name string) bool {
	switch st := s.(type) {
	case *ast.LetStmt:
		return st.Value != nil && exprMentions(st.Value, name)
	case *ast.AssignStmt:
		return exprMentions(st.Left, name) || exprMentions(st.Right, name)
	case *ast.ExprStmt:
		return exprMentions(st.Expr, name)
	case *ast.ReturnStmt:
		return st.Expr != nil && exprMentions(st.Expr, name)
	case *ast.WhileStmt:
		return exprMentions(st.Cond, name) || blockMentions(st.Body, name)
	case *ast.ForStmt:
		return exprMentions(st.Iter, name) || blockMentions(st.Body, name)
	default:
		// Unknown statement node: be conservative and assume a mention so the
		// loan stays alive (no missed borrow errors).
		return true
	}
}

func blockMentions(b *ast.BlockExpr, name string) bool {
	if b == nil {
		return false
	}
	return stmtsMention(b.Stmts, b.Result, name)
}

func exprMentions(e ast.Expr, name string) bool {
	switch ex := e.(type) {
	case nil:
		return false
	case *ast.Ident:
		return ex.Name == name
	case *ast.IntLit, *ast.BoolLit, *ast.StringLit, *ast.PathExpr, *ast.MacroCallExpr:
		return false
	case *ast.BinaryExpr:
		return exprMentions(ex.Left, name) || exprMentions(ex.Right, name)
	case *ast.UnaryExpr:
		return exprMentions(ex.Operand, name)
	case *ast.CastExpr:
		return exprMentions(ex.Expr, name)
	case *ast.RangeExpr:
		return exprMentions(ex.From, name) || exprMentions(ex.To, name)
	case *ast.CallExpr:
		if exprMentions(ex.Func, name) {
			return true
		}
		for _, a := range ex.Args {
			if exprMentions(a, name) {
				return true
			}
		}
		return false
	case *ast.FieldExpr:
		return exprMentions(ex.Expr, name)
	case *ast.IndexExpr:
		return exprMentions(ex.Expr, name) || exprMentions(ex.Index, name)
	case *ast.TupleExpr:
		for _, el := range ex.Elements {
			if exprMentions(el, name) {
				return true
			}
		}
		return false
	case *ast.ArrayLit:
		for _, el := range ex.Elems {
			if exprMentions(el, name) {
				return true
			}
		}
		return false
	case *ast.StructLit:
		for _, f := range ex.Fields {
			if exprMentions(f.Value, name) {
				return true
			}
		}
		return false
	case *ast.IfExpr:
		return exprMentions(ex.Cond, name) || blockMentions(ex.ThenBlock, name) || blockMentions(ex.ElseBlock, name)
	case *ast.MatchExpr:
		if exprMentions(ex.Scrutinee, name) {
			return true
		}
		for _, arm := range ex.Arms {
			if exprMentions(arm.Body, name) {
				return true
			}
		}
		return false
	case *ast.BlockExpr:
		return blockMentions(ex, name)
	case *ast.UnsafeBlockExpr:
		return blockMentions(ex.Body, name)
	case *ast.ClosureExpr:
		// A closure parameter shadows the outer holder name.
		for _, a := range ex.Args {
			if a == name {
				return false
			}
		}
		return exprMentions(ex.Body, name)
	default:
		// Unknown expression node: be conservative and assume a mention.
		return true
	}
}

package checker

import (
	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/types"
)

// borrowCtx tracks ownership and loans for local variables within a scope.
type borrowCtx struct {
	parent *borrowCtx
	states map[string]*varState
}

type varState struct {
	moved       bool
	sharedLoans int
	mutableLoan bool
}

func newBorrowCtx(parent *borrowCtx) *borrowCtx {
	return &borrowCtx{parent: parent, states: make(map[string]*varState)}
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

// releaseLoans clears all active borrows in this scope. It is called after each
// statement so that a mutable/immutable borrow does not outlive its statement.
func (b *borrowCtx) releaseLoans() {
	for _, s := range b.states {
		s.mutableLoan = false
		s.sharedLoans = 0
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

// reapplyBorrow re-establishes a borrow that was stored into a let binding.
// Statement-level loan release clears borrows after each statement; a reference
// stored in a variable must keep its borrow alive for the variable's lifetime.
func (c *Checker) reapplyBorrow(ctx *borrowCtx, env *environment, expr ast.Expr, ty types.Type) {
	u, ok := expr.(*ast.UnaryExpr)
	if !ok {
		return
	}
	// self is already borrowed by the method receiver; re-borrowing it here
	// would spuriously flag valid sequential &mut self uses.
	if rootVar(u.Operand) == "self" {
		return
	}
	switch u.Op {
	case "&mut":
		c.borrowMut(ctx, env, u.Operand, ty)
	case "&":
		c.borrowShared(ctx, u.Operand, ty)
	}
}

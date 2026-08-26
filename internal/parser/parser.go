package parser

import (
	"fmt"
	"strconv"

	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/lexer"
)

// Parser parses a sequence of tokens into an AST.
type Parser struct {
	l        *lexer.Lexer
	tok      lexer.Token
	err      error
	source   []byte
	errCount int
}

// New creates a new parser from a lexer.
func New(l *lexer.Lexer, src []byte) *Parser {
	p := &Parser{l: l, source: src}
	p.next()
	return p
}

// ParseFile parses a full source file into a File.
func (p *Parser) ParseFile() (*ast.File, error) {
	file := &ast.File{}
	for p.tok.Kind != lexer.EOF {
		if p.err != nil {
			break
		}
		decl := p.parseDecl()
		if decl == nil {
			break
		}
		file.Decls = append(file.Decls, decl)
	}
	if p.err != nil {
		return nil, p.err
	}
	return file, nil
}

// Err returns the first parsing error.
func (p *Parser) Err() error { return p.err }

func (p *Parser) setErr(format string, args ...interface{}) {
	if p.err != nil {
		p.next()
		return
	}
	p.err = fmt.Errorf(format, args...)
	p.errCount++
	p.next()
	if p.errCount > 100 {
		p.err = fmt.Errorf("too many parse errors, aborting")
	}
}

func (p *Parser) next() {
	p.tok = p.l.Next()
}

func (p *Parser) expect(kind lexer.TokenKind) lexer.Token {
	if p.tok.Kind != kind {
		p.setErr("expected %v, got %v", kind, p.tok.Kind)
		return p.tok
	}
	t := p.tok
	p.next()
	return t
}

func (p *Parser) parseDecl() ast.Decl {
	if p.err != nil {
		return nil
	}
	switch p.tok.Kind {
	case lexer.Fn:
		return p.parseFnDecl()
	case lexer.Struct:
		return p.parseStructDecl()
	case lexer.Enum:
		return p.parseEnumDecl()
	default:
		p.setErr("unexpected token %v at top level", p.tok.Kind)
		return nil
	}
}

func (p *Parser) parseFnDecl() ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // fn
	name := p.expect(lexer.Ident)
	p.expect(lexer.LParen)
	var params []ast.Param
	for p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
		paramPos := ast.Pos(p.tok.Pos)
		paramName := p.expect(lexer.Ident)
		p.expect(lexer.Colon)
		ty := p.parseType()
		params = append(params, ast.Param{Pos: paramPos, Name: paramName.Text, Ty: ty})
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.RParen)
	var ret ast.Type
	if p.tok.Kind == lexer.Arrow {
		p.next()
		ret = p.parseType()
	}
	body := p.parseBlock()
	return &ast.FnDecl{Pos: pos, Name: name.Text, Params: params, Ret: ret, Body: body}
}

func (p *Parser) parseStructDecl() ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // struct
	name := p.expect(lexer.Ident)
	p.expect(lexer.LBrace)
	var fields []ast.Field
	for p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		fieldPos := ast.Pos(p.tok.Pos)
		fieldName := p.expect(lexer.Ident)
		p.expect(lexer.Colon)
		ty := p.parseType()
		fields = append(fields, ast.Field{Pos: fieldPos, Name: fieldName.Text, Ty: ty})
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.RBrace)
	return &ast.StructDecl{Pos: pos, Name: name.Text, Fields: fields}
}

func (p *Parser) parseEnumDecl() ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // enum
	name := p.expect(lexer.Ident)
	p.expect(lexer.LBrace)
	var variants []ast.Variant
	for p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		variantPos := ast.Pos(p.tok.Pos)
		variantName := p.expect(lexer.Ident)
		variants = append(variants, ast.Variant{Pos: variantPos, Name: variantName.Text})
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.RBrace)
	return &ast.EnumDecl{Pos: pos, Name: name.Text, Variants: variants}
}

func (p *Parser) parseType() ast.Type {
	if p.err != nil {
		return nil
	}
	switch p.tok.Kind {
	case lexer.Ident:
		pos := ast.Pos(p.tok.Pos)
		name := p.tok.Text
		p.next()
		return &ast.NamedType{Pos: pos, Name: name}
	case lexer.I32, lexer.Bool:
		pos := ast.Pos(p.tok.Pos)
		name := p.tok.Text
		p.next()
		return &ast.NamedType{Pos: pos, Name: name}
	case lexer.And:
		pos := ast.Pos(p.tok.Pos)
		p.next()
		isMut := false
		if p.tok.Kind == lexer.Ident && p.tok.Text == "mut" {
			isMut = true
			p.next()
		}
		elem := p.parseType()
		return &ast.RefType{Pos: pos, Elem: elem, IsMut: isMut}
	case lexer.LBracket:
		pos := ast.Pos(p.tok.Pos)
		p.next() // [
		elem := p.parseType()
		p.expect(lexer.Semi)
		lenTok := p.expect(lexer.IntLit)
		lenVal, _ := strconv.ParseInt(lenTok.Text, 10, 64)
		p.expect(lexer.RBracket)
		return &ast.ArrayType{Pos: pos, Elem: elem, Len: lenVal}
	default:
		p.setErr("expected type, got %v", p.tok.Kind)
		return nil
	}
}

func (p *Parser) parseBlock() *ast.BlockExpr {
	if p.err != nil {
		return &ast.BlockExpr{}
	}
	pos := ast.Pos(p.tok.Pos)
	p.expect(lexer.LBrace)
	var stmts []ast.Stmt
	var result ast.Expr
	for p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		if p.err != nil {
			break
		}
		if p.tok.Kind == lexer.Semi {
			p.next()
			continue
		}
		if isTrailingExprStart(p.tok.Kind) {
			expr := p.parseExpr()
			if p.tok.Kind == lexer.Semi {
				stmts = append(stmts, &ast.ExprStmt{Expr: expr})
				p.next()
			} else {
				result = expr
				if p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
					p.setErr("expected `}` or `;` after expression")
				}
			}
		} else {
			stmts = append(stmts, p.parseStmt())
		}
	}
	p.expect(lexer.RBrace)
	return &ast.BlockExpr{Pos: pos, Stmts: stmts, Result: result}
}

func isTrailingExprStart(kind lexer.TokenKind) bool {
	switch kind {
	case lexer.Ident, lexer.IntLit, lexer.True, lexer.False, lexer.StringLit,
		lexer.LParen, lexer.LBrace, lexer.LBracket, lexer.Minus, lexer.Bang:
		return true
	}
	return false
}

func (p *Parser) parseStmt() ast.Stmt {
	if p.err != nil {
		return nil
	}
	switch p.tok.Kind {
	case lexer.Let:
		return p.parseLetStmt()
	case lexer.Return:
		return p.parseReturnStmt()
	case lexer.While:
		return p.parseWhileStmt()
	case lexer.If:
		return &ast.ExprStmt{Expr: p.parseIfExpr()}
	default:
		expr := p.parseExpr()
		if p.tok.Kind == lexer.Semi {
			p.next()
		}
		return &ast.ExprStmt{Expr: expr}
	}
}

func (p *Parser) parseLetStmt() ast.Stmt {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // let
	name := p.expect(lexer.Ident)
	var ty ast.Type
	if p.tok.Kind == lexer.Colon {
		p.next()
		ty = p.parseType()
	}
	p.expect(lexer.Eq)
	value := p.parseExpr()
	if p.tok.Kind == lexer.Semi {
		p.next()
	}
	return &ast.LetStmt{Pos: pos, Name: name.Text, Ty: ty, Value: value}
}

func (p *Parser) parseReturnStmt() ast.Stmt {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // return
	var expr ast.Expr
	if p.tok.Kind != lexer.Semi && p.tok.Kind != lexer.RBrace {
		expr = p.parseExpr()
	}
	if p.tok.Kind == lexer.Semi {
		p.next()
	}
	return &ast.ReturnStmt{Pos: pos, Expr: expr}
}

func (p *Parser) parseWhileStmt() ast.Stmt {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // while
	cond := p.parseExpr()
	body := p.parseBlock()
	return &ast.WhileStmt{Pos: pos, Cond: cond, Body: body}
}

func (p *Parser) parseExpr() ast.Expr {
	if p.err != nil {
		return nil
	}
	return p.parseOr()
}

func (p *Parser) parseOr() ast.Expr {
	left := p.parseAnd()
	for p.tok.Kind == lexer.Or {
		pos := ast.Pos(p.tok.Pos)
		op := p.tok.Text
		p.next()
		right := p.parseAnd()
		left = &ast.BinaryExpr{Pos: pos, Op: op, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseAnd() ast.Expr {
	left := p.parseEquality()
	for p.tok.Kind == lexer.And {
		pos := ast.Pos(p.tok.Pos)
		op := p.tok.Text
		p.next()
		right := p.parseEquality()
		left = &ast.BinaryExpr{Pos: pos, Op: op, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseEquality() ast.Expr {
	left := p.parseRelational()
	for p.tok.Kind == lexer.EqEq || p.tok.Kind == lexer.NotEq {
		pos := ast.Pos(p.tok.Pos)
		op := p.tok.Text
		p.next()
		right := p.parseRelational()
		left = &ast.BinaryExpr{Pos: pos, Op: op, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseRelational() ast.Expr {
	left := p.parseAdditive()
	for p.tok.Kind == lexer.Lt || p.tok.Kind == lexer.Gt {
		pos := ast.Pos(p.tok.Pos)
		op := p.tok.Text
		p.next()
		right := p.parseAdditive()
		left = &ast.BinaryExpr{Pos: pos, Op: op, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseAdditive() ast.Expr {
	left := p.parseMultiplicative()
	for p.tok.Kind == lexer.Plus || p.tok.Kind == lexer.Minus {
		pos := ast.Pos(p.tok.Pos)
		op := p.tok.Text
		p.next()
		right := p.parseMultiplicative()
		left = &ast.BinaryExpr{Pos: pos, Op: op, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseMultiplicative() ast.Expr {
	left := p.parseUnary()
	for p.tok.Kind == lexer.Star || p.tok.Kind == lexer.Slash {
		pos := ast.Pos(p.tok.Pos)
		op := p.tok.Text
		p.next()
		right := p.parseUnary()
		left = &ast.BinaryExpr{Pos: pos, Op: op, Left: left, Right: right}
	}
	return left
}

func (p *Parser) parseUnary() ast.Expr {
	if p.err != nil {
		return nil
	}
	switch p.tok.Kind {
	case lexer.Minus, lexer.Bang:
		pos := ast.Pos(p.tok.Pos)
		op := p.tok.Text
		p.next()
		operand := p.parseUnary()
		return &ast.UnaryExpr{Pos: pos, Op: op, Operand: operand}
	default:
		return p.parsePrimary()
	}
}

func (p *Parser) parsePrimary() ast.Expr {
	if p.err != nil {
		return nil
	}
	switch p.tok.Kind {
	case lexer.IntLit:
		pos := ast.Pos(p.tok.Pos)
		val, _ := strconv.ParseInt(p.tok.Text, 10, 64)
		p.next()
		return &ast.IntLit{Pos: pos, Val: val}
	case lexer.True:
		pos := ast.Pos(p.tok.Pos)
		p.next()
		return &ast.BoolLit{Pos: pos, Val: true}
	case lexer.False:
		pos := ast.Pos(p.tok.Pos)
		p.next()
		return &ast.BoolLit{Pos: pos, Val: false}
	case lexer.StringLit:
		pos := ast.Pos(p.tok.Pos)
		text := p.tok.Text
		p.next()
		return &ast.StringLit{Pos: pos, Val: text}
	case lexer.Ident:
		pos := ast.Pos(p.tok.Pos)
		name := p.tok.Text
		p.next()
		expr := ast.Expr(&ast.Ident{Pos: pos, Name: name})
		for {
			if p.tok.Kind == lexer.LParen {
				expr = p.parseCall(expr)
			} else if p.tok.Kind == lexer.Dot {
				p.next()
				field := p.expect(lexer.Ident)
				expr = &ast.FieldExpr{Pos: ast.Pos(field.Pos), Expr: expr, Field: field.Text}
			} else if p.tok.Kind == lexer.LBracket {
				p.next()
				idx := p.parseExpr()
				p.expect(lexer.RBracket)
				expr = &ast.IndexExpr{Pos: pos, Expr: expr, Index: idx}
			} else if p.tok.Kind == lexer.LBrace {
				// Disambiguate: lowercase ident followed by `{` is not a struct literal.
				// This allows `if b { ... }` without consuming the block brace.
				if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' {
					expr = p.parseStructLitFromName(pos, name)
				} else {
					break
				}
			} else {
				break
			}
		}
		return expr
	case lexer.LBrace:
		return p.parseBlock()
	case lexer.LBracket:
		return p.parseArrayLit()
	default:
		p.setErr("unexpected token %v in expression", p.tok.Kind)
		return nil
	}
}

func (p *Parser) parseCall(fn ast.Expr) ast.Expr {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.expect(lexer.LParen)
	var args []ast.Expr
	for p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
		args = append(args, p.parseExpr())
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.RParen)
	return &ast.CallExpr{Pos: pos, Func: fn, Args: args}
}

func (p *Parser) parseStructLitFromName(pos ast.Pos, name string) ast.Expr {
	if p.err != nil {
		return nil
	}
	p.expect(lexer.LBrace)
	var fields []ast.FieldInit
	for p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		fieldPos := ast.Pos(p.tok.Pos)
		fieldName := p.expect(lexer.Ident)
		p.expect(lexer.Colon)
		value := p.parseExpr()
		fields = append(fields, ast.FieldInit{Pos: fieldPos, Name: fieldName.Text, Value: value})
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.RBrace)
	return &ast.StructLit{Pos: pos, Name: name, Fields: fields}
}

func (p *Parser) parseIfExpr() ast.Expr {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // if
	cond := p.parseExpr()
	thenBlock := p.parseBlock()
	var elseBlock *ast.BlockExpr
	if p.tok.Kind == lexer.Else {
		p.next()
		elseBlock = p.parseBlock()
	}
	return &ast.IfExpr{Pos: pos, Cond: cond, ThenBlock: thenBlock, ElseBlock: elseBlock}
}

func (p *Parser) parseArrayLit() ast.Expr {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // [
	var elems []ast.Expr
	for p.tok.Kind != lexer.RBracket && p.tok.Kind != lexer.EOF {
		elems = append(elems, p.parseExpr())
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.RBracket)
	return &ast.ArrayLit{Pos: pos, Elems: elems}
}

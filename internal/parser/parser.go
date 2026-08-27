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
	pub := false
	if p.tok.Kind == lexer.Pub {
		pub = true
		p.next()
	}
	switch p.tok.Kind {
	case lexer.Fn:
		return p.parseFnDecl(pub)
	case lexer.Struct:
		return p.parseStructDecl(pub)
	case lexer.Enum:
		return p.parseEnumDecl(pub)
	case lexer.Trait:
		return p.parseTraitDecl(pub)
	case lexer.Impl:
		return p.parseImplDecl()
	case lexer.Mod:
		return p.parseModDecl(pub)
	case lexer.Use:
		return p.parseUseDecl()
	case lexer.Ident:
		if p.tok.Text == "const" {
			return p.parseConstDecl(pub)
		}
		if p.tok.Text == "static" {
			return p.parseStaticDecl(pub)
		}
		fallthrough
	default:
		p.setErr("unexpected token %v at top level", p.tok.Kind)
		return nil
	}
}

func (p *Parser) parseConstDecl(pub bool) ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // const
	name := p.expect(lexer.Ident)
	p.expect(lexer.Colon)
	ty := p.parseType()
	p.expect(lexer.Eq)
	value := p.parseExpr()
	if p.tok.Kind == lexer.Semi {
		p.next()
	}
	return &ast.ConstDecl{Pos: pos, Name: name.Text, Ty: ty, Value: value}
}

func (p *Parser) parseStaticDecl(pub bool) ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // static
	name := p.expect(lexer.Ident)
	p.expect(lexer.Colon)
	ty := p.parseType()
	p.expect(lexer.Eq)
	value := p.parseExpr()
	if p.tok.Kind == lexer.Semi {
		p.next()
	}
	return &ast.StaticDecl{Pos: pos, Name: name.Text, Ty: ty, Value: value}
}

func (p *Parser) parseFnDecl(pub bool) ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // fn
	name := p.expect(lexer.Ident)
	lifeParams, genParams, bounds := p.parseParams()
	p.expect(lexer.LParen)
	var params []ast.Param
	for p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
		params = append(params, p.parseParam())
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
	return &ast.FnDecl{Pos: pos, Pub: pub, Name: name.Text, LifetimeParams: lifeParams, GenParams: genParams, Bounds: bounds, Params: params, Ret: ret, Body: body}
}

func (p *Parser) parseParam() ast.Param {
	paramPos := ast.Pos(p.tok.Pos)
	if p.tok.Kind == lexer.And {
		p.next()
		if (p.tok.Kind == lexer.Ident && p.tok.Text == "self") || p.tok.Kind == lexer.Self {
			p.next()
			return ast.Param{Pos: paramPos, Name: "self", IsSelf: true}
		}
		p.setErr("expected `self` after `&`")
		return ast.Param{Pos: paramPos, Name: "self", IsSelf: true}
	}
	if (p.tok.Kind == lexer.Ident && p.tok.Text == "self") || p.tok.Kind == lexer.Self {
		p.next()
		return ast.Param{Pos: paramPos, Name: "self", IsSelf: true}
	}
	paramName := p.expect(lexer.Ident)
	p.expect(lexer.Colon)
	ty := p.parseType()
	return ast.Param{Pos: paramPos, Name: paramName.Text, Ty: ty}
}

func (p *Parser) parseTraitDecl(pub bool) ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // trait
	name := p.expect(lexer.Ident)
	lifeParams, genParams, bounds := p.parseParams()
	p.expect(lexer.LBrace)
	var methods []*ast.FnDecl
	for p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		decl := p.parseFnSig()
		if decl == nil {
			break
		}
		fn, ok := decl.(*ast.FnDecl)
		if !ok {
			p.setErr("expected method signature")
			break
		}
		p.expect(lexer.Semi)
		methods = append(methods, fn)
	}
	p.expect(lexer.RBrace)
	return &ast.TraitDecl{Pos: pos, Pub: pub, Name: name.Text, LifetimeParams: lifeParams, GenParams: genParams, Bounds: bounds, Methods: methods}
}

func (p *Parser) parseImplDecl() ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // impl
	var genParams []string
	var bounds []ast.Constraint
	if p.tok.Kind == lexer.Lt {
		_, genParams, bounds = p.parseParams()
	}
	firstType := p.parseType()
	var trait string
	var forType ast.Type
	if p.tok.Kind == lexer.For {
		p.next()
		forType = p.parseType()
		if named, ok := firstType.(*ast.NamedType); ok {
			trait = named.Name
		} else {
			p.setErr("expected trait name before `for`")
		}
	} else {
		forType = firstType
	}
	p.expect(lexer.LBrace)
	var methods []*ast.FnDecl
	for p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		decl := p.parseFnDecl(false)
		if decl == nil {
			break
		}
		fn, ok := decl.(*ast.FnDecl)
		if !ok {
			p.setErr("expected method declaration")
			break
		}
		methods = append(methods, fn)
	}
	p.expect(lexer.RBrace)
	return &ast.ImplDecl{Pos: pos, Trait: trait, ForType: forType, GenParams: genParams, Bounds: bounds, Methods: methods}
}

func (p *Parser) parseModDecl(pub bool) ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // mod
	name := p.expect(lexer.Ident)
	if p.tok.Kind == lexer.Semi {
		p.next()
		return &ast.ModDecl{Pos: pos, Pub: pub, Name: name.Text, File: name.Text + ".rs"}
	}
	p.expect(lexer.LBrace)
	inline := &ast.File{}
	for p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		inline.Decls = append(inline.Decls, p.parseDecl())
	}
	p.expect(lexer.RBrace)
	return &ast.ModDecl{Pos: pos, Pub: pub, Name: name.Text, Inline: inline}
}

func (p *Parser) parseUseDecl() ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // use
	var path []string
	first := p.expect(lexer.Ident)
	path = append(path, first.Text)
	for p.tok.Kind == lexer.ColonColon {
		p.next()
		part := p.expect(lexer.Ident)
		path = append(path, part.Text)
	}
	alias := ""
	if p.tok.Kind == lexer.As {
		p.next()
		aliasTok := p.expect(lexer.Ident)
		alias = aliasTok.Text
	}
	p.expect(lexer.Semi)
	return &ast.UseDecl{Pos: pos, Path: path, Alias: alias}
}

func (p *Parser) parseFnSig() ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.expect(lexer.Fn)
	name := p.expect(lexer.Ident)
	lifeParams, genParams, bounds := p.parseParams()
	p.expect(lexer.LParen)
	var params []ast.Param
	for p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
		params = append(params, p.parseParam())
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
	return &ast.FnDecl{Pos: pos, Name: name.Text, LifetimeParams: lifeParams, GenParams: genParams, Bounds: bounds, Params: params, Ret: ret}
}

func (p *Parser) parseStructDecl(pub bool) ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // struct
	name := p.expect(lexer.Ident)
	lifeParams, genParams, bounds := p.parseParams()
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
	return &ast.StructDecl{Pos: pos, Pub: pub, Name: name.Text, LifetimeParams: lifeParams, GenParams: genParams, Bounds: bounds, Fields: fields}
}

func (p *Parser) parseEnumDecl(pub bool) ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // enum
	name := p.expect(lexer.Ident)
	genParams := p.parseGenericParams()
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
	return &ast.EnumDecl{Pos: pos, Pub: pub, Name: name.Text, GenParams: genParams, Variants: variants}
}

func (p *Parser) parseParams() ([]string, []string, []ast.Constraint) {
	if p.tok.Kind != lexer.Lt {
		return nil, nil, nil
	}
	p.next() // <
	var lifetimes []string
	var generics []string
	var bounds []ast.Constraint
	for p.tok.Kind != lexer.Gt && p.tok.Kind != lexer.EOF {
		switch p.tok.Kind {
		case lexer.Lifetime:
			lifetimes = append(lifetimes, p.tok.Text)
			p.next()
		case lexer.Ident:
			name := p.tok.Text
			generics = append(generics, name)
			p.next()
			if p.tok.Kind == lexer.Colon {
				p.next()
				for {
					traitTok := p.expect(lexer.Ident)
					bounds = append(bounds, ast.Constraint{Param: name, Trait: traitTok.Text})
					if p.tok.Kind == lexer.Plus {
						p.next()
					} else {
						break
					}
				}
			}
		default:
			p.setErr("expected lifetime or generic parameter")
		}
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.Gt)
	return lifetimes, generics, bounds
}

func (p *Parser) parseGenericParams() []string {
	_, generics, _ := p.parseParams()
	return generics
}

func (p *Parser) parseTypeArgs() []ast.Type {
	if p.tok.Kind != lexer.Lt {
		return nil
	}
	p.next() // <
	var args []ast.Type
	for p.tok.Kind != lexer.Gt && p.tok.Kind != lexer.EOF {
		args = append(args, p.parseType())
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.Gt)
	return args
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
		var args []ast.Type
		if p.tok.Kind == lexer.Lt {
			args = p.parseTypeArgs()
		}
		return &ast.NamedType{Pos: pos, Name: name, Args: args}
	case lexer.I32, lexer.Bool:
		pos := ast.Pos(p.tok.Pos)
		name := p.tok.Text
		p.next()
		if p.tok.Kind == lexer.Lt {
			p.setErr("primitive type `%s` cannot have generic arguments", name)
			p.parseTypeArgs()
		}
		return &ast.NamedType{Pos: pos, Name: name}
	case lexer.And:
		pos := ast.Pos(p.tok.Pos)
		p.next()
		var lifetime string
		if p.tok.Kind == lexer.Lifetime {
			lifetime = p.tok.Text
			p.next()
		}
		isMut := false
		if p.tok.Kind == lexer.Ident && p.tok.Text == "mut" {
			isMut = true
			p.next()
		}
		elem := p.parseType()
		return &ast.RefType{Pos: pos, Lifetime: lifetime, Elem: elem, IsMut: isMut}
	case lexer.LBracket:
		pos := ast.Pos(p.tok.Pos)
		p.next() // [
		elem := p.parseType()
		p.expect(lexer.Semi)
		lenTok := p.expect(lexer.IntLit)
		lenVal, _ := strconv.ParseInt(lenTok.Text, 10, 64)
		p.expect(lexer.RBracket)
		return &ast.ArrayType{Pos: pos, Elem: elem, Len: lenVal}
	case lexer.LParen:
		pos := ast.Pos(p.tok.Pos)
		p.next() // (
		if p.tok.Kind == lexer.RParen {
			p.next()
			return &ast.NamedType{Pos: pos, Name: "()"}
		}
		first := p.parseType()
		if p.tok.Kind != lexer.Comma {
			p.expect(lexer.RParen)
			return first
		}
		var elems []ast.Type
		elems = append(elems, first)
		for p.tok.Kind == lexer.Comma {
			p.next()
			if p.tok.Kind == lexer.RParen {
				break
			}
			elems = append(elems, p.parseType())
		}
		p.expect(lexer.RParen)
		return &ast.TupleType{Pos: pos, ElementTypes: elems}
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
		stmt, hadSemi := p.parseStmt()
		if stmt == nil {
			break
		}
		if !hadSemi && (p.tok.Kind == lexer.RBrace || p.tok.Kind == lexer.EOF) {
			if es, ok := stmt.(*ast.ExprStmt); ok {
				result = es.Expr
				break
			}
		}
		stmts = append(stmts, stmt)
	}
	p.expect(lexer.RBrace)
	return &ast.BlockExpr{Pos: pos, Stmts: stmts, Result: result}
}

// parseStmt parses a statement. The returned bool is true if a semicolon was consumed.
func (p *Parser) parseStmt() (ast.Stmt, bool) {
	if p.err != nil {
		return nil, false
	}
	switch p.tok.Kind {
	case lexer.Let:
		return p.parseLetStmt(), true
	case lexer.Return:
		return p.parseReturnStmt(), true
	case lexer.While:
		return p.parseWhileStmt(), true
	case lexer.If:
		stmt := &ast.ExprStmt{Expr: p.parseIfExpr()}
		if p.tok.Kind == lexer.Semi {
			p.next()
			return stmt, true
		}
		return stmt, false
	default:
		expr := p.parseExpr()
		if p.tok.Kind == lexer.Eq {
			pos := ast.Pos(p.tok.Pos)
			p.next()
			right := p.parseExpr()
			if p.tok.Kind == lexer.Semi {
				p.next()
				return &ast.AssignStmt{Pos: pos, Left: expr, Right: right}, true
			}
			return &ast.AssignStmt{Pos: pos, Left: expr, Right: right}, false
		}
		if p.tok.Kind == lexer.Semi {
			p.next()
			return &ast.ExprStmt{Expr: expr}, true
		}
		return &ast.ExprStmt{Expr: expr}, false
	}
}

func (p *Parser) parseLetStmt() ast.Stmt {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // let
	isMut := false
	if p.tok.Kind == lexer.Ident && p.tok.Text == "mut" {
		isMut = true
		p.next()
	}
	pat := p.parsePattern()
	var name string
	if ident, ok := pat.(*ast.PatIdent); ok {
		name = ident.Name
	}
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
	return &ast.LetStmt{Pos: pos, Name: name, Pattern: pat, IsMut: isMut, Ty: ty, Value: value}
}

func (p *Parser) parsePattern() ast.Pattern {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	if p.tok.Kind == lexer.Ident {
		name := p.tok.Text
		p.next()
		if name == "_" {
			return &ast.PatWildcard{Pos: pos}
		}
		if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' && p.tok.Kind == lexer.LBrace {
			return p.parseStructPattern(pos, name)
		}
		return &ast.PatIdent{Pos: pos, Name: name}
	}
	if p.tok.Kind == lexer.LParen {
		p.next()
		var elems []ast.Pattern
		for p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
			elems = append(elems, p.parsePattern())
			if p.tok.Kind == lexer.Comma {
				p.next()
			}
		}
		p.expect(lexer.RParen)
		return &ast.PatTuple{Pos: pos, Elements: elems}
	}
	p.setErr("expected pattern")
	return nil
}

func (p *Parser) parseStructPattern(pos ast.Pos, name string) ast.Pattern {
	p.expect(lexer.LBrace)
	var fields []ast.PatField
	for p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		fieldPos := ast.Pos(p.tok.Pos)
		fieldName := p.expect(lexer.Ident)
		bindName := ""
		if p.tok.Kind == lexer.Colon {
			p.next()
			bindTok := p.expect(lexer.Ident)
			bindName = bindTok.Text
		}
		fields = append(fields, ast.PatField{Pos: fieldPos, Field: fieldName.Text, BindName: bindName})
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.RBrace)
	return &ast.PatStruct{Pos: pos, Name: name, Fields: fields}
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
	case lexer.Minus, lexer.Bang, lexer.Star:
		pos := ast.Pos(p.tok.Pos)
		op := p.tok.Text
		p.next()
		operand := p.parseUnary()
		return &ast.UnaryExpr{Pos: pos, Op: op, Operand: operand}
	case lexer.And:
		pos := ast.Pos(p.tok.Pos)
		op := "&"
		p.next()
		if p.tok.Kind == lexer.Ident && p.tok.Text == "mut" {
			op = "&mut"
			p.next()
		}
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
	case lexer.Ident, lexer.Self:
		pos := ast.Pos(p.tok.Pos)
		name := p.tok.Text
		if p.tok.Kind == lexer.Self {
			name = "self"
		}
		p.next()
		expr := p.parsePathOrExpr(pos, name)
		for {
			if p.tok.Kind == lexer.LParen {
				expr = p.parseCall(expr)
			} else if p.tok.Kind == lexer.Dot {
				p.next()
				if p.tok.Kind == lexer.IntLit {
					idx := p.tok.Text
					pos := ast.Pos(p.tok.Pos)
					p.next()
					expr = &ast.FieldExpr{Pos: pos, Expr: expr, Field: idx}
				} else {
					field := p.expect(lexer.Ident)
					if p.tok.Kind == lexer.LParen {
						expr = p.parseMethodCall(expr, field)
					} else {
						expr = &ast.FieldExpr{Pos: ast.Pos(field.Pos), Expr: expr, Field: field.Text}
					}
				}
			} else if p.tok.Kind == lexer.LBracket {
				p.next()
				idx := p.parseExpr()
				p.expect(lexer.RBracket)
				expr = &ast.IndexExpr{Pos: pos, Expr: expr, Index: idx}
			} else if p.tok.Kind == lexer.LBrace {
				// Disambiguate: lowercase ident followed by `{` is not a struct literal.
				if ident, ok := expr.(*ast.Ident); ok && len(ident.Name) > 0 && ident.Name[0] >= 'A' && ident.Name[0] <= 'Z' {
					expr = p.parseStructLitFromName(pos, ident.Name)
				} else if path, ok := expr.(*ast.PathExpr); ok && len(path.Segments) > 0 {
					last := path.Segments[len(path.Segments)-1]
					if len(last) > 0 && last[0] >= 'A' && last[0] <= 'Z' {
						expr = p.parseStructLitFromName(pos, last)
					} else {
						break
					}
				} else {
					break
				}
			} else {
				break
			}
		}
		return expr
	case lexer.LParen:
		pos := ast.Pos(p.tok.Pos)
		p.next()
		if p.tok.Kind == lexer.RParen {
			p.next()
			return &ast.TupleExpr{Pos: pos, Elements: []ast.Expr{}}
		}
		first := p.parseExpr()
		if p.tok.Kind != lexer.Comma {
			p.expect(lexer.RParen)
			return first
		}
		var elems []ast.Expr
		elems = append(elems, first)
		for p.tok.Kind == lexer.Comma {
			p.next()
			if p.tok.Kind == lexer.RParen {
				break
			}
			elems = append(elems, p.parseExpr())
		}
		p.expect(lexer.RParen)
		return &ast.TupleExpr{Pos: pos, Elements: elems}
	case lexer.LBrace:
		return p.parseBlock()
	case lexer.LBracket:
		return p.parseArrayLit()
	default:
		p.setErr("unexpected token %v in expression", p.tok.Kind)
		return nil
	}
}

func (p *Parser) parsePathOrExpr(pos ast.Pos, first string) ast.Expr {
	if p.tok.Kind != lexer.ColonColon {
		return &ast.Ident{Pos: pos, Name: first}
	}
	segs := []string{first}
	for p.tok.Kind == lexer.ColonColon {
		p.next()
		part := p.expect(lexer.Ident)
		segs = append(segs, part.Text)
	}
	return &ast.PathExpr{Pos: pos, Segments: segs}
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

func (p *Parser) parseMethodCall(receiver ast.Expr, method lexer.Token) ast.Expr {
	pos := ast.Pos(method.Pos)
	field := &ast.FieldExpr{Pos: pos, Expr: receiver, Field: method.Text}
	return p.parseCall(field)
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

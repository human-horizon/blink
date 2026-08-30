package parser

import (
	"fmt"
	"strconv"
	"strings"

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
	peeked   lexer.Token
	hasPeek  bool
}

// New creates a new parser from a lexer.
func New(l *lexer.Lexer, src []byte) *Parser {
	p := &Parser{l: l, source: src}
	p.next()
	if l.Err() != nil && p.err == nil {
		p.err = l.Err()
	}
	return p
}

// ParseFile parses a full source file into a File.
func (p *Parser) ParseFile() (*ast.File, error) {
	file := &ast.File{}
	_ = p.skipInnerAttributes()
	for p.err == nil && p.tok.Kind != lexer.EOF {
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
		return
	}
	p.err = fmt.Errorf(format, args...)
	p.errCount++
	if p.errCount > 100 {
		p.err = fmt.Errorf("too many parse errors, aborting")
	}
}

func (p *Parser) next() {
	if p.hasPeek {
		p.tok = p.peeked
		p.hasPeek = false
		return
	}
	p.tok = p.l.Next()
	if p.l.Err() != nil && p.err == nil {
		p.err = p.l.Err()
	}
}

func (p *Parser) peekNext() lexer.Token {
	if !p.hasPeek {
		p.peeked = p.l.Next()
		p.hasPeek = true
	}
	return p.peeked
}

func (p *Parser) expect(kind lexer.TokenKind) lexer.Token {
	if p.tok.Kind != kind {
		p.setErr("expected %v, got %v at offset %d", kind, p.tok.Kind, p.tok.Pos)
		return p.tok
	}
	t := p.tok
	p.next()
	return t
}

func (p *Parser) skipParens() {
	if p.err != nil {
		return
	}
	p.next() // (
	p.skipDelimited(lexer.RParen)
	if p.tok.Kind == lexer.RParen {
		p.next()
	}
}

func (p *Parser) skipDelimited(end lexer.TokenKind) {
	depth := 1
	for p.err == nil && p.tok.Kind != lexer.EOF && depth > 0 {
		if p.tok.Kind == end {
			depth--
			if depth == 0 {
				p.next()
				return
			}
			p.next()
			continue
		}
		if p.tok.Kind == lexer.LBrace || p.tok.Kind == lexer.LParen || p.tok.Kind == lexer.LBracket {
			depth++
		} else if p.tok.Kind == lexer.RBrace || p.tok.Kind == lexer.RParen || p.tok.Kind == lexer.RBracket {
			depth--
			if depth <= 0 {
				p.next()
				return
			}
		}
		p.next()
	}
	if depth > 0 {
		p.setErr("unterminated delimiter")
	}
}

func (p *Parser) skipVisibility() string {
	if p.tok.Kind != lexer.LParen {
		return ""
	}
	p.next()
	start := p.tok.Pos
	for p.err == nil && p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
		p.next()
	}
	end := p.tok.Pos
	p.expect(lexer.RParen)
	if start < end {
		return string(p.source[start:end])
	}
	return ""
}

func (p *Parser) skipAttribute() string {
	if p.tok.Kind != lexer.Hash {
		return ""
	}
	p.next()
	if p.tok.Kind == lexer.Bang {
		p.next()
	}
	if p.tok.Kind != lexer.LBracket {
		p.setErr("expected `[` after attribute")
		return ""
	}
	start := p.tok.Pos
	p.next() // [
	p.skipDelimited(lexer.RBracket)
	end := p.tok.Pos

	if start < end && end <= len(p.source) {
		return string(p.source[start:end])
	}
	return ""
}

func (p *Parser) skipAttributes() []string {
	var attrs []string
	for p.err == nil && p.tok.Kind == lexer.Hash {
		attrs = append(attrs, p.skipAttribute())
	}
	return attrs
}

func (p *Parser) skipInnerAttributes() []string {
	var attrs []string
	for p.err == nil && p.tok.Kind == lexer.Hash {
		// Only consume inner attributes #![...]; regular #[...] belong to the next item.
		if p.peekNext().Kind != lexer.Bang {
			break
		}
		attrs = append(attrs, p.skipAttribute())
	}
	return attrs
}

func (p *Parser) parseDecl() ast.Decl {
	if p.err != nil {
		return nil
	}
	attrs := p.skipAttributes()
	cfgTest := hasCfgTest(attrs)
	_ = attrs
	pub := false
	if p.tok.Kind == lexer.Pub {
		pub = true
		p.next()
		p.skipVisibility()
	}
	// Re-check attributes after visibility (e.g. #[attr] pub fn)
	for p.err == nil && p.tok.Kind == lexer.Hash {
		p.skipAttribute()
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
		return p.parseModDecl(pub, cfgTest)
	case lexer.Use:
		return p.parseUseDecl()
	case lexer.Ident:
		if p.tok.Text == "extern" {
			return p.parseExternCrateDecl()
		}
		if p.tok.Text == "macro_rules" {
			p.next()
			if p.tok.Kind == lexer.Bang {
				return p.parseMacroDecl(pub)
			}
			p.setErr("expected `!` after `macro_rules`")
			return nil
		}
		if p.tok.Text == "const" {
			return p.parseConstDecl(pub)
		}
		if p.tok.Text == "static" {
			return p.parseStaticDecl(pub)
		}
		if p.tok.Text == "type" {
			return p.parseTypeAliasDecl(pub)
		}
		if p.tok.Text == "unsafe" {
			p.next()
			if p.tok.Kind == lexer.Fn {
				return p.parseFnDecl(pub)
			}
			p.setErr("expected `fn` after `unsafe`")
			return nil
		}
		// Item-level macro invocation: ident! ... or path::ident! ...
		if p.peekNext().Kind == lexer.Bang {
			return p.parseItemMacroCall(pub)
		}
		// Scan for path::ident! ...
		if p.peekNext().Kind == lexer.ColonColon {
			savedTok := p.tok
			savedPeek := p.peeked
			savedHasPeek := p.hasPeek
			p.next()
			p.next()
			for p.err == nil && p.tok.Kind == lexer.Ident {
				if p.peekNext().Kind == lexer.ColonColon {
					p.next()
					p.next()
					continue
				}
				if p.peekNext().Kind == lexer.Bang {
					p.tok = savedTok
					p.peeked = savedPeek
					p.hasPeek = savedHasPeek
					return p.parseItemMacroCall(pub)
				}
				break
			}
			p.tok = savedTok
			p.peeked = savedPeek
			p.hasPeek = savedHasPeek
		}
		p.setErr("unexpected token %v at top level at offset %d", p.tok.Kind, p.tok.Pos)
		return nil
	case lexer.Unsafe:
		p.next()
		if p.tok.Kind == lexer.Fn {
			return p.parseFnDecl(pub)
		}
		p.setErr("expected `fn` after `unsafe`")
		return nil
	default:
		p.setErr("unexpected token %v at top level at offset %d", p.tok.Kind, p.tok.Pos)
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
	return &ast.ConstDecl{Pos: pos, Pub: pub, Name: name.Text, Ty: ty, Value: value}
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
	return &ast.StaticDecl{Pos: pos, Pub: pub, Name: name.Text, Ty: ty, Value: value}
}

func (p *Parser) parseTypeAliasDecl(pub bool) ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // type
	name := p.expect(lexer.Ident)
	p.parseParams() // optional generic params
	p.expect(lexer.Eq)
	ty := p.parseType()
	if p.tok.Kind == lexer.Semi {
		p.next()
	}
	return &ast.TypeAliasDecl{Pos: pos, Pub: pub, Name: name.Text, Ty: ty}
}

func (p *Parser) parseMacroDecl(pub bool) ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // !
	name := p.expect(lexer.Ident)
	if p.tok.Kind != lexer.LBrace && p.tok.Kind != lexer.LBracket && p.tok.Kind != lexer.LParen {
		p.setErr("expected delimiter after `!` in macro_rules")
		return nil
	}
	kind := p.tok.Kind
	p.next() // consume opening delimiter
	var body ast.Expr
	if kind == lexer.LBrace && p.tok.Kind == lexer.LParen {
		// Try to parse a simple rule: () => expr ; ...
		savedTok := p.tok
		savedPeek := p.peeked
		savedHasPeek := p.hasPeek
		savedErr := p.err
		savedErrCount := p.errCount
		p.next() // (
		if p.tok.Kind == lexer.RParen {
			p.next() // )
			if p.tok.Kind == lexer.FatArrow {
				p.next() // =>
				body = p.parseExpr()
			}
		}
		if body == nil {
			// Rollback and skip body.
			p.tok = savedTok
			p.peeked = savedPeek
			p.hasPeek = savedHasPeek
			p.err = savedErr
			p.errCount = savedErrCount
		}
	}
	if body == nil {
		p.skipDelimited(matchingEnd(kind))
	} else {
		// Skip any remaining rules and consume closing delimiter.
		p.skipDelimited(matchingEnd(kind))
	}
	if p.tok.Kind == lexer.Semi {
		p.next()
	}
	return &ast.MacroRulesDecl{Pos: pos, Pub: pub, Name: name.Text, Body: body}
}

func (p *Parser) parseItemMacroCall(pub bool) ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	name := p.tok.Text
	p.next() // ident
	p.next() // !
	if p.tok.Kind != lexer.LBrace && p.tok.Kind != lexer.LBracket && p.tok.Kind != lexer.LParen {
		p.setErr("expected delimiter after `!` in macro invocation")
		return nil
	}
	kind := p.tok.Kind
	p.next() // consume opening delimiter
	p.skipDelimited(matchingEnd(kind))
	if p.tok.Kind == lexer.Semi {
		p.next()
	}
	return &ast.MacroCallDecl{Pos: pos, Pub: pub, Name: name}
}

func matchingEnd(kind lexer.TokenKind) lexer.TokenKind {
	switch kind {
	case lexer.LBrace:
		return lexer.RBrace
	case lexer.LBracket:
		return lexer.RBracket
	case lexer.LParen:
		return lexer.RParen
	}
	return lexer.EOF
}

func (p *Parser) parseExternCrateDecl() ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // extern
	crate := p.expect(lexer.Ident)
	if crate.Text != "crate" {
		p.setErr("expected `crate` after `extern`")
		return nil
	}
	name := ""
	if p.tok.Kind == lexer.Self {
		name = p.tok.Text
		p.next()
	} else {
		nameTok := p.expect(lexer.Ident)
		name = nameTok.Text
	}
	alias := ""
	if p.tok.Kind == lexer.As {
		p.next()
		aliasTok := p.expect(lexer.Ident)
		alias = aliasTok.Text
	}
	if p.tok.Kind == lexer.Semi {
		p.next()
	}
	return &ast.ExternCrateDecl{Pos: pos, Name: name, Alias: alias}
}

func (p *Parser) parseMacroInvocation(pos ast.Pos, name string) ast.Expr {
	if p.err != nil {
		return nil
	}
	p.next() // !
	if p.tok.Kind != lexer.LBrace && p.tok.Kind != lexer.LBracket && p.tok.Kind != lexer.LParen {
		p.setErr("expected delimiter after `!` in macro invocation")
		return nil
	}
	kind := p.tok.Kind
	p.next() // consume opening delimiter
	p.skipDelimited(matchingEnd(kind))
	return &ast.MacroCallExpr{Pos: pos, Name: name}
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
	for p.err == nil && p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
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
		isMut := false
		if p.tok.Kind == lexer.Ident && p.tok.Text == "mut" {
			p.next()
			isMut = true
		}
		if (p.tok.Kind == lexer.Ident && p.tok.Text == "self") || p.tok.Kind == lexer.Self {
			p.next()
			return ast.Param{Pos: paramPos, Name: "self", IsSelf: true, IsMut: isMut}
		}
		p.setErr("expected `self` after `&`")
		return ast.Param{Pos: paramPos, Name: "self", IsSelf: true, IsMut: isMut}
	}
	if (p.tok.Kind == lexer.Ident && p.tok.Text == "self") || p.tok.Kind == lexer.Self {
		p.next()
		return ast.Param{Pos: paramPos, Name: "self", IsSelf: true}
	}
	// mut param: Type
	if p.tok.Kind == lexer.Ident && p.tok.Text == "mut" {
		p.next()
		paramName := p.expect(lexer.Ident)
		p.expect(lexer.Colon)
		ty := p.parseType()
		return ast.Param{Pos: paramPos, Name: paramName.Text, Ty: ty}
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
	for p.err == nil && p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		p.skipAttributes()
		isConst := false
		isUnsafe := false
		if p.tok.Kind == lexer.Ident && p.tok.Text == "const" {
			isConst = true
			p.next()
		}
		if p.tok.Kind == lexer.Unsafe || (p.tok.Kind == lexer.Ident && p.tok.Text == "unsafe") {
			isUnsafe = true
			p.next()
		}
		if p.tok.Kind != lexer.Fn {
			// Skip type aliases inside trait
			if p.tok.Kind == lexer.Ident && p.tok.Text == "type" {
				p.next() // type
				_ = p.expect(lexer.Ident)
				if p.tok.Kind == lexer.Lt {
					_, _, _ = p.parseParams()
				}
				if p.tok.Kind == lexer.Eq {
					p.next()
					_ = p.parseType()
				}
				if p.tok.Kind == lexer.Semi {
					p.next()
				}
				continue
			}
			p.setErr("expected method signature at offset %d (tok=%v/%q)", p.tok.Pos, p.tok.Kind, p.tok.Text)
			break
		}
		decl := p.parseFnSig()
		if decl == nil {
			break
		}
		fn, ok := decl.(*ast.FnDecl)
		if !ok {
			p.setErr("expected method signature after decl at offset %d (tok=%v/%q)", p.tok.Pos, p.tok.Kind, p.tok.Text)
			break
		}
		fn.IsConst = isConst
		fn.IsUnsafe = isUnsafe
		if p.tok.Kind == lexer.Semi {
			p.next()
		}
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
	p.skipAttributes()
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
	for p.err == nil && p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		p.skipAttributes()
		pub := false
		if p.tok.Kind == lexer.Pub {
			pub = true
			p.next()
			p.skipVisibility()
		}
		isConst := false
		isUnsafe := false
		if p.tok.Kind == lexer.Ident && p.tok.Text == "const" && p.peekNext().Kind == lexer.Fn {
			isConst = true
			p.next()
		}
		if p.tok.Kind == lexer.Unsafe || (p.tok.Kind == lexer.Ident && p.tok.Text == "unsafe") {
			isUnsafe = true
			p.next()
		}
		if p.tok.Kind != lexer.Fn {
			// Skip const items inside impl
			if p.tok.Kind == lexer.Ident && p.tok.Text == "const" {
				p.skipConstItem()
				continue
			}
			// Skip type aliases inside impl
			if p.tok.Kind == lexer.Ident && p.tok.Text == "type" {
				p.next() // type
				_ = p.expect(lexer.Ident)
				if p.tok.Kind == lexer.Lt {
					_, _, _ = p.parseParams()
				}
				if p.tok.Kind == lexer.Eq {
					p.next()
					_ = p.parseType()
				}
				if p.tok.Kind == lexer.Semi {
					p.next()
				}
				continue
			}
			p.setErr("expected method declaration at offset %d (tok=%v/%q)", p.tok.Pos, p.tok.Kind, p.tok.Text)
			break
		}
		decl := p.parseFnDecl(pub)
		if fn, ok := decl.(*ast.FnDecl); ok {
			fn.IsConst = isConst
			fn.IsUnsafe = isUnsafe
			methods = append(methods, fn)
		}
	}
	p.expect(lexer.RBrace)
	return &ast.ImplDecl{Pos: pos, Trait: trait, ForType: forType, GenParams: genParams, Bounds: bounds, Methods: methods}
}

func (p *Parser) skipConstItem() {
	if p.err != nil {
		return
	}
	// const NAME: Type = expr;
	p.next() // const
	p.expect(lexer.Ident)
	p.expect(lexer.Colon)
	p.parseType()
	p.expect(lexer.Eq)
	p.parseExpr()
	if p.tok.Kind == lexer.Semi {
		p.next()
	}
}

func hasCfgTest(attrs []string) bool {
	for _, a := range attrs {
		if strings.Contains(a, "cfg(test)") {
			return true
		}
	}
	return false
}

func (p *Parser) parseModDecl(pub bool, cfgTest bool) ast.Decl {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // mod
	name := p.expect(lexer.Ident)
	if p.tok.Kind == lexer.Semi {
		p.next()
		if cfgTest {
			return &ast.ModDecl{Pos: pos, Pub: pub, Name: name.Text, Inline: &ast.File{}}
		}
		return &ast.ModDecl{Pos: pos, Pub: pub, Name: name.Text, File: name.Text + ".rs"}
	}
	p.expect(lexer.LBrace)
	if cfgTest {
		// Skip parsing cfg(test) module body to avoid pulling in test-only items.
		p.skipDelimited(lexer.RBrace)
		return &ast.ModDecl{Pos: pos, Pub: pub, Name: name.Text, Inline: &ast.File{}}
	}
	inline := &ast.File{}
	for p.err == nil && p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		decl := p.parseDecl()
		if decl != nil {
			inline.Decls = append(inline.Decls, decl)
		} else if p.err == nil {
			// Skip an unexpected token in module body to recover.
			if p.tok.Kind == lexer.Semi {
				p.next()
			} else {
				p.next()
			}
		}
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
	path, group, glob := p.parseUsePath()
	alias := ""
	if p.tok.Kind == lexer.As {
		p.next()
		aliasTok := p.expect(lexer.Ident)
		alias = aliasTok.Text
	}
	if p.tok.Kind == lexer.Semi {
		p.next()
	}
	return &ast.UseDecl{Pos: pos, Path: path, Group: group, Glob: glob, Alias: alias}
}

func (p *Parser) parseUsePath() (path []string, group bool, glob bool) {
	if p.tok.Kind == lexer.LBrace {
		// grouped use body: just consume until matching brace
		group = true
		p.next()
		for p.err == nil && p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
			_, _, _ = p.parseUsePath()
			if p.tok.Kind == lexer.Comma {
				p.next()
			}
		}
		p.expect(lexer.RBrace)
		return nil, group, glob
	}
	first := p.expect(lexer.Ident)
	path = append(path, first.Text)
	for p.err == nil && p.tok.Kind == lexer.ColonColon {
		p.next()
		if p.tok.Kind == lexer.Ident {
			part := p.expect(lexer.Ident)
			path = append(path, part.Text)
		} else if p.tok.Kind == lexer.LBrace {
			group = true
			_, _, _ = p.parseUsePath()
			return path, group, glob
		} else if p.tok.Kind == lexer.Star {
			p.next()
			glob = true
			return path, group, glob
		} else {
			p.setErr("expected identifier, `*`, or `{` in use path")
			break
		}
	}
	return path, false, false
}

func (p *Parser) parseFnSig() ast.Decl {
	if p.err != nil {
		return nil
	}
	p.skipAttributes()
	pos := ast.Pos(p.tok.Pos)
	p.expect(lexer.Fn)
	name := p.expect(lexer.Ident)
	lifeParams, genParams, bounds := p.parseParams()
	p.expect(lexer.LParen)
	var params []ast.Param
	for p.err == nil && p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
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
	if p.tok.Kind == lexer.Semi {
		p.next()
		return &ast.StructDecl{Pos: pos, Pub: pub, Name: name.Text, LifetimeParams: lifeParams, GenParams: genParams, Bounds: bounds}
	}
	if p.tok.Kind == lexer.LParen {
		// Tuple/unit struct
		p.next()
		var fields []ast.Field
		for p.err == nil && p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
			fieldPos := ast.Pos(p.tok.Pos)
			fieldPub := false
			if p.tok.Kind == lexer.Pub {
				fieldPub = true
				p.next()
				p.skipVisibility()
			}
			ty := p.parseType()
			fields = append(fields, ast.Field{Pos: fieldPos, Name: fmt.Sprintf("_%d", len(fields)), Pub: fieldPub, Ty: ty})
			if p.tok.Kind == lexer.Comma {
				p.next()
			}
		}
		p.expect(lexer.RParen)
		p.expect(lexer.Semi)
		return &ast.StructDecl{Pos: pos, Pub: pub, Name: name.Text, LifetimeParams: lifeParams, GenParams: genParams, Bounds: bounds, Fields: fields}
	}
	p.expect(lexer.LBrace)
	var fields []ast.Field
	for p.err == nil && p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		p.skipAttributes()
		if p.tok.Kind == lexer.RBrace {
			break
		}
		fieldPos := ast.Pos(p.tok.Pos)
		fieldPub := false
		if p.tok.Kind == lexer.Pub {
			fieldPub = true
			p.next()
			p.skipVisibility()
		}
		fieldName := p.expect(lexer.Ident)
		p.expect(lexer.Colon)
		ty := p.parseType()
		fields = append(fields, ast.Field{Pos: fieldPos, Name: fieldName.Text, Pub: fieldPub, Ty: ty})
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
	for p.err == nil && p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
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
	for p.err == nil && p.tok.Kind != lexer.Gt && p.tok.Kind != lexer.EOF {
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
				var trait string
				for p.err == nil {
					traitTok := p.expect(lexer.Ident)
					if trait != "" {
						trait += "::"
					}
					trait += traitTok.Text
					if p.tok.Kind != lexer.Plus {
						break
					}
					p.next()
				}
				bounds = append(bounds, ast.Constraint{Param: name, Trait: trait})
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
	for p.err == nil && p.tok.Kind != lexer.Gt && p.tok.Kind != lexer.EOF {
		// Handle associated type bound: Name = Type
		if p.tok.Kind == lexer.Ident && p.peekNext().Kind == lexer.Eq {
			name := p.tok.Text
			p.next()
			p.next()
			ty := p.parseType()
			args = append(args, &ast.NamedType{Pos: ast.Pos(0), Name: name, Args: []ast.Type{ty}})
		} else {
			args = append(args, p.parseType())
		}
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.Gt)
	return args
}

func (p *Parser) skipGenericArgs() {
	if p.err != nil {
		return
	}
	p.expect(lexer.Lt)
	for p.err == nil && p.tok.Kind != lexer.Gt && p.tok.Kind != lexer.EOF {
		p.next()
	}
	p.expect(lexer.Gt)
}

func (p *Parser) parseType() ast.Type {
	if p.err != nil {
		return nil
	}
	switch p.tok.Kind {
	case lexer.Ident, lexer.Self:
		pos := ast.Pos(p.tok.Pos)
		name := p.tok.Text
		p.next()
		var args []ast.Type
		if p.tok.Kind == lexer.Lt {
			args = p.parseTypeArgs()
		}
		// Handle path like Self::Item or std::vec::Vec
		for p.err == nil && p.tok.Kind == lexer.ColonColon {
			p.next()
			if p.tok.Kind != lexer.Ident {
				p.setErr("expected ident in type path")
				return nil
			}
			name = name + "::" + p.tok.Text
			p.next()
			if p.tok.Kind == lexer.Lt {
				args = p.parseTypeArgs()
			}
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
	case lexer.Impl:
		return p.parseImplTraitType()
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
		if p.tok.Kind == lexer.Semi {
			p.next()
			lenTok := p.expect(lexer.IntLit)
			lenVal, _ := strconv.ParseInt(lenTok.Text, 10, 64)
			p.expect(lexer.RBracket)
			return &ast.ArrayType{Pos: pos, Elem: elem, Len: lenVal}
		}
		p.expect(lexer.RBracket)
		return &ast.SliceType{Pos: pos, Elem: elem}
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
		for p.err == nil && p.tok.Kind == lexer.Comma {
			p.next()
			if p.tok.Kind == lexer.RParen {
				break
			}
			elems = append(elems, p.parseType())
		}
		p.expect(lexer.RParen)
		return &ast.TupleType{Pos: pos, ElementTypes: elems}
	default:
		p.setErr("expected type, got %v at offset %d", p.tok.Kind, p.tok.Pos)
		return nil
	}
}

func (p *Parser) parseImplTraitType() ast.Type {
	pos := ast.Pos(p.tok.Pos)
	p.next() // impl
	trait := p.parseType()
	for p.err == nil && p.tok.Kind == lexer.Plus {
		p.next()
		if p.tok.Kind == lexer.Lifetime {
			p.next()
		} else {
			p.parseType()
		}
	}
	return &ast.ImplTraitType{Pos: pos, Trait: trait}
}

func (p *Parser) parseBlock() *ast.BlockExpr {
	if p.err != nil {
		return &ast.BlockExpr{}
	}
	pos := ast.Pos(p.tok.Pos)
	p.expect(lexer.LBrace)
	var stmts []ast.Stmt
	var result ast.Expr
	for p.err == nil && p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
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
	case lexer.For:
		return p.parseForStmt(), true
	case lexer.If:
		stmt := &ast.ExprStmt{Expr: p.parseIfExpr()}
		if p.tok.Kind == lexer.Semi {
			p.next()
			return stmt, true
		}
		return stmt, false
	case lexer.Match:
		return &ast.ExprStmt{Expr: p.parseMatchExpr()}, true
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

func (p *Parser) parseMatchStmt() ast.Stmt {
	return nil
}

func (p *Parser) parseMatchExpr() ast.Expr {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // match
	_ = p.parseExpr()
	p.expect(lexer.LBrace)
	for p.err == nil && p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		_ = p.parseMatchPattern()
		if p.tok.Kind == lexer.Pipe {
			for p.err == nil && p.tok.Kind == lexer.Pipe {
				p.next()
				_ = p.parseMatchPattern()
			}
		}
		if p.tok.Kind == lexer.FatArrow {
			p.next()
			if p.tok.Kind == lexer.LBrace {
				p.next()
				depth := 1
				for p.err == nil && p.tok.Kind != lexer.EOF && depth > 0 {
					if p.tok.Kind == lexer.LBrace {
						depth++
					} else if p.tok.Kind == lexer.RBrace {
						depth--
						if depth == 0 {
							p.next()
							break
						}
					}
					p.next()
				}
			} else {
				parenDepth := 0
				bracketDepth := 0
				braceDepth := 0
				for p.err == nil && p.tok.Kind != lexer.EOF && p.tok.Kind != lexer.RBrace {
					if parenDepth == 0 && bracketDepth == 0 && braceDepth == 0 && (p.tok.Kind == lexer.Comma || p.tok.Kind == lexer.FatArrow) {
						break
					}
					switch p.tok.Kind {
					case lexer.LParen:
						parenDepth++
					case lexer.RParen:
						parenDepth--
					case lexer.LBracket:
						bracketDepth++
					case lexer.RBracket:
						bracketDepth--
					case lexer.LBrace:
						braceDepth++
					case lexer.RBrace:
						braceDepth--
					}
					p.next()
				}
			}
		}
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.RBrace)
	return &ast.Ident{Pos: pos, Name: "<match>"}
}

// parseMatchPattern is a thin wrapper around parsePattern that returns nil for unsupported forms.
func (p *Parser) parseMatchPattern() ast.Pattern {
	if p.err != nil {
		return nil
	}
	return p.parsePattern()
}

func (p *Parser) parseForStmt() ast.Stmt {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // for
	pat := p.parsePattern()
	if p.tok.Kind != lexer.Ident || p.tok.Text != "in" {
		p.setErr("expected `in` in for loop")
		return nil
	}
	p.next() // in
	iter := p.parseExpr()
	body := p.parseBlock()
	return &ast.ForStmt{Pos: pos, Pat: pat, Iter: iter, Body: body}
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
	if p.tok.Kind == lexer.IntLit || p.tok.Kind == lexer.StringLit || p.tok.Kind == lexer.True || p.tok.Kind == lexer.False {
		text := p.tok.Text
		kind := p.tok.Kind
		p.next()
		_ = text
		_ = kind
		return &ast.PatWildcard{Pos: pos}
	}
	if p.tok.Kind == lexer.Minus {
		p.next()
		_ = p.parsePattern()
		return &ast.PatWildcard{Pos: pos}
	}
	if p.tok.Kind == lexer.Ident {
		name := p.tok.Text
		p.next()
		if name == "_" {
			return &ast.PatWildcard{Pos: pos}
		}
		if p.tok.Kind == lexer.ColonColon {
			// Path pattern like Enum::Variant or Enum::Variant(binding)
			var path []string
			path = append(path, name)
			for p.err == nil && p.tok.Kind == lexer.ColonColon {
				p.next()
				if p.tok.Kind != lexer.Ident {
					p.setErr("expected ident in pattern path")
					return nil
				}
				path = append(path, p.tok.Text)
				p.next()
			}
			if p.tok.Kind == lexer.LParen {
				p.next()
				var elems []ast.Pattern
				for p.err == nil && p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
					elems = append(elems, p.parsePattern())
					if p.tok.Kind == lexer.Comma {
						p.next()
					}
				}
				p.expect(lexer.RParen)
				return &ast.PatTuple{Pos: pos, Elements: elems}
			}
			return &ast.PatIdent{Pos: pos, Name: strings.Join(path, "::")}
		}
		if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' && p.tok.Kind == lexer.LBrace {
			return p.parseStructPattern(pos, name)
		}
		if len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z' && p.tok.Kind == lexer.LParen {
			// Enum variant tuple pattern: Name(args)
			p.next()
			var elems []ast.Pattern
			for p.err == nil && p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
				elems = append(elems, p.parsePattern())
				if p.tok.Kind == lexer.Comma {
					p.next()
				}
			}
			p.expect(lexer.RParen)
			return &ast.PatTuple{Pos: pos, Elements: elems}
		}
		return &ast.PatIdent{Pos: pos, Name: name}
	}
	if p.tok.Kind == lexer.LParen {
		p.next()
		var elems []ast.Pattern
		for p.err == nil && p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
			elems = append(elems, p.parsePattern())
			if p.tok.Kind == lexer.Comma {
				p.next()
			}
		}
		p.expect(lexer.RParen)
		return &ast.PatTuple{Pos: pos, Elements: elems}
	}
	if p.tok.Kind == lexer.LBracket {
		p.next()
		var elems []ast.Pattern
		for p.err == nil && p.tok.Kind != lexer.RBracket && p.tok.Kind != lexer.EOF {
			elems = append(elems, p.parsePattern())
			if p.tok.Kind == lexer.Comma {
				p.next()
			}
		}
		p.expect(lexer.RBracket)
		return &ast.PatTuple{Pos: pos, Elements: elems}
	}
	p.setErr("expected pattern at offset %d (tok=%v/%q)", p.tok.Pos, p.tok.Kind, p.tok.Text)
	return nil
}

func (p *Parser) parseStructPattern(pos ast.Pos, name string) ast.Pattern {
	p.expect(lexer.LBrace)
	var fields []ast.PatField
	for p.err == nil && p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
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
	for p.err == nil && p.tok.Kind == lexer.Or {
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
	for p.err == nil && p.tok.Kind == lexer.And {
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
	for p.err == nil && (p.tok.Kind == lexer.EqEq || p.tok.Kind == lexer.NotEq) {
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
	for p.err == nil && (p.tok.Kind == lexer.Lt || p.tok.Kind == lexer.Gt) {
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
	for p.err == nil && (p.tok.Kind == lexer.Plus || p.tok.Kind == lexer.Minus) {
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
	for p.err == nil && (p.tok.Kind == lexer.Star || p.tok.Kind == lexer.Slash) {
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
	var expr ast.Expr
	switch p.tok.Kind {
	case lexer.Minus, lexer.Bang, lexer.Star:
		pos := ast.Pos(p.tok.Pos)
		op := p.tok.Text
		p.next()
		expr = p.parseUnary()
		return &ast.UnaryExpr{Pos: pos, Op: op, Operand: expr}
	case lexer.And:
		pos := ast.Pos(p.tok.Pos)
		op := "&"
		p.next()
		if p.tok.Kind == lexer.Ident && p.tok.Text == "mut" {
			op = "&mut"
			p.next()
		}
		expr = p.parseUnary()
		return &ast.UnaryExpr{Pos: pos, Op: op, Operand: expr}
	case lexer.Ident:
		if p.tok.Kind == lexer.Ident && p.tok.Text == "mut" {
			p.next()
			return p.parseUnary()
		}
		fallthrough
	default:
		expr = p.parsePrimary()
	}
	// Postfix ? operator (try)
	for p.err == nil && p.tok.Kind == lexer.Question {
		pos := ast.Pos(p.tok.Pos)
		p.next()
		expr = &ast.UnaryExpr{Pos: pos, Op: "?", Operand: expr}
	}
	return expr
}

func (p *Parser) parsePrimary() ast.Expr {
	if p.err != nil {
		return nil
	}
	var expr ast.Expr
	switch p.tok.Kind {
	case lexer.IntLit:
		pos := ast.Pos(p.tok.Pos)
		val, _ := strconv.ParseInt(p.tok.Text, 10, 64)
		p.next()
		expr = &ast.IntLit{Pos: pos, Val: val}
	case lexer.True:
		pos := ast.Pos(p.tok.Pos)
		p.next()
		expr = &ast.BoolLit{Pos: pos, Val: true}
	case lexer.False:
		pos := ast.Pos(p.tok.Pos)
		p.next()
		expr = &ast.BoolLit{Pos: pos, Val: false}
	case lexer.StringLit:
		pos := ast.Pos(p.tok.Pos)
		text := p.tok.Text
		p.next()
		expr = &ast.StringLit{Pos: pos, Val: text}
	case lexer.Ident, lexer.Self:
		pos := ast.Pos(p.tok.Pos)
		name := p.tok.Text
		p.next()
		if p.tok.Kind == lexer.Bang {
			return p.parseMacroInvocation(pos, name)
		}
		expr = p.parsePathOrExpr(pos, name)
	case lexer.LParen:
		pos := ast.Pos(p.tok.Pos)
		p.next()
		if p.tok.Kind == lexer.RParen {
			p.next()
			expr = &ast.TupleExpr{Pos: pos, Elements: []ast.Expr{}}
		} else {
			first := p.parseExpr()
			if p.tok.Kind != lexer.Comma {
				p.expect(lexer.RParen)
				expr = first
			} else {
				var elems []ast.Expr
				elems = append(elems, first)
				for p.err == nil && p.tok.Kind == lexer.Comma {
					p.next()
					if p.tok.Kind == lexer.RParen {
						break
					}
					elems = append(elems, p.parseExpr())
				}
				p.expect(lexer.RParen)
				expr = &ast.TupleExpr{Pos: pos, Elements: elems}
			}
		}
	case lexer.LBrace:
		expr = p.parseBlock()
	case lexer.Unsafe:
		expr = p.parseUnsafeBlock()
	case lexer.LBracket:
		expr = p.parseArrayLit()
	case lexer.If:
		return p.parseIfExpr()
	case lexer.Match:
		expr = p.parseMatchExpr()
	case lexer.Pipe:
		return p.parseClosureExpr()
	case lexer.Dot:
		if p.peekNext().Kind == lexer.Dot {
			return p.parseRangeExpr()
		}
		p.setErr("unexpected token %v in expression at offset %d", p.tok.Kind, p.tok.Pos)
		return nil
	default:
		p.setErr("unexpected token %v in expression at offset %d", p.tok.Kind, p.tok.Pos)
		return nil
	}
	// Postfix operators: calls, fields, indexing, casts, struct literals, turbofish.
	for p.err == nil {
		pos := ast.Pos(p.tok.Pos)
		if p.tok.Kind == lexer.ColonColon {
			p.next()
			if p.tok.Kind != lexer.Lt {
				p.setErr("expected `<` after `::` in turbofish")
				return nil
			}
			p.skipGenericArgs()
			continue
		} else if p.tok.Kind == lexer.Bang {
			return p.parseMacroInvocation(pos, exprNameForCall(expr))
		} else if p.tok.Kind == lexer.LParen {
			expr = p.parseCall(expr)
		} else if p.tok.Kind == lexer.Dot {
			if p.peekNext().Kind == lexer.Dot {
				expr = p.parseOpenRange(expr)
				continue
			}
			p.next()
			if p.tok.Kind == lexer.IntLit {
				idx := p.tok.Text
				p.next()
				expr = &ast.FieldExpr{Pos: pos, Expr: expr, Field: idx}
			} else if p.tok.Kind == lexer.Ident && p.tok.Text == "await" {
				p.next()
				expr = &ast.FieldExpr{Pos: pos, Expr: expr, Field: "await"}
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
		} else if p.tok.Kind == lexer.As {
			p.next()
			ty := p.parseType()
			expr = &ast.CastExpr{Pos: pos, Expr: expr, Ty: ty}
		} else if p.tok.Kind == lexer.LBrace {
			// Disambiguate: capitalized ident/path followed by `{` is a struct literal.
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
}

func (p *Parser) parsePathOrExpr(pos ast.Pos, first string) ast.Expr {
	if p.tok.Kind != lexer.ColonColon {
		return &ast.Ident{Pos: pos, Name: first}
	}
	// Do not consume `::` if it introduces a turbofish (`::<...>`).
	if p.peekNext().Kind == lexer.Lt {
		return &ast.Ident{Pos: pos, Name: first}
	}
	segs := []string{first}
	for p.err == nil && p.tok.Kind == lexer.ColonColon {
		p.next()
		part := p.expect(lexer.Ident)
		segs = append(segs, part.Text)
		if p.tok.Kind == lexer.ColonColon && p.peekNext().Kind == lexer.Lt {
			break
		}
	}
	return &ast.PathExpr{Pos: pos, Segments: segs}
}

func exprNameForCall(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.PathExpr:
		return strings.Join(e.Segments, "::")
	default:
		return ""
	}
}

func (p *Parser) parseCall(fn ast.Expr) ast.Expr {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.expect(lexer.LParen)
	var args []ast.Expr
	for p.err == nil && p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.EOF {
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
	for p.err == nil && p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.EOF {
		fieldPos := ast.Pos(p.tok.Pos)
		fieldName := p.expect(lexer.Ident)
		var value ast.Expr
		if p.tok.Kind == lexer.Colon {
			p.next()
			value = p.parseExpr()
		} else {
			value = &ast.Ident{Pos: ast.Pos(fieldName.Pos), Name: fieldName.Text}
		}
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
	var cond ast.Expr
	if p.tok.Kind == lexer.Let {
		p.next() // let
		_ = p.parsePattern()
		if p.tok.Kind == lexer.Eq {
			p.next()
			cond = p.parseExpr()
		}
	} else {
		cond = p.parseExpr()
	}
	thenBlock := p.parseBlock()
	var elseBlock *ast.BlockExpr
	if p.tok.Kind == lexer.Else {
		p.next()
		elseBlock = p.parseBlock()
	}
	return &ast.IfExpr{Pos: pos, Cond: cond, ThenBlock: thenBlock, ElseBlock: elseBlock}
}

func (p *Parser) parseRangeExpr() ast.Expr {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // .
	p.next() // .
	var end ast.Expr
	if p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.RBracket && p.tok.Kind != lexer.Semi && p.tok.Kind != lexer.Comma && p.tok.Kind != lexer.EOF {
		end = p.parseExpr()
	}
	return &ast.RangeExpr{Pos: pos, To: end}
}

func (p *Parser) parseOpenRange(left ast.Expr) ast.Expr {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // .
	p.next() // .
	var end ast.Expr
	if p.tok.Kind != lexer.RBrace && p.tok.Kind != lexer.RParen && p.tok.Kind != lexer.RBracket && p.tok.Kind != lexer.Semi && p.tok.Kind != lexer.Comma && p.tok.Kind != lexer.EOF {
		end = p.parseExpr()
	}
	return &ast.RangeExpr{Pos: pos, From: left, To: end}
}

func (p *Parser) parseClosureExpr() ast.Expr {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // |
	var args []string
	for p.err == nil && p.tok.Kind != lexer.Pipe && p.tok.Kind != lexer.EOF {
		if p.tok.Kind == lexer.LParen {
			p.skipParens()
			continue
		}
		if p.tok.Kind == lexer.And {
			p.next()
		}
		paramName := p.expect(lexer.Ident)
		args = append(args, paramName.Text)
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.Pipe)
	body := p.parseExpr()
	return &ast.ClosureExpr{Pos: pos, Args: args, Body: body}
}

func (p *Parser) parseUnsafeBlock() ast.Expr {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.expect(lexer.Unsafe)
	body := p.parseBlock()
	return &ast.UnsafeBlockExpr{Pos: pos, Body: body}
}

func (p *Parser) parseArrayLit() ast.Expr {
	if p.err != nil {
		return nil
	}
	pos := ast.Pos(p.tok.Pos)
	p.next() // [
	var elems []ast.Expr
	for p.err == nil && p.tok.Kind != lexer.RBracket && p.tok.Kind != lexer.EOF {
		elems = append(elems, p.parseExpr())
		if p.tok.Kind == lexer.Comma {
			p.next()
		}
	}
	p.expect(lexer.RBracket)
	return &ast.ArrayLit{Pos: pos, Elems: elems}
}

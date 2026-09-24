package checker

import (
	"strconv"
	"strings"

	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/types"
)

// C code generation for match expressions and enum variants with payload.
//
// Enums where every variant is a unit stay integer #defines. Enums with any
// payload variant get a tagged-union struct:
//
//	struct E {
//	  int32_t tag;
//	  union E__payload {
//	    struct E__V { T f0; ... } V;
//	  } payload;
//	};
//
// A match compiles to a TinyCC statement expression with an if/else-if chain
// over a materialized scrutinee.

// enumHasPayload reports whether any variant of the enum carries fields.
func (g *CGen) enumHasPayload(ei *enumInfo) bool {
	for _, v := range ei.decl.Variants {
		if len(v.Fields) > 0 {
			return true
		}
	}
	return false
}

// enumByTypeName resolves a possibly module-qualified enum name as stored in
// types.Named values.
func (g *CGen) enumByTypeName(name string) (string, *enumInfo, bool) {
	if ei, ok := g.c.enums[name]; ok {
		return name, ei, true
	}
	for key, ei := range g.c.enums {
		if strings.HasSuffix(key, "::"+name) {
			return key, ei, true
		}
	}
	return "", nil, false
}

func variantIndexOf(ei *enumInfo, name string) int {
	for i := range ei.decl.Variants {
		if ei.decl.Variants[i].Name == name {
			return i
		}
	}
	return -1
}

// variantFieldName returns the C member name for payload field i of a variant.
func variantFieldName(v *ast.Variant, i int) string {
	if len(v.FieldNames) == len(v.Fields) && v.FieldNames[i] != "" {
		return v.FieldNames[i]
	}
	return "f" + strconv.Itoa(i)
}

// emitEnumPayloadDefs emits tagged-union structs for enums with payload.
func (g *CGen) emitEnumPayloadDefs() {
	for key, ei := range g.c.enums {
		if !g.enumHasPayload(ei) {
			continue
		}
		name := g.mangleName(key, nil)
		g.writeln("struct ", name, " {")
		g.indent++
		g.writei("int32_t tag;")
		g.newline()
		g.writei("union ", name, "__payload {")
		g.newline()
		g.indent++
		for _, v := range ei.decl.Variants {
			if len(v.Fields) == 0 {
				continue
			}
			fs, _ := g.c.variantFields(key, ei, v.Name)
			vstruct := name + "__" + v.Name
			g.writei("struct ", vstruct, " {")
			g.newline()
			g.indent++
			for i, ft := range fs {
				g.writei(g.cType(ft), " ", variantFieldName(&v, i), ";")
				g.newline()
			}
			g.indent--
			g.writei("} ", v.Name, ";")
			g.newline()
		}
		g.indent--
		g.writei("} payload;")
		g.newline()
		g.indent--
		g.writeln("};")
		g.writeln("")
	}
}

// enumConstructor renders `((struct E){ .tag = N })` or with payload
// `((struct E){ .tag = N, .payload = { .V = { .f0 = arg0, ... } } })`.
func (g *CGen) enumConstructor(enumKey string, ei *enumInfo, variant string, args []string) string {
	name := g.mangleName(enumKey, nil)
	idx := variantIndexOf(ei, variant)
	var b strings.Builder
	b.WriteString("((struct ")
	b.WriteString(name)
	b.WriteString("){ .tag = ")
	b.WriteString(strconv.Itoa(idx))
	if len(args) > 0 && idx >= 0 {
		v := &ei.decl.Variants[idx]
		b.WriteString(", .payload = { .")
		b.WriteString(variant)
		b.WriteString(" = { ")
		for i, a := range args {
			if i > 0 {
				b.WriteString(", ")
			}
			b.WriteString(".")
			b.WriteString(variantFieldName(v, i))
			b.WriteString(" = ")
			b.WriteString(a)
		}
		b.WriteString(" } }")
	}
	b.WriteString(" })")
	return b.String()
}

// armPattern converts a match arm (which is an or-pattern group) into a C
// condition over subject plus local declarations for bindings.
func (g *CGen) armPattern(patterns []ast.Pattern, subject string, ty types.Type) (string, string) {
	conds := make([]string, 0, len(patterns))
	decls := ""
	for _, p := range patterns {
		cond, ds := g.patCond(p, subject, ty)
		conds = append(conds, cond)
		if decls == "" {
			decls = ds
		}
	}
	if len(conds) == 0 {
		return "0", ""
	}
	return "(" + strings.Join(conds, " || ") + ")", decls
}

// patCond converts one pattern into a C condition plus binding declarations.
func (g *CGen) patCond(pat ast.Pattern, subject string, ty types.Type) (string, string) {
	switch p := pat.(type) {
	case *ast.PatWildcard:
		return "1", ""
	case *ast.PatOr:
		return g.armPattern(p.Alternatives, subject, ty)
	case *ast.PatTuple:
		tup, ok := ty.(*types.Tuple)
		if !ok || len(tup.Elems) != len(p.Elements) {
			return "0", "/* unsupported tuple pattern */ "
		}
		conds := make([]string, 0, len(p.Elements))
		decls := ""
		for i, elem := range p.Elements {
			c, ds := g.patCond(elem, subject+".f"+strconv.Itoa(i), tup.Elems[i])
			conds = append(conds, c)
			decls += ds
		}
		return "(" + strings.Join(conds, " && ") + ")", decls
	case *ast.PatPath:
		return g.pathPatCond(p, subject, ty)
	case *ast.PatRange:
		upper := " < "
		if p.Inclusive {
			upper = " <= "
		}
		return "(((" + subject + ") >= " + strconv.FormatInt(p.Low, 10) + ") && ((" + subject + ")" + upper + strconv.FormatInt(p.High, 10) + "))", ""
	case *ast.PatLit:
		switch p.Kind {
		case "int":
			v := p.Val
			if p.Neg {
				v = "-" + v
			}
			return "((" + subject + ") == " + v + ")", ""
		case "bool":
			return "((" + subject + ") == " + p.Val + ")", ""
		default:
			return "0", "/* unsupported string pattern */ "
		}
	case *ast.PatIdent:
		if p.Name == "_" {
			return "1", ""
		}
		// Capitalized identifier matching a variant name is a unit-variant
		// path pattern (`None`, `Status::Active` written as `Active`), not a
		// binding.
		if len(p.Name) > 0 && p.Name[0] >= 'A' && p.Name[0] <= 'Z' {
			if _, _, isVariant := adtTag(p.Name); isVariant && isAdt(ty) {
				return g.pathPatCond(&ast.PatPath{Path: []string{p.Name}}, subject, ty)
			}
			if name := g.typeName(ty); name != "" {
				if _, ei, ok := g.enumByTypeName(name); ok {
					if idx := variantIndexOf(ei, p.Name); idx >= 0 && len(ei.decl.Variants[idx].Fields) == 0 {
						return g.pathPatCond(&ast.PatPath{Path: []string{p.Name}}, subject, ty)
					}
				}
			}
		}
		declTy := g.cType(ty)
		if p.IsRef {
			declTy = declTy + "*"
			return "1", declTy + " " + g.ident(p.Name) + " = &(" + subject + "); "
		}
		return "1", declTy + " " + g.ident(p.Name) + " = (" + subject + "); "
	case *ast.PatStruct:
		return g.structPatCond(p, subject, ty)
	default:
		return "0", "/* unsupported pattern */ "
	}
}

// pathPatCond handles `Enum::Variant` and `Enum::Variant(sub, ...)` patterns.
func (g *CGen) pathPatCond(p *ast.PatPath, subject string, ty types.Type) (string, string) {
	if len(p.Path) == 0 {
		return "0", ""
	}
	variant := p.Path[len(p.Path)-1]
	// User enum: tag comparison plus payload bindings.
	if name := g.typeName(ty); name != "" {
		if enumKey, ei, ok := g.enumByTypeName(name); ok {
			idx := variantIndexOf(ei, variant)
			if idx >= 0 {
				if g.enumHasPayload(ei) {
					cond := "((" + subject + ").tag == " + strconv.Itoa(idx) + ")"
					fs, _ := g.c.variantFields(enumKey, ei, variant)
					if len(fs) != len(p.Elements) {
						return cond, ""
					}
					v := &ei.decl.Variants[idx]
					decls := ""
					for i, elem := range p.Elements {
						path := subject + ".payload." + variant + "." + variantFieldName(v, i)
						c, ds := g.patCond(elem, path, fs[i])
						_ = c // sub-conditions on payload fields are folded via bindings; wildcards/idents bind only
						decls += ds
					}
					return cond, decls
				}
				// Plain unit enum: compare against generated #define constant.
				return "((" + subject + ") == " + g.mangleName(enumKey, nil) + "__" + variant + ")", ""
			}
		}
	}
	// Builtin Option/Result variants: tag comparisons against ADT structs.
	if ap, ok := ty.(*types.Applied); ok && isAdt(ap) {
		if tag, member, known := adtTag(variant); known {
			cond := "((" + subject + ").tag == " + strconv.Itoa(tag) + ")"
			decls := ""
			if tag != 0 && len(p.Elements) == 1 {
				ft := adtPayloadType(ap, variant)
				_, ds := g.patCond(p.Elements[0], subject+".payload."+member, ft)
				decls = ds
			}
			return cond, decls
		}
	}
	// Builtin enums without an ADT runtime representation cannot be tested.
	return "0", "/* unsupported builtin pattern */ "
}

// structPatCond handles `Name { field: pat }` patterns for structs and for
// struct-style enum variants.
func (g *CGen) structPatCond(p *ast.PatStruct, subject string, ty types.Type) (string, string) {
	if name := g.typeName(ty); name != "" {
		if enumKey, ei, ok := g.enumByTypeName(name); ok {
			idx := variantIndexOf(ei, p.Name)
			if idx >= 0 && g.enumHasPayload(ei) {
				v := &ei.decl.Variants[idx]
				fs, _ := g.c.variantFields(enumKey, ei, p.Name)
				decls := ""
				for _, f := range p.Fields {
					fi := -1
					for j, fn := range v.FieldNames {
						if fn == f.Field {
							fi = j
							break
						}
					}
					if fi < 0 || fi >= len(fs) {
						continue
					}
					path := subject + ".payload." + p.Name + "." + f.Field
					bind := f.BindName
					if bind == "" {
						bind = f.Field
					}
					if bind == "_" {
						continue
					}
					decls += g.cType(fs[fi]) + " " + g.ident(bind) + " = (" + path + "); "
				}
				return "((" + subject + ").tag == " + strconv.Itoa(idx) + ")", decls
			}
		}
	}
	// Plain struct pattern: always matches, binds named fields.
	if _, ok := ty.(*types.Named); ok {
		decls := ""
		for _, f := range p.Fields {
			bind := f.BindName
			if bind == "" {
				bind = f.Field
			}
			if bind == "_" {
				continue
			}
			ft := g.fieldType(ty, f.Field)
			decls += g.cType(ft) + " " + g.ident(bind) + " = (" + subject + "." + f.Field + "); "
		}
		return "1", decls
	}
	return "0", "/* unsupported struct pattern */ "
}

// matchExpr renders a Rust match as `({ scrut; if ... else if ... else {} r; })`.
func (g *CGen) matchExpr(e *ast.MatchExpr) string {
	resTy := g.c.ExprType(e)
	unit := resTy == nil || isUnit(resTy)
	scrutTy := g.c.ExprType(e.Scrutinee)
	scrutExpr := g.expr(e.Scrutinee)
	var b strings.Builder
	b.WriteString("({ ")
	subject := ""
	if _, isArray := scrutTy.(*types.Array); isArray || scrutTy == nil {
		// Arrays cannot be materialized by assignment; use the expression
		// directly (single evaluation is not guaranteed in this corner).
		subject = "(" + scrutExpr + ")"
	} else {
		tmp := g.fresh("m")
		b.WriteString(g.cType(scrutTy))
		b.WriteString(" ")
		b.WriteString(tmp)
		b.WriteString(" = ")
		b.WriteString(scrutExpr)
		b.WriteString("; ")
		subject = tmp
	}
	rname := ""
	if !unit {
		rname = g.fresh("mr")
		b.WriteString(g.cType(resTy))
		b.WriteString(" ")
		b.WriteString(rname)
		b.WriteString(" = {0}; ")
	}
	for _, arm := range e.Arms {
		cond, decls := g.armPattern(arm.Patterns, subject, scrutTy)
		b.WriteString("if (")
		b.WriteString(cond)
		b.WriteString(") { ")
		b.WriteString(decls)
		body := g.expr(arm.Body)
		if unit {
			b.WriteString("(void)(")
			b.WriteString(body)
			b.WriteString("); ")
		} else {
			b.WriteString(rname)
			b.WriteString(" = (")
			b.WriteString(body)
			b.WriteString("); ")
		}
		b.WriteString("} else ")
	}
	b.WriteString("{}")
	if !unit {
		b.WriteString(" ")
		b.WriteString(rname)
		b.WriteString(";")
	} else {
		b.WriteString(" 0;")
	}
	b.WriteString(" })")
	return b.String()
}

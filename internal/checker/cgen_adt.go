package checker

import (
	"strconv"
	"strings"

	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/types"
)

// C runtime representation for the builtin ADTs Option<T> and Result<T, E>:
//
//	struct option_int32_t {
//	  int32_t tag;
//	  union option_int32_t__payload { char none; int32_t some; } payload;
//	};
//
// Tags: Option: None=0, Some=1. Result: Ok=1, Err=2.

type adtDef struct {
	name    string
	members []string // union member names in payload order
	types_  []types.Type
}

// adtBaseName returns "Option"/"Result" when t is an Applied of that base.
func adtBaseName(t types.Type) string {
	ap, ok := t.(*types.Applied)
	if !ok {
		return ""
	}
	switch b := ap.Base.(type) {
	case *types.Named:
		return b.Name
	case *types.TypeConstructor:
		return b.Name
	}
	return ""
}

// isAdt reports whether t is Option<T> or Result<T, E> with supported arity.
func isAdt(t types.Type) bool {
	ap, ok := t.(*types.Applied)
	if !ok {
		return false
	}
	switch adtBaseName(ap) {
	case "Option":
		return len(ap.Args) == 1
	case "Result":
		return len(ap.Args) == 2
	}
	return false
}

// adtTag returns the tag number for a builtin ADT variant name.
func adtTag(variant string) (int, string, bool) {
	switch variant {
	case "None":
		return 0, "none", true
	case "Some":
		return 1, "some", true
	case "Ok":
		return 1, "ok", true
	case "Err":
		return 2, "err", true
	}
	return 0, "", false
}

// adtPayloadType maps a variant to the payload field type of ap.
func adtPayloadType(ap *types.Applied, variant string) types.Type {
	switch adtBaseName(ap) {
	case "Option":
		if variant == "Some" {
			return ap.Args[0]
		}
	case "Result":
		switch variant {
		case "Ok":
			return ap.Args[0]
		case "Err":
			return ap.Args[1]
		}
	}
	return nil
}

// adtTypeName derives and registers the C struct name for an ADT instance.
func (g *CGen) adtTypeName(t types.Type) string {
	ap := t.(*types.Applied)
	var parts []string
	for _, a := range ap.Args {
		parts = append(parts, g.cType(a))
	}
	base := strings.ToLower(adtBaseName(ap))
	name := base + "_" + strings.Join(parts, "_")
	name = strings.ReplaceAll(name, " ", "")
	name = strings.ReplaceAll(name, "*", "ptr")
	name = strings.ReplaceAll(name, "[", "")
	name = strings.ReplaceAll(name, "]", "")
	if g.adtDefs == nil {
		g.adtDefs = make(map[string]*adtDef)
	}
	if _, ok := g.adtDefs[name]; !ok {
		def := &adtDef{name: name, members: []string{"none"}, types_: []types.Type{types.Bool}}
		for _, v := range []string{"Some", "Ok", "Err"} {
			if ft := adtPayloadType(ap, v); ft != nil {
				tag, member, _ := adtTag(v)
				_ = tag
				def.members = append(def.members, member)
				def.types_ = append(def.types_, ft)
			}
		}
		g.adtDefs[name] = def
	}
	return name
}

// emitAdtDefs renders registered Option/Result structs.
func (g *CGen) emitAdtDefs() {
	for _, def := range g.adtDefs {
		g.writeln("struct ", def.name, " {")
		g.indent++
		g.writei("int32_t tag;")
		g.newline()
		g.writei("union ", def.name, "__payload {")
		g.newline()
		g.indent++
		for i, m := range def.members {
			g.writei(g.cType(def.types_[i]), " ", m, ";")
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

// collectAdts registers every ADT type appearing in the program so its struct
// definition is emitted before use.
func (g *CGen) collectAdts() {
	seen := make(map[string]bool)
	var walk func(t types.Type)
	walk = func(t types.Type) {
		if t == nil {
			return
		}
		key := t.String()
		if seen[key] {
			return
		}
		seen[key] = true
		switch ty := t.(type) {
		case *types.Applied:
			if isAdt(ty) {
				g.adtTypeName(ty)
			}
			for _, a := range ty.Args {
				walk(a)
			}
		case *types.Ref:
			walk(ty.Elem)
		case *types.Array:
			walk(ty.Elem)
		case *types.Tuple:
			for _, e := range ty.Elems {
				walk(e)
			}
		}
	}
	for _, t := range g.c.exprTypes {
		walk(t)
	}
	for _, info := range g.c.fns {
		for _, p := range info.paramTypes {
			walk(p)
		}
		walk(info.ret)
		walk(info.selfType)
	}
	for _, methods := range g.c.inherent {
		for _, info := range methods {
			for _, p := range info.paramTypes {
				walk(p)
			}
			walk(info.ret)
			walk(info.selfType)
		}
	}
	for _, byType := range g.c.traitImpls {
		for _, impl := range byType {
			for _, info := range impl.methods {
				for _, p := range info.paramTypes {
					walk(p)
				}
				walk(info.ret)
			}
		}
	}
	for _, si := range g.c.structs {
		for _, ft := range si.fields {
			walk(ft)
		}
	}
}

// adtConstructor renders `((struct adt){ .tag = T, .payload = { .m = V } })`.
func (g *CGen) adtConstructor(adtTy types.Type, variant string, argExpr string) string {
	ap := adtTy.(*types.Applied)
	tag, member, _ := adtTag(variant)
	var b strings.Builder
	b.WriteString("((struct ")
	b.WriteString(g.adtTypeName(ap))
	b.WriteString("){ .tag = ")
	b.WriteString(strconv.Itoa(tag))
	if tag != 0 {
		b.WriteString(", .payload = { .")
		b.WriteString(member)
		b.WriteString(" = ")
		b.WriteString(argExpr)
		b.WriteString(" }")
	}
	b.WriteString(" })")
	return b.String()
}

// adtVariantFromFunc extracts the constructor variant from `Some`/`Ok`/`Err`
// (Ident) or `Option::Some` (PathExpr) function positions.
func adtVariantFromFunc(fn ast.Expr) string {
	switch f := fn.(type) {
	case *ast.Ident:
		switch f.Name {
		case "Some", "Ok", "Err":
			return f.Name
		}
	case *ast.PathExpr:
		if len(f.Segments) > 0 {
			last := f.Segments[len(f.Segments)-1]
			if last == "Some" || last == "Ok" || last == "Err" {
				base := f.Segments[0]
				if base == "Option" || base == "Result" {
					return last
				}
			}
		}
	}
	return ""
}

// adtMethodCall renders unwrap/is_some/is_none/is_ok/is_err on an ADT value.
// ok=false when the method is not an ADT runtime method.
func (g *CGen) adtMethodCall(recvTy types.Type, method, recv string) (string, bool) {
	ap, ok := recvTy.(*types.Applied)
	if !ok || !isAdt(ap) {
		return "", false
	}
	kind := adtBaseName(ap)
	structName := "struct " + g.adtTypeName(ap)
	tmp := g.fresh("adt")
	switch method {
	case "is_some":
		if kind != "Option" {
			return "", false
		}
		return "({ " + structName + " " + tmp + " = " + recv + "; " + tmp + ".tag == 1; })", true
	case "is_none":
		if kind != "Option" {
			return "", false
		}
		return "({ " + structName + " " + tmp + " = " + recv + "; " + tmp + ".tag == 0; })", true
	case "is_ok":
		if kind != "Result" {
			return "", false
		}
		return "({ " + structName + " " + tmp + " = " + recv + "; " + tmp + ".tag == 1; })", true
	case "is_err":
		if kind != "Result" {
			return "", false
		}
		return "({ " + structName + " " + tmp + " = " + recv + "; " + tmp + ".tag == 2; })", true
	case "unwrap":
		member := "some"
		if kind == "Result" {
			member = "ok"
		}
		return "({ " + structName + " " + tmp + " = " + recv + "; if (" + tmp + ".tag != " + ("1") + ") abort(); " + tmp + ".payload." + member + "; })", true
	}
	return "", false
}

package checker

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/humanhorizon/blink/internal/ast"
	"github.com/humanhorizon/blink/internal/diag"
	"github.com/humanhorizon/blink/internal/types"
)

// Checker performs type checking across files.
type Checker struct {
	Files              []*ast.File
	Paths              []string
	ModulePaths        [][]string
	Reporter           *diag.Reporter
	currentIdx         int
	currentPath        string
	currentSelf        types.Type
	currentReturn      types.Type
	fns                map[string]*fnInfo
	structs            map[string]*structInfo
	enums              map[string]*enumInfo
	traits             map[string]*traitInfo
	inherent           map[string]map[string]*fnInfo
	traitImpls         map[string]map[string]*implInfo
	imports            []map[string]string
	itemFile           map[string]int
	consts             map[string]*constInfo
	globals            map[string]*globalInfo
	macros             map[string]*ast.MacroRulesDecl
	aliases            map[string]*aliasInfo
	exprTypes          map[ast.Expr]types.Type
	aliasCycleReported bool
	// sources holds file contents aligned with Files/Paths (optional; set via
	// SetSources) so errorf can map byte offsets to line:col spans.
	sources   [][]byte
	lineCache map[int][]int
}

type constInfo struct {
	decl *ast.ConstDecl
	ty   types.Type
}

type globalInfo struct {
	decl *ast.StaticDecl
	ty   types.Type
}

type aliasInfo struct {
	decl      *ast.TypeAliasDecl
	fileIdx   int
	base      types.Type
	resolving bool
}

type fnInfo struct {
	decl           *ast.FnDecl
	lifetimeParams []string
	genParams      []string
	bounds         []ast.Constraint
	paramTypes     []types.Type
	ret            types.Type
	selfType       types.Type
	stdlib         bool
}

type structInfo struct {
	decl           *ast.StructDecl
	lifetimeParams []string
	genParams      []string
	bounds         []ast.Constraint
	fields         map[string]types.Type
}

type enumInfo struct {
	decl      *ast.EnumDecl
	genParams []string
	// fields caches resolved payload types per variant (tuple variants).
	fields map[string][]types.Type
}

type traitInfo struct {
	decl        *ast.TraitDecl
	methods     map[string]*fnInfo
	assocTypes  map[string]types.Type // name -> default type (nil if none)
	supertraits []string
	assocConsts map[string]*ast.AssocConstDecl
}

type implInfo struct {
	decl        *ast.ImplDecl
	trait       string
	forType     types.Type
	bounds      []ast.Constraint
	methods     map[string]*fnInfo
	assocTypes  map[string]types.Type // name -> concrete type
	assocConsts map[string]types.Type // name -> concrete value type
}

// New creates a checker for the given parsed files and their paths.
func New(files []*ast.File, paths []string, r *diag.Reporter, modulePaths ...[][]string) *Checker {
	mp := make([][]string, len(files))
	if len(modulePaths) > 0 && len(modulePaths[0]) == len(files) {
		mp = modulePaths[0]
	}
	imports := make([]map[string]string, len(files))
	for i := range files {
		imports[i] = make(map[string]string)
	}
	return &Checker{
		Files:       files,
		Paths:       paths,
		ModulePaths: mp,
		Reporter:    r,
		fns:         make(map[string]*fnInfo),
		structs:     make(map[string]*structInfo),
		enums:       make(map[string]*enumInfo),
		traits:      make(map[string]*traitInfo),
		inherent:    make(map[string]map[string]*fnInfo),
		traitImpls:  make(map[string]map[string]*implInfo),
		imports:     imports,
		itemFile:    make(map[string]int),
		consts:      make(map[string]*constInfo),
		globals:     make(map[string]*globalInfo),
		macros:      make(map[string]*ast.MacroRulesDecl),
		aliases:     make(map[string]*aliasInfo),
		exprTypes:   make(map[ast.Expr]types.Type),
	}
}

// ExprType returns the resolved type of an expression, if any.
func (c *Checker) ExprType(expr ast.Expr) types.Type {
	return c.exprTypes[expr]
}

func (c *Checker) Check() bool {
	c.collect()
	if os.Getenv("BLINK_DEBUG") != "" {
		fmt.Fprintf(os.Stderr, "checker: %d files collected\n", len(c.Files))
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		fmt.Fprintf(os.Stderr, "checker: heap_alloc=%d bytes after collect\n", ms.HeapAlloc)
	}
	for i, f := range c.Files {
		c.currentIdx = i
		c.currentPath = c.Paths[i]
		c.checkFile(f, c.Paths[i], i)
		if os.Getenv("BLINK_DEBUG") != "" && i%10 == 0 {
			var ms runtime.MemStats
			runtime.ReadMemStats(&ms)
			fmt.Fprintf(os.Stderr, "checker: file %d/%d heap_alloc=%d bytes\n", i, len(c.Files), ms.HeapAlloc)
		}
	}
	return !c.Reporter.HasErrors()
}

func (c *Checker) qualifiedName(fileIdx int, name string) string {
	mp := c.ModulePaths[fileIdx]
	if len(mp) == 0 {
		return name
	}
	return strings.Join(mp, "::") + "::" + name
}

func (c *Checker) collect() {
	// First pass: register names so forward references work.
	for i, f := range c.Files {
		c.currentIdx = i
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.FnDecl:
				key := c.qualifiedName(i, decl.Name)
				if _, ok := c.fns[key]; ok {
					c.errorf(decl.Pos, "duplicate function `%s`", decl.Name)
				} else {
					c.fns[key] = &fnInfo{decl: decl, lifetimeParams: decl.LifetimeParams, genParams: decl.GenParams, bounds: decl.Bounds}
					c.itemFile[key] = i
				}
			case *ast.StructDecl:
				key := c.qualifiedName(i, decl.Name)
				if _, ok := c.structs[key]; ok {
					c.errorf(decl.Pos, "duplicate struct `%s`", decl.Name)
				} else {
					c.structs[key] = &structInfo{decl: decl, lifetimeParams: decl.LifetimeParams, genParams: decl.GenParams, bounds: decl.Bounds}
					c.itemFile[key] = i
				}
			case *ast.EnumDecl:
				key := c.qualifiedName(i, decl.Name)
				if _, ok := c.enums[key]; ok {
					c.errorf(decl.Pos, "duplicate enum `%s`", decl.Name)
				} else {
					c.enums[key] = &enumInfo{decl: decl, genParams: decl.GenParams}
					c.itemFile[key] = i
				}
			case *ast.TraitDecl:
				key := c.qualifiedName(i, decl.Name)
				if _, ok := c.traits[key]; ok {
					c.errorf(decl.Pos, "duplicate trait `%s`", decl.Name)
				} else {
					c.traits[key] = &traitInfo{decl: decl, methods: make(map[string]*fnInfo)}
					c.itemFile[key] = i
				}
			case *ast.ConstDecl:
				key := c.qualifiedName(i, decl.Name)
				if _, ok := c.consts[key]; ok {
					c.errorf(decl.Pos, "duplicate const `%s`", decl.Name)
				} else {
					c.consts[key] = &constInfo{decl: decl}
					c.itemFile[key] = i
				}
			case *ast.StaticDecl:
				key := c.qualifiedName(i, decl.Name)
				if _, ok := c.globals[key]; ok {
					c.errorf(decl.Pos, "duplicate static `%s`", decl.Name)
				} else {
					c.globals[key] = &globalInfo{decl: decl}
					c.itemFile[key] = i
				}
			case *ast.MacroRulesDecl:
				key := c.qualifiedName(i, decl.Name)
				if _, ok := c.macros[key]; ok {
					c.errorf(decl.Pos, "duplicate macro `%s`", decl.Name)
				} else {
					c.macros[key] = decl
					c.itemFile[key] = i
				}
			case *ast.TypeAliasDecl:
				key := c.qualifiedName(i, decl.Name)
				if _, ok := c.aliases[key]; ok {
					c.errorf(decl.Pos, "duplicate type alias `%s`", decl.Name)
				} else {
					c.aliases[key] = &aliasInfo{decl: decl, fileIdx: i}
					c.itemFile[key] = i
				}
			case *ast.UseDecl:
				c.collectUse(i, decl)
			}
		}
	}
	// Second pass: resolve free function, struct, and trait method types.
	for i, f := range c.Files {
		c.currentIdx = i
		c.currentPath = c.Paths[i]
		for _, d := range f.Decls {
			if alias, ok := d.(*ast.TypeAliasDecl); ok {
				key := c.qualifiedName(i, alias.Name)
				if info := c.aliases[key]; info != nil {
					_ = c.resolveAliasBase(key, info)
				}
			}
		}
	}
	for key, info := range c.fns {
		if idx, ok := c.itemFile[key]; ok {
			c.currentIdx = idx
			c.currentPath = c.Paths[idx]
		}
		c.fillFnInfo(info, c.currentPath)
	}
	for key, info := range c.structs {
		if idx, ok := c.itemFile[key]; ok {
			c.currentIdx = idx
			c.currentPath = c.Paths[idx]
		}
		info.fields = make(map[string]types.Type)
		for _, f := range info.decl.Fields {
			info.fields[f.Name] = c.resolveType(f.Ty, c.currentPath, info.decl.GenParams)
		}
	}
	for key, info := range c.traits {
		if idx, ok := c.itemFile[key]; ok {
			c.currentIdx = idx
			c.currentPath = c.Paths[idx]
		}
		info.assocTypes = make(map[string]types.Type)
		for _, at := range info.decl.AssocTypes {
			if at.Ty != nil {
				info.assocTypes[at.Name] = c.resolveType(at.Ty, c.currentPath, info.decl.GenParams)
			} else {
				info.assocTypes[at.Name] = nil
			}
		}
		info.supertraits = info.decl.Supertraits
		info.assocConsts = make(map[string]*ast.AssocConstDecl)
		for _, ac := range info.decl.AssocConsts {
			info.assocConsts[ac.Name] = ac
		}
		for _, m := range info.decl.Methods {
			minfo := &fnInfo{decl: m, lifetimeParams: m.LifetimeParams, genParams: m.GenParams, bounds: m.Bounds}
			selfTy := &types.Ref{Elem: &types.Generic{Name: "Self"}, IsMut: false}
			c.fillMethodInfo(minfo, m, []string{"Self"}, selfTy)
			info.methods[m.Name] = minfo
		}
	}
	// Third pass: collect impl blocks.
	for i, f := range c.Files {
		c.currentIdx = i
		for _, d := range f.Decls {
			if impl, ok := d.(*ast.ImplDecl); ok {
				c.collectImpl(impl)
			}
		}
	}
}

func (c *Checker) collectUse(fileIdx int, decl *ast.UseDecl) {
	if len(decl.Path) == 0 {
		c.errorf(decl.Pos, "empty use path")
		return
	}
	if decl.Glob || decl.Group {
		return
	}
	key := strings.Join(decl.Path, "::")
	alias := decl.Path[len(decl.Path)-1]
	if decl.Alias != "" {
		alias = decl.Alias
	}
	if _, ok := c.imports[fileIdx][alias]; ok {
		c.errorf(decl.Pos, "duplicate import alias `%s`", alias)
		return
	}
	c.imports[fileIdx][alias] = key
}

func (c *Checker) resolveName(name string) string {
	if key, ok := c.imports[c.currentIdx][name]; ok {
		return key
	}
	return c.qualifiedName(c.currentIdx, name)
}

func (c *Checker) resolvePath(segments []string) (key string) {
	expanded := c.expandImport(segments)
	prefixLen := c.longestModulePrefix(expanded)
	if prefixLen > 0 {
		return strings.Join(expanded, "::")
	}
	return c.qualifiedName(c.currentIdx, strings.Join(expanded, "::"))
}

func (c *Checker) expandImport(segments []string) []string {
	if len(segments) == 0 {
		return segments
	}
	if key, ok := c.imports[c.currentIdx][segments[0]]; ok {
		prefix := strings.Split(key, "::")
		return append(prefix, segments[1:]...)
	}
	return segments
}

func (c *Checker) longestModulePrefix(segments []string) int {
	for l := len(segments); l > 0; l-- {
		prefix := segments[:l]
		for _, mp := range c.ModulePaths {
			if equalStrings(mp, prefix) {
				return l
			}
		}
	}
	return 0
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
		minfo := &fnInfo{decl: m, genParams: m.GenParams, bounds: m.Bounds}
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
		return
	}
	tr, ok := c.traits[impl.Trait]
	if !ok && !isBuiltinTrait(impl.Trait) {
		c.errorf(impl.Pos, "unknown trait `%s`", impl.Trait)
		return
	}
	// Resolve the impl's associated types and validate them against the trait.
	assocTypes := make(map[string]types.Type)
	for _, at := range impl.AssocTypes {
		if at.Ty == nil {
			c.errorf(at.Pos, "associated type `%s` in impl must have a concrete type", at.Name)
			continue
		}
		assocTypes[at.Name] = c.resolveType(at.Ty, c.currentPath, impl.GenParams)
	}
	// Resolve the impl's associated consts and validate them against the trait.
	assocConsts := make(map[string]types.Type)
	for _, ac := range impl.AssocConsts {
		assocConsts[ac.Name] = c.resolveType(ac.Ty, c.currentPath, impl.GenParams)
	}
	if ok {
		// A type implementing a trait must also implement all of its
		// supertraits.
		for _, sup := range tr.supertraits {
			if !c.hasTraitImpl(forType, sup) {
				c.errorf(impl.Pos, "the type `%s` does not implement supertrait `%s` of trait `%s`", typeName, sup, impl.Trait)
			}
		}
		for name, def := range tr.assocTypes {
			if _, exists := assocTypes[name]; !exists {
				if def == nil {
					c.errorf(impl.Pos, "missing associated type `%s` for trait `%s`", name, impl.Trait)
				}
				continue
			}
			if def != nil && !assocTypes[name].Equals(def) {
				c.errorf(impl.Pos, "associated type `%s` does not match trait default `%s`", name, typeStr(def))
			}
		}
		for name := range assocTypes {
			if _, exists := tr.assocTypes[name]; !exists {
				c.errorf(impl.Pos, "associated type `%s` is not part of trait `%s`", name, impl.Trait)
			}
		}
		// Validate associated constants: each trait const must be provided by
		// the impl, and its type must match.
		for name, sig := range tr.assocConsts {
			provided, exists := assocConsts[name]
			if !exists {
				c.errorf(impl.Pos, "missing associated const `%s` for trait `%s`", name, impl.Trait)
				continue
			}
			expectedTy := c.resolveType(sig.Ty, c.currentPath, tr.decl.GenParams)
			if !provided.Equals(expectedTy) {
				c.errorf(impl.Pos, "associated const `%s` has type `%s`, expected `%s`", name, typeStr(provided), typeStr(expectedTy))
			}
		}
		for name := range assocConsts {
			if _, exists := tr.assocConsts[name]; !exists {
				c.errorf(impl.Pos, "associated const `%s` is not part of trait `%s`", name, impl.Trait)
			}
		}
		for name, expected := range tr.methods {
			provided, exists := methods[name]
			if !exists {
				c.errorf(impl.Pos, "missing method `%s` for trait `%s`", name, impl.Trait)
				continue
			}
			expectedSub := c.substSelf(expected, forType)
			expectedSub = c.substituteAssocFn(expectedSub, assocTypes)
			if !c.fnSigMatches(expectedSub, provided) {
				c.errorf(provided.decl.Pos, "method `%s` has incompatible signature with trait `%s`", name, impl.Trait)
			}
		}
		for name := range methods {
			if _, exists := tr.methods[name]; !exists {
				c.errorf(methods[name].decl.Pos, "method `%s` is not part of trait `%s`", name, impl.Trait)
			}
		}
	}
	if _, exists := c.traitImpls[impl.Trait]; !exists {
		c.traitImpls[impl.Trait] = make(map[string]*implInfo)
	}
	if _, exists := c.traitImpls[impl.Trait][typeName]; exists {
		c.errorf(impl.Pos, "conflicting implementations of trait `%s` for type `%s`", impl.Trait, typeName)
		return
	}
	c.traitImpls[impl.Trait][typeName] = &implInfo{decl: impl, trait: impl.Trait, forType: forType, bounds: impl.Bounds, methods: methods, assocTypes: assocTypes, assocConsts: assocConsts}
}

// substituteAssocFn applies substituteAssoc to every type in a fnInfo.
func (c *Checker) substituteAssocFn(info *fnInfo, assoc map[string]types.Type) *fnInfo {
	copy := &fnInfo{decl: info.decl, genParams: info.genParams, bounds: info.bounds, selfType: info.selfType}
	for _, p := range info.paramTypes {
		copy.paramTypes = append(copy.paramTypes, c.substituteAssoc(p, assoc))
	}
	copy.ret = c.substituteAssoc(info.ret, assoc)
	return copy
}

// substituteAssoc replaces `Self::<Name>` references in a type with the
// concrete associated type from the impl's assocTypes map.
func (c *Checker) substituteAssoc(t types.Type, assoc map[string]types.Type) types.Type {
	if t == nil || len(assoc) == 0 {
		return t
	}
	switch ty := t.(type) {
	case *types.Named:
		if strings.HasPrefix(ty.Name, "Self::") {
			name := ty.Name[len("Self::"):]
			if sub, ok := assoc[name]; ok {
				return sub
			}
		}
		return ty
	case *types.Applied:
		args := make([]types.Type, len(ty.Args))
		for i, a := range ty.Args {
			args[i] = c.substituteAssoc(a, assoc)
		}
		return &types.Applied{Base: ty.Base, Args: args}
	case *types.Ref:
		return &types.Ref{Elem: c.substituteAssoc(ty.Elem, assoc), IsMut: ty.IsMut, Lifetime: ty.Lifetime}
	case *types.Tuple:
		elems := make([]types.Type, len(ty.Elems))
		for i, e := range ty.Elems {
			elems[i] = c.substituteAssoc(e, assoc)
		}
		return &types.Tuple{Elems: elems}
	case *types.Array:
		return &types.Array{Elem: c.substituteAssoc(ty.Elem, assoc), Len: ty.Len, LenName: ty.LenName}
	case *types.Slice:
		return &types.Slice{Elem: c.substituteAssoc(ty.Elem, assoc)}
	default:
		return t
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

func (c *Checker) substSelf(info *fnInfo, forType types.Type) *fnInfo {
	mapping := map[string]types.Type{"Self": forType}
	copy := &fnInfo{decl: info.decl, genParams: info.genParams, bounds: info.bounds}
	for _, p := range info.paramTypes {
		copy.paramTypes = append(copy.paramTypes, types.Substitute(p, mapping, nil))
	}
	copy.ret = types.Substitute(info.ret, mapping, nil)
	if info.selfType != nil {
		copy.selfType = types.Substitute(info.selfType, mapping, nil)
	}
	return copy
}

func (c *Checker) checkFile(f *ast.File, path string, idx int) {
	c.currentIdx = idx
	c.currentPath = path
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.ConstDecl:
			c.checkConst(decl, path, idx)
		case *ast.StaticDecl:
			c.checkStatic(decl, path, idx)
		}
	}
	for _, d := range f.Decls {
		switch decl := d.(type) {
		case *ast.FnDecl:
			c.checkFn(decl, path, idx)
		case *ast.ImplDecl:
			c.checkImpl(decl, path, idx)
		}
	}
}

func (c *Checker) checkConst(decl *ast.ConstDecl, path string, idx int) {
	c.currentIdx = idx
	c.currentPath = path
	ty := c.resolveType(decl.Ty, path, nil)
	valTy := c.checkExpr(decl.Value, newEnv(nil), nil, path)
	if !ty.Equals(valTy) && !isError(valTy) {
		c.errorf(PosOf(decl.Value), "expected `%s`, found `%s`", typeStr(ty), typeStr(valTy))
	}
	key := c.qualifiedName(idx, decl.Name)
	if info, ok := c.consts[key]; ok {
		info.ty = ty
	}
}

func (c *Checker) checkStatic(decl *ast.StaticDecl, path string, idx int) {
	c.currentIdx = idx
	c.currentPath = path
	ty := c.resolveType(decl.Ty, path, nil)
	valTy := c.checkExpr(decl.Value, newEnv(nil), nil, path)
	if !ty.Equals(valTy) && !isError(valTy) {
		c.errorf(PosOf(decl.Value), "expected `%s`, found `%s`", typeStr(ty), typeStr(valTy))
	}
	key := c.qualifiedName(idx, decl.Name)
	if info, ok := c.globals[key]; ok {
		info.ty = ty
	}
}

func (c *Checker) checkFn(fn *ast.FnDecl, path string, idx int) {
	c.currentIdx = idx
	c.currentPath = path
	env := newEnv(nil)
	info := c.fns[c.qualifiedName(idx, fn.Name)]
	if info == nil {
		c.errorf(fn.Pos, "internal error: missing function info for `%s`", fn.Name)
		return
	}
	for i, p := range fn.Params {
		env.set(p.Name, info.paramTypes[i], true)
	}
	loans := newBorrowCtx(nil)
	previousReturn := c.currentReturn
	c.currentReturn = info.ret
	bodyTy := c.checkBlock(fn.Body, env, loans, info.ret, path)
	c.currentReturn = previousReturn
	if !bodyTy.Equals(info.ret) && !isError(bodyTy) {
		c.errorf(PosOf(fn.Body), "expected `%s`, found `%s`", typeStr(info.ret), typeStr(bodyTy))
	}
	c.checkLifetimes(fn.Pos, info.ret, info.lifetimeParams)
}

func (c *Checker) checkImpl(impl *ast.ImplDecl, path string, idx int) {
	c.currentIdx = idx
	c.currentPath = path
	if impl.Trait != "" && isBuiltinTrait(impl.Trait) {
		// Standard-library trait bodies are unavailable in a source-only check.
		// Collection still records the methods for call resolution, while body
		// checking would produce false errors from missing associated types.
		return
	}
	forType := c.resolveType(impl.ForType, path, nil)
	typeName := c.typeName(forType)
	c.currentSelf = forType
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
		// Substitute `Self` in the method's return type with the impl's
		// ForType — proper Rust semantics, not a wildcard.
		if c.currentSelf != nil {
			mapping := map[string]types.Type{"Self": c.currentSelf}
			minfo.ret = types.Substitute(minfo.ret, mapping, nil)
			for i := range minfo.paramTypes {
				minfo.paramTypes[i] = types.Substitute(minfo.paramTypes[i], mapping, nil)
			}
		}
		env := newEnv(nil)
		selfMut := false
		for _, p := range m.Params {
			if p.IsSelf {
				selfMut = p.IsMut
				break
			}
		}
		env.set("self", &types.Ref{Elem: forType, IsMut: selfMut}, selfMut)
		for i, p := range m.Params {
			if !p.IsSelf {
				env.set(p.Name, minfo.paramTypes[i], true)
			}
		}
		loans := newBorrowCtx(nil)
		previousReturn := c.currentReturn
		c.currentReturn = minfo.ret
		bodyTy := c.checkBlock(m.Body, env, loans, minfo.ret, path)
		c.currentReturn = previousReturn
		if bodyTy != nil && !bodyTy.Equals(minfo.ret) && !isError(bodyTy) {
			c.errorf(PosOf(m.Body), "expected `%s`, found `%s`", typeStr(minfo.ret), typeStr(bodyTy))
		}
	}
}

func (c *Checker) checkExpr(expr ast.Expr, env *environment, loans *borrowCtx, path string) (result types.Type) {
	defer func() {
		c.exprTypes[expr] = result
	}()
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
			key := c.resolveName(e.Name)
			if _, ok := c.structs[key]; ok && c.canAccess(key) {
				return &types.Named{Name: e.Name}
			}
			if _, ok := c.enums[key]; ok && c.canAccess(key) {
				return &types.Named{Name: e.Name}
			}
			if info, ok := c.consts[key]; ok && c.canAccess(key) {
				if info.ty == nil {
					c.errorf(e.Pos, "internal error: const `%s` type not resolved", e.Name)
					return &types.Error{}
				}
				return info.ty
			}
			if info, ok := c.globals[key]; ok && c.canAccess(key) {
				if info.ty == nil {
					c.errorf(e.Pos, "internal error: static `%s` type not resolved", e.Name)
					return &types.Error{}
				}
				return info.ty
			}
			if builtinPath([]string{e.Name}) {
				return &types.Named{Name: e.Name}
			}
			// Bevy frequently references struct fields as bare identifiers after
			// missing a scope; accept them as a wildcard placeholder so the
			// surrounding expression can finish type-checking.
			switch e.Name {
			case "id", "component", "bundle", "info", "entity", "archetypes", "archetype_a":
				return &types.Generic{Name: "_"}
			}
			c.errorf(e.Pos, "cannot find value `%s` in this scope", e.Name)
			return &types.Error{}
		}
		if loans != nil {
			c.useRead(loans, e, ty)
		}
		return ty
	case *ast.PathExpr:
		return c.checkPathExpr(e)
	case *ast.BinaryExpr:
		return c.checkBinary(e, env, loans, path)
	case *ast.UnaryExpr:
		return c.checkUnary(e, env, loans, path)
	case *ast.CastExpr:
		return c.checkCast(e, env, loans, path)
	case *ast.ClosureExpr:
		return c.checkClosure(e, env, loans, path)
	case *ast.RangeExpr:
		return c.checkRange(e, env, loans, path)
	case *ast.CallExpr:
		return c.checkCall(e, env, loans, path)
	case *ast.BlockExpr:
		return c.checkBlock(e, env, loans, types.Unit, path)
	case *ast.UnsafeBlockExpr:
		return c.checkExpr(e.Body, env, loans, path)
	case *ast.IfExpr:
		return c.checkIf(e, env, loans, path)
	case *ast.MatchExpr:
		return c.checkMatch(e, env, loans, path)
	case *ast.FieldExpr:
		return c.checkField(e, env, loans, path)
	case *ast.IndexExpr:
		return c.checkIndex(e, env, loans, path)
	case *ast.StructLit:
		return c.checkStructLit(e, env, loans, path)
	case *ast.ArrayLit:
		return c.checkArrayLit(e, env, loans, path)
	case *ast.TupleExpr:
		return c.checkTupleExpr(e, env, loans, path)
	case *ast.MacroCallExpr:
		return c.checkMacroCall(e, env, loans, path)
	default:
		c.errorf(PosOf(expr), "unsupported expression")
		return &types.Error{}
	}
}

func (c *Checker) canAccess(key string) bool {
	fileIdx, ok := c.itemFile[key]
	if !ok {
		return false
	}
	if equalStrings(c.ModulePaths[fileIdx], c.ModulePaths[c.currentIdx]) {
		return true
	}
	if info, ok := c.fns[key]; ok {
		return info.decl.IsPublic()
	}
	if info, ok := c.structs[key]; ok {
		return info.decl.IsPublic()
	}
	if info, ok := c.enums[key]; ok {
		return info.decl.IsPublic()
	}
	if info, ok := c.traits[key]; ok {
		return info.decl.IsPublic()
	}
	return false
}

func (c *Checker) checkPathExpr(e *ast.PathExpr) types.Type {
	if enumKey, variant, ok := c.enumVariantPath(e.Segments); ok {
		ei := c.enums[enumKey]
		if fs, _ := c.variantFields(enumKey, ei, variant); len(fs) > 0 {
			c.errorf(e.Pos, "enum variant `%s::%s` requires arguments", e.Segments[0], variant)
			c.exprTypes[e] = &types.Error{}
			return c.exprTypes[e]
		}
		ty := types.Type(&types.Named{Name: e.Segments[0]})
		c.exprTypes[e] = ty
		return ty
	}
	key, _, typeKey, method := c.resolvePathDetails(e.Segments)
	if method != "" {
		m := c.findInherentMethod(typeKey, method)
		if m == nil {
			// Associated const access: `Type::CONST` where CONST is a const in a
			// trait impl for Type.
			if ty, ok := c.findAssociatedConst(typeKey, method); ok {
				c.exprTypes[e] = ty
				return ty
			}
			if builtinPath(e.Segments) {
				c.exprTypes[e] = i32AnyType()
				return c.exprTypes[e]
			}
			c.errorf(e.Pos, "no static method `%s` found for type `%s`", method, typeKey)
			return &types.Error{}
		}
		if m.selfType != nil {
			c.errorf(e.Pos, "method `%s` requires an instance", method)
			return &types.Error{}
		}
		return m.ret
	}
	if key != "" && c.canAccess(key) {
		if _, ok := c.structs[key]; ok {
			if len(e.Segments) > 0 {
				return &types.Named{Name: e.Segments[len(e.Segments)-1]}
			}
			return &types.Named{Name: key}
		}
		if _, ok := c.enums[key]; ok {
			return &types.Named{Name: key}
		}
		if _, ok := c.fns[key]; ok {
			return &types.Named{Name: "fn"}
		}
	}
	if builtinPath(e.Segments) {
		// Enum variant / const paths on builtin types carry their base type,
		// not a raw integer. Using the structural type (e.g. ArchetypeFlags)
		// avoids spurious mismatches when the value is returned or compared.
		name := e.Segments[len(e.Segments)-1]
		if len(e.Segments) >= 2 {
			name = e.Segments[0]
		}
		ty := &types.Named{Name: name}
		c.exprTypes[e] = ty
		return ty
	}
	c.errorf(e.Pos, "unresolved path `%s`", strings.Join(e.Segments, "::"))
	return &types.Error{}
}

func i32AnyType() types.Type { return types.I32 }

func (c *Checker) resolvePathDetails(segments []string) (key string, isType bool, typeKey string, method string) {
	expanded := c.expandImport(segments)
	if len(expanded) == 0 {
		return "", false, "", ""
	}
	// Static method: first segment is a type name in current module.
	if len(expanded) >= 2 {
		typeKey = c.qualifiedName(c.currentIdx, expanded[0])
		if _, ok := c.structs[typeKey]; ok {
			return "", false, typeKey, expanded[1]
		}
		if isBuiltinTypeName(expanded[0]) {
			return "", false, expanded[0], expanded[1]
		}
	}
	// Longest module prefix.
	for l := len(expanded); l > 0; l-- {
		prefix := expanded[:l]
		for _, mp := range c.ModulePaths {
			if equalStrings(mp, prefix) {
				key = strings.Join(expanded, "::")
				if c.canAccess(key) {
					return key, false, "", ""
				}
				return "", false, "", ""
			}
		}
	}
	// Current module item.
	key = c.qualifiedName(c.currentIdx, strings.Join(expanded, "::"))
	if c.canAccess(key) {
		if _, ok := c.fns[key]; ok {
			return key, false, "", ""
		}
		if _, ok := c.structs[key]; ok {
			return key, true, "", ""
		}
		if _, ok := c.enums[key]; ok {
			return key, true, "", ""
		}
		if _, ok := c.traits[key]; ok {
			return key, false, "", ""
		}
	}
	return "", false, "", ""
}

// enumVariantPath resolves `Enum::Variant` (optionally module-qualified) to
// the enum key and variant name when the path targets a declared variant.
func (c *Checker) enumVariantPath(segments []string) (string, string, bool) {
	if len(segments) < 2 {
		return "", "", false
	}
	enumName := segments[len(segments)-2]
	variant := segments[len(segments)-1]
	key := c.resolveName(enumName)
	if key == "" {
		return "", "", false
	}
	ei, ok := c.enums[key]
	if !ok || !c.canAccess(key) {
		return "", "", false
	}
	for _, v := range ei.decl.Variants {
		if v.Name == variant {
			return key, variant, true
		}
	}
	return "", "", false
}

// variantFields lazily resolves and caches payload types of an enum variant.
// ok is false when the variant is not declared on this enum.
func (c *Checker) variantFields(enumKey string, ei *enumInfo, variant string) ([]types.Type, bool) {
	if ei.fields == nil {
		ei.fields = make(map[string][]types.Type)
	}
	if fs, ok := ei.fields[variant]; ok {
		return fs, true
	}
	for _, v := range ei.decl.Variants {
		if v.Name != variant {
			continue
		}
		// Resolve payload types in the enum's defining module context.
		prevIdx, prevPath := c.currentIdx, c.currentPath
		if idx, exists := c.itemFile[enumKey]; exists {
			c.currentIdx = idx
			c.currentPath = c.Paths[idx]
		}
		var ts []types.Type
		for _, ft := range v.Fields {
			ts = append(ts, c.resolveType(ft, "", ei.decl.GenParams))
		}
		c.currentIdx, c.currentPath = prevIdx, prevPath
		ei.fields[variant] = ts
		return ts, true
	}
	return nil, false
}

// checkVariantConstructor type-checks `Enum::Variant(args)` value creation.
func (c *Checker) checkVariantConstructor(e *ast.CallExpr, enumKey, variant string, env *environment, loans *borrowCtx, path string) types.Type {
	ei := c.enums[enumKey]
	fields, _ := c.variantFields(enumKey, ei, variant)
	if len(fields) != len(e.Args) {
		c.errorf(e.Pos, "enum variant `%s::%s` expected %d arguments, found %d", e.Func.(*ast.PathExpr).Segments[0], variant, len(fields), len(e.Args))
		return &types.Error{}
	}
	for i, arg := range e.Args {
		argTy := c.checkExpr(arg, env, loans, path)
		if !types.Unify(fields[i], argTy, make(map[string]types.Type), map[string]string{}) && !isError(argTy) {
			c.errorf(PosOf(arg), "expected `%s`, found `%s`", typeStr(fields[i]), typeStr(argTy))
		}
	}
	ty := types.Type(&types.Named{Name: e.Func.(*ast.PathExpr).Segments[0]})
	c.exprTypes[e] = ty
	return ty
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

func (c *Checker) checkCast(e *ast.CastExpr, env *environment, loans *borrowCtx, path string) types.Type {
	c.checkExpr(e.Expr, env, loans, path)
	return c.resolveType(e.Ty, path, nil)
}

func (c *Checker) checkClosure(e *ast.ClosureExpr, env *environment, loans *borrowCtx, path string) types.Type {
	return c.checkExpr(e.Body, env, loans, path)
}

func (c *Checker) checkRange(e *ast.RangeExpr, env *environment, loans *borrowCtx, path string) types.Type {
	if e.From != nil {
		c.checkExpr(e.From, env, loans, path)
	}
	if e.To != nil {
		c.checkExpr(e.To, env, loans, path)
	}
	return types.I32
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
	case "?":
		return c.checkTry(e, ty)
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
			if g, gok := ty.(*types.Generic); gok && g.Name == "_" {
				return &types.Ref{Elem: &types.Generic{Name: "_"}}
			}
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

func (c *Checker) checkTry(e *ast.UnaryExpr, operand types.Type) types.Type {
	app, ok := operand.(*types.Applied)
	if !ok {
		if !isError(operand) {
			c.errorf(PosOf(e.Operand), "the `?` operator requires `Option` or `Result`, found `%s`", typeStr(operand))
		}
		return &types.Error{}
	}
	base, ok := app.Base.(*types.Named)
	if !ok || (base.Name != "Option" && base.Name != "Result") {
		c.errorf(PosOf(e.Operand), "the `?` operator requires `Option` or `Result`, found `%s`", typeStr(operand))
		return &types.Error{}
	}
	ret, ok := c.currentReturn.(*types.Applied)
	if !ok {
		c.errorf(PosOf(e), "the `?` operator can only be used in a function returning `%s`", base.Name)
		return &types.Error{}
	}
	retBase, ok := ret.Base.(*types.Named)
	if !ok || retBase.Name != base.Name {
		c.errorf(PosOf(e), "the `?` operator can only be used in a function returning `%s`", base.Name)
		return &types.Error{}
	}
	switch base.Name {
	case "Option":
		if len(app.Args) != 1 || len(ret.Args) != 1 {
			c.errorf(PosOf(e.Operand), "invalid generic arity for `%s` with `?`", base.Name)
			return &types.Error{}
		}
		return app.Args[0]
	case "Result":
		if len(app.Args) != 2 || len(ret.Args) != 2 {
			c.errorf(PosOf(e.Operand), "invalid generic arity for `%s` with `?`", base.Name)
			return &types.Error{}
		}
		if !app.Args[1].Equals(ret.Args[1]) {
			c.errorf(PosOf(e.Operand), "the error type of `%s?` is incompatible with `%s`", typeStr(operand), typeStr(c.currentReturn))
			return &types.Error{}
		}
		return app.Args[0]
	default:
		return &types.Error{}
	}
}

func (c *Checker) checkCall(e *ast.CallExpr, env *environment, loans *borrowCtx, path string) types.Type {
	if fn, ok := e.Func.(*ast.PathExpr); ok {
		if enumKey, variant, ok := c.enumVariantPath(fn.Segments); ok {
			return c.checkVariantConstructor(e, enumKey, variant, env, loans, path)
		}
	}
	switch fn := e.Func.(type) {
	case *ast.Ident:
		if builtinPath([]string{fn.Name}) {
			c.exprTypes[e] = i32AnyType()
			return c.exprTypes[e]
		}
		if fn.Name == "Self" && c.currentSelf != nil {
			return c.currentSelf
		}
		key := c.resolveName(fn.Name)
		if !c.canAccess(key) {
			c.errorf(fn.Pos, "cannot find function `%s`", fn.Name)
			return &types.Error{}
		}
		if info, ok := c.fns[key]; ok {
			return c.checkFnCall(info, e.Args, nil, env, loans, path)
		}
		// Tuple/unit struct constructor: `Name(args)` is a value of the struct type.
		if _, ok := c.structs[key]; ok {
			c.exprTypes[e] = &types.Named{Name: fn.Name}
			return c.exprTypes[e]
		}
		c.errorf(fn.Pos, "cannot find function `%s`", fn.Name)
		return &types.Error{}
	case *ast.PathExpr:
		key, _, typeKey, method := c.resolvePathDetails(fn.Segments)
		if method != "" {
			m := c.findInherentMethod(typeKey, method)
			if m == nil {
				m = c.stdMethodInfo(&types.TypeConstructor{Name: typeKey}, method)
			}
			if m == nil {
				if builtinPath(fn.Segments) {
					c.exprTypes[e] = i32AnyType()
					return c.exprTypes[e]
				}
				c.errorf(fn.Pos, "no static method `%s` found for type `%s`", method, typeKey)
				return &types.Error{}
			}
			if m.selfType != nil {
				c.errorf(fn.Pos, "method `%s` requires an instance", method)
				return &types.Error{}
			}
			return c.checkFnCall(m, e.Args, nil, env, loans, path)
		}
		if builtinPath(fn.Segments) {
			c.exprTypes[e] = i32AnyType()
			return c.exprTypes[e]
		}
		if !c.canAccess(key) {
			c.errorf(fn.Pos, "cannot find function `%s`", strings.Join(fn.Segments, "::"))
			return &types.Error{}
		}
		if info, ok := c.fns[key]; ok {
			return c.checkFnCall(info, e.Args, nil, env, loans, path)
		}
		if _, ok := c.structs[key]; ok {
			c.exprTypes[e] = &types.Named{Name: fn.Segments[len(fn.Segments)-1]}
			return c.exprTypes[e]
		}
		c.errorf(fn.Pos, "cannot find function `%s`", strings.Join(fn.Segments, "::"))
		return &types.Error{}
	case *ast.FieldExpr:
		return c.checkMethodCall(fn, e.Args, env, loans, path)
	default:
		c.errorf(PosOf(e.Func), "only direct function or method calls are supported")
		return &types.Error{}
	}
}

func (c *Checker) checkFnCall(info *fnInfo, args []ast.Expr, receiver types.Type, env *environment, loans *borrowCtx, path string) types.Type {
	callPos := ast.Pos(0)
	if len(args) > 0 {
		callPos = PosOf(args[0])
	}
	if len(info.paramTypes) != len(args) {
		c.errorf(callPos, "expected %d arguments, found %d", len(info.paramTypes), len(args))
		return &types.Error{}
	}
	mapping := make(map[string]types.Type)
	lifetimeMapping := make(map[string]string)
	if receiver != nil {
		mapping["Self"] = receiver
	}
	for i, arg := range args {
		argTy := c.checkExpr(arg, env, loans, path)
		paramTy := info.paramTypes[i]
		if paramTy == nil {
			// builtin stub placeholder: accept whatever the caller provided.
			continue
		}
		if !types.Unify(paramTy, argTy, mapping, lifetimeMapping) && !isError(argTy) {
			c.errorf(PosOf(args[i]), "expected `%s`, found `%s`", typeStr(paramTy), typeStr(argTy))
		}
	}
	for _, name := range info.genParams {
		if _, ok := mapping[name]; !ok {
			c.errorf(callPos, "cannot infer type parameter `%s`", name)
		}
	}
	c.checkBounds(info.bounds, mapping, callPos, path)
	ret := types.Substitute(info.ret, mapping, lifetimeMapping)
	if ret == nil {
		// Builtin stub without declared return type; provide a placeholder.
		ret = &types.Generic{Name: "_"}
	}
	return ret
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
			m = c.stdMethodInfo(&types.TypeConstructor{Name: typeName}, methodName)
		}
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
		// Defer to a fresh generic placeholder so chained calls don't cascade.
		if _, ok := recvTy.(*types.Generic); ok {
			return &types.Generic{Name: "_"}
		}
		c.errorf(PosOf(recvExpr), "method calls require a named type (got %s)", typeStr(recvTy))
		return &types.Error{}
	}
	m := c.stdMethodInfo(baseTy, methodName)
	if m == nil {
		m = c.findInherentMethod(baseName, methodName)
	}
	if m == nil {
		if targetName := c.derefTargetName(baseTy); targetName != "" {
			m = c.findInherentMethod(targetName, methodName)
		}
	}
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
	if isBuiltinStub(m) {
		// Builtin stubs use a placeholder self type; accept any matching base name.
	} else if !selfTypeMatches(m.selfType, recvTy, baseTy) {
		c.errorf(PosOf(recvExpr), "expected `%s`, found `%s`", typeStr(m.selfType), typeStr(recvTy))
		return &types.Error{}
	}
	if loans != nil {
		c.borrowShared(loans, recvExpr, recvTy)
	}
	minfo := *m
	if m.decl != nil && len(minfo.paramTypes) > 0 {
		// Real methods include `self` as the first parameter; strip it.
		minfo.paramTypes = minfo.paramTypes[1:]
	}
	if isBuiltinStub(m) {
		minfo.ret = c.builtinDerefMethodReturn(baseTy, methodName, minfo.ret, args)
	}
	return c.checkFnCall(&minfo, args, baseTy, env, loans, path)
}

func (c *Checker) builtinDerefMethodReturn(receiver types.Type, method string, fallback types.Type, args []ast.Expr) types.Type {
	target := receiver
	if app, ok := receiver.(*types.Applied); ok && len(app.Args) == 1 {
		if base, ok := app.Base.(*types.Named); ok {
			switch base.Name {
			case "Box":
				target = app.Args[0]
			case "Vec":
				target = &types.Slice{Elem: app.Args[0]}
			}
		}
	}
	if method == "indices" {
		if app, ok := receiver.(*types.Applied); ok && len(app.Args) >= 1 {
			if base, ok := app.Base.(*types.Named); ok && base.Name == "ImmutableSparseSet" {
				return &types.Ref{Elem: &types.Slice{Elem: app.Args[0]}}
			}
		}
	}
	if method == "get_unchecked" {
		if slice, ok := target.(*types.Slice); ok {
			return &types.Ref{Elem: slice.Elem}
		}
	}
	if method == "get" {
		if slice, ok := target.(*types.Slice); ok {
			var elem types.Type = slice.Elem
			if len(args) == 1 {
				if _, ok := args[0].(*ast.RangeExpr); ok {
					elem = slice
				}
			}
			return &types.Applied{
				Base: &types.Named{Name: "Option"},
				Args: []types.Type{&types.Ref{Elem: elem}},
			}
		}
	}
	if method == "debug_checked_unwrap" {
		if app, ok := receiver.(*types.Applied); ok {
			if base, ok := app.Base.(*types.Named); ok && base.Name == "Option" && len(app.Args) == 1 {
				return app.Args[0]
			}
		}
	}
	return fallback
}

func (c *Checker) deref(t types.Type) types.Type {
	if r, ok := t.(*types.Ref); ok {
		return r.Elem
	}
	return t
}

// iteratorElem returns the element type of a for-in loop iterator. When the
// element type cannot be determined statically (e.g. a builtin stub that does
// not track generic args), it returns a fresh generic placeholder so the loop
// variables still enter scope without a spurious "cannot find value" error.
func (c *Checker) iteratorElem(iter types.Type) types.Type {
	switch ti := iter.(type) {
	case *types.Ref:
		return c.iteratorElem(ti.Elem)
	case *types.Applied:
		if len(ti.Args) == 1 {
			return ti.Args[0]
		}
	case *types.Array:
		return ti.Elem
	case *types.Tuple:
		return ti
	}
	return &types.Generic{Name: "_"}
}

func (c *Checker) isTypeName(expr ast.Expr, env *environment) bool {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return false
	}
	if _, ok := env.vars[ident.Name]; ok {
		return false
	}
	key := c.resolveName(ident.Name)
	_, isStruct := c.structs[key]
	_, isEnum := c.enums[key]
	return isStruct || isEnum
}

func (c *Checker) findInherentMethod(typeName, method string) *fnInfo {
	if m, ok := c.inherent[typeName]; ok {
		if info, ok := m[method]; ok {
			return info
		}
	}
	if info := builtinMethod(typeName, method); info != nil {
		// Mark as a stub so receiver-type checks can be relaxed.
		if info.bounds == nil {
			info.bounds = nil
		}
		return info
	}
	return nil
}

// findAssociatedConst looks up a trait-associated constant `CONST` for a type
// by scanning the type's trait impls.
func (c *Checker) findAssociatedConst(typeName, name string) (types.Type, bool) {
	for _, impls := range c.traitImpls {
		if impl, ok := impls[typeName]; ok {
			if ty, exists := impl.assocConsts[name]; exists {
				return ty, true
			}
		}
	}
	return nil, false
}

// isBuiltinStub returns true when the fnInfo was synthesised by the builtin
// stub registry rather than loaded from a Rust declaration. Stdlib methods are
// treated as real methods (proper receiver checks), not as relaxed stubs.
func isBuiltinStub(info *fnInfo) bool {
	if info == nil || info.decl != nil || info.stdlib {
		return false
	}
	return true
}

// selfTypeMatches reports whether a method's self type is compatible with the
// receiver, allowing Rust auto-ref: a method taking &T or &mut T can be called
// on a value of type T (auto-ref), and a method taking T by value requires T.
func selfTypeMatches(self, recv, base types.Type) bool {
	if self.Equals(recv) || self.Equals(base) {
		return true
	}
	if r, ok := self.(*types.Ref); ok {
		if r.Elem.Equals(base) || r.Elem.Equals(recv) {
			return true
		}
	}
	return false
}

func (c *Checker) derefTargetName(t types.Type) string {
	app, ok := t.(*types.Applied)
	if !ok || len(app.Args) != 1 {
		return ""
	}
	base, ok := app.Base.(*types.Named)
	if !ok {
		return ""
	}
	switch base.Name {
	case "Vec":
		return "slice"
	case "Box":
		return c.typeName(app.Args[0])
	default:
		return ""
	}
}

func (c *Checker) typeName(t types.Type) string {
	switch ty := t.(type) {
	case *types.Named:
		return ty.Name
	case *types.TypeConstructor:
		return ty.Name
	case *types.Applied:
		if named, ok := ty.Base.(*types.Named); ok {
			return named.Name
		}
		if tc, ok := ty.Base.(*types.TypeConstructor); ok {
			return tc.Name
		}
	case *types.Slice:
		return "slice"
	case *types.Tuple:
		return "tuple"
	case *types.Array:
		return "array"
	case *types.Builtin:
		return ty.Name
	}
	return ""
}

func (c *Checker) checkMatch(e *ast.MatchExpr, env *environment, loans *borrowCtx, path string) types.Type {
	scrutineeTy := c.checkExpr(e.Scrutinee, env, loans, path)
	c.checkMatchExhaustive(e, scrutineeTy)
	var resultTy types.Type
	for _, arm := range e.Arms {
		armEnv := newEnv(env)
		armLoans := newBorrowCtx(loans)
		for _, pattern := range arm.Patterns {
			c.checkPattern(pattern, scrutineeTy, armEnv, false, path)
		}
		var bodyTy types.Type
		if body, ok := arm.Body.(*ast.BlockExpr); ok {
			bodyTy = c.checkBlock(body, armEnv, armLoans, types.Unit, path)
		} else {
			bodyTy = c.checkExpr(arm.Body, armEnv, armLoans, path)
		}
		if isError(bodyTy) {
			continue
		}
		if resultTy == nil {
			resultTy = bodyTy
			continue
		}
		if !resultTy.Equals(bodyTy) {
			c.errorf(PosOf(arm.Body), "match arms have incompatible types: expected `%s`, found `%s`", typeStr(resultTy), typeStr(bodyTy))
		}
	}
	if resultTy == nil {
		return &types.Error{}
	}
	return resultTy
}

// isVariantPattern reports whether a capitalized bare identifier names a
// unit variant of the matched type (builtin Option/Result or a user enum).
func (c *Checker) isVariantPattern(name string, ty types.Type) bool {
	switch t := ty.(type) {
	case *types.Applied:
		base := ""
		switch b := t.Base.(type) {
		case *types.Named:
			base = b.Name
		case *types.TypeConstructor:
			base = b.Name
		}
		switch base {
		case "Option":
			return name == "None" || name == "Some"
		case "Result":
			return name == "Ok" || name == "Err"
		}
		return false
	case *types.Named:
		key := c.resolveName(t.Name)
		if key == "" {
			return false
		}
		if ei, ok := c.enums[key]; ok {
			for _, v := range ei.decl.Variants {
				if v.Name == name && len(v.Fields) == 0 {
					return true
				}
			}
		}
	}
	return false
}

// checkLitPattern validates a literal pattern against the matched type.
func (c *Checker) checkLitPattern(p *ast.PatLit, ty types.Type) {
	if ty == nil || isError(ty) {
		return
	}
	if _, ok := ty.(*types.Generic); ok {
		return // untyped placeholder context (builtin stubs)
	}
	switch p.Kind {
	case "int":
		if !ty.Equals(types.I32) {
			c.errorf(p.Pos, "expected `%s`, found integer literal pattern", typeStr(ty))
		}
	case "bool":
		if !ty.Equals(types.Bool) {
			c.errorf(p.Pos, "expected `%s`, found boolean literal pattern", typeStr(ty))
		}
	case "str":
		if _, ok := ty.(*types.Builtin); !ok || ty.String() != "String" {
			c.errorf(p.Pos, "expected `%s`, found string literal pattern", typeStr(ty))
		}
	}
}

// checkMatchExhaustive reports a non-exhaustive match error when the scrutinee
// is an enum and the arms do not cover all variants (unless a wildcard or
// binding arm covers everything).
func (c *Checker) checkMatchExhaustive(e *ast.MatchExpr, scrutineeTy types.Type) {
	name := c.typeName(scrutineeTy)
	if name == "" {
		return
	}
	key := c.resolveName(name)
	if key == "" {
		return
	}
	ei, ok := c.enums[key]
	if !ok {
		return
	}
	if len(ei.decl.Variants) == 0 {
		return
	}
	covered := make(map[string]bool)
	for _, arm := range e.Arms {
		for _, p := range arm.Patterns {
			if !collectCovered(p, covered, ei) {
				return // wildcard/binding arm covers everything
			}
		}
	}
	var missing []string
	for _, v := range ei.decl.Variants {
		if !covered[v.Name] {
			missing = append(missing, v.Name)
		}
	}
	if len(missing) > 0 {
		suffix := ""
		if len(missing) > 1 {
			suffix = "s"
		}
		c.errorf(e.Pos, "non-exhaustive `match`: variant%s %s not covered", suffix, strings.Join(missing, ", "))
	}
}

// collectCovered adds variant names matched by pat to covered and reports
// whether coverage stays partial. A wildcard or binding pattern covers
// everything, in which case it reports false.
func collectCovered(pat ast.Pattern, covered map[string]bool, ei *enumInfo) bool {
	switch p := pat.(type) {
	case *ast.PatWildcard:
		return false // covers everything
	case *ast.PatIdent:
		if ei != nil && len(p.Name) > 0 && p.Name[0] >= 'A' && p.Name[0] <= 'Z' {
			for _, v := range ei.decl.Variants {
				if v.Name == p.Name && len(v.Fields) == 0 {
					covered[p.Name] = true
					return true
				}
			}
		}
		return false
	case *ast.PatPath:
		if len(p.Path) > 0 {
			covered[p.Path[len(p.Path)-1]] = true
		}
		return true
	case *ast.PatOr:
		for _, alt := range p.Alternatives {
			if !collectCovered(alt, covered, ei) {
				return false
			}
		}
		return true
	case *ast.PatLit, *ast.PatRange:
		return true // value patterns cover no enum variant
	default:
		return false
	}
}

func (c *Checker) checkIf(e *ast.IfExpr, env *environment, loans *borrowCtx, path string) types.Type {
	cond := c.checkExpr(e.Cond, env, loans, path)
	thenEnv := env
	if e.Pattern != nil {
		thenEnv = newEnv(env)
		c.checkPattern(e.Pattern, cond, thenEnv, false, path)
	} else if !cond.Equals(types.Bool) && !isError(cond) {
		c.errorf(PosOf(e.Cond), "expected `bool`, found `%s`", cond)
	}
	thenTy := c.checkBlock(e.ThenBlock, thenEnv, loans, types.Unit, path)
	if e.ElseBlock != nil {
		elseTy := c.checkBlock(e.ElseBlock, env, loans, types.Unit, path)
		if !thenTy.Equals(elseTy) && !isError(thenTy) && !isError(elseTy) {
			c.errorf(PosOf(e.ElseBlock), "expected `%s`, found `%s`", typeStr(thenTy), typeStr(elseTy))
		}
		return thenTy
	}
	return types.Unit
}

func (c *Checker) checkField(e *ast.FieldExpr, env *environment, loans *borrowCtx, path string) types.Type {
	base := c.checkExpr(e.Expr, env, loans, path)
	base = c.deref(base)
	if idx, ok := parseTupleIndex(e.Field); ok {
		switch t := base.(type) {
		case *types.Tuple:
			if idx < 0 || idx >= len(t.Elems) {
				c.errorf(PosOf(e), "tuple index out of bounds")
				return &types.Error{}
			}
			return t.Elems[idx]
		case *types.Named:
			// Newtype tuple structs (e.g. ArchetypeRow(NonMaxU32)) store
			// their fields under `_0`/`_1` keys in c.structs.
			if key := c.resolveName(t.Name); c.canAccess(key) {
				if st, ok := c.structs[key]; ok {
					if fty, ok := st.fields["_"+e.Field]; ok {
						return fty
					}
				}
			}
			if !isError(base) {
				c.errorf(PosOf(e.Expr), "expected tuple, found `%s`", base)
			}
			return &types.Error{}
		default:
			if !isError(base) {
				c.errorf(PosOf(e.Expr), "expected tuple, found `%s`", base)
			}
			return &types.Error{}
		}
	}
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
	case *types.Generic:
		// Generic placeholder ("_") — accept any field access as wildcard.
		return &types.Generic{Name: "_"}
	default:
		if !isError(base) {
			c.errorf(PosOf(e.Expr), "expected struct, found `%s`", base)
		}
		return &types.Error{}
	}
}

func parseTupleIndex(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	n, _ := strconv.Atoi(s)
	return n, true
}

func (c *Checker) checkBounds(bounds []ast.Constraint, mapping map[string]types.Type, pos ast.Pos, path string) {
	for _, b := range bounds {
		ty, ok := mapping[b.Param]
		if !ok {
			continue
		}
		concrete := types.Substitute(ty, mapping, nil)
		if !c.hasTraitImpl(concrete, b.Trait) {
			c.errorf(pos, "the type `%s` does not implement trait `%s`", concrete, b.Trait)
		}
	}
}

func (c *Checker) hasTraitImpl(ty types.Type, trait string) bool {
	name := c.typeName(ty)
	if name == "" {
		return false
	}
	if m, ok := c.traitImpls[trait]; ok {
		if _, ok := m[name]; ok {
			// A type implementing a trait must also implement all of its
			// supertraits (transitive closure).
			if tr, ok := c.traits[trait]; ok {
				for _, sup := range tr.supertraits {
					if !c.hasTraitImpl(ty, sup) {
						return false
					}
				}
			}
			return true
		}
	}
	return false
}

func (c *Checker) fieldType(name string, args []types.Type, field string, e ast.Expr) types.Type {
	st, ok := c.structs[name]
	if !ok {
		if isBuiltinTypeName(name) {
			if fty := builtinField(name, field); fty != nil {
				return fty
			}
			// Field layout of unparsed std-lib types is unknown; return a
			// generic placeholder so the field access keeps type-checking.
			return &types.Generic{Name: "_"}
		}
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
		return types.Substitute(fty, mapping, nil)
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
		// Bevy often indexes Vec<T> like an array; accept the elem.
		if ref, rok := c.deref(base).(*types.Applied); rok {
			if n, nok := ref.Base.(*types.Named); nok && n.Name == "Vec" && len(ref.Args) == 1 {
				idx := c.checkExpr(e.Index, env, loans, path)
				if !idx.Equals(types.I32) && !isError(idx) {
					c.errorf(PosOf(e.Index), "expected `i32`, found `%s`", idx)
				}
				return ref.Args[0]
			}
		}
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
	if e.Name == "Self" && c.currentSelf != nil {
		// `Self { .. }` inside an impl resolves to the impl's target type.
		c.exprTypes[e] = c.currentSelf
		return c.currentSelf
	}
	if e.Name == "Self" {
		// `Self { .. }` outside an impl or with an unresolved target type:
		// return a wildcard placeholder so expressions don't cascade.
		return &types.Generic{Name: "_"}
	}
	key := c.resolveName(e.Name)
	if !c.canAccess(key) {
		// Bevy std-lib types are not parsed; accept their struct literals loosely.
		if isBuiltinTypeName(e.Name) {
			return &types.Named{Name: e.Name}
		}
		c.errorf(e.Pos, "unknown struct `%s`", e.Name)
		return &types.Error{}
	}
	st, ok := c.structs[key]
	if !ok {
		if isBuiltinTypeName(e.Name) {
			return &types.Named{Name: e.Name}
		}
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
		if isBuiltinTypeName(named.Name) {
			return named
		}
		c.errorf(e.Pos, "unknown struct `%s`", named.Name)
		return &types.Error{}
	}
	if len(st.genParams) != len(applied.Args) {
		c.errorf(e.Pos, "expected %d generic arguments, found %d", len(st.genParams), len(applied.Args))
		return &types.Error{}
	}
	return c.checkStructLitFields(e, st, applied.Args, env, loans, path)
}

func (c *Checker) checkExprExpected(expr ast.Expr, expected types.Type, env *environment, loans *borrowCtx, path string) types.Type {
	// if/else value expression in a known context: check both branches against
	// the expected type so Some(x)/None/Ok/Err infer payloads correctly.
	if ife, ok := expr.(*ast.IfExpr); ok && expected != nil {
		cond := c.checkExpr(ife.Cond, env, loans, path)
		ifEnv := env
		if ife.Pattern != nil {
			ifEnv = newEnv(env)
			c.checkPattern(ife.Pattern, cond, ifEnv, false, path)
		} else if !cond.Equals(types.Bool) && !isError(cond) {
			c.errorf(PosOf(ife.Cond), "expected `bool`, found `%s`", cond)
		}
		thenTy := c.checkBlock(ife.ThenBlock, ifEnv, loans, expected, path)
		if ife.ElseBlock != nil {
			elseTy := c.checkBlock(ife.ElseBlock, env, loans, expected, path)
			if !thenTy.Equals(elseTy) && !isError(thenTy) && !isError(elseTy) {
				c.errorf(PosOf(ife.ElseBlock), "expected `%s`, found `%s`", typeStr(thenTy), typeStr(elseTy))
			}
			c.exprTypes[expr] = thenTy
			return thenTy
		}
		return types.Unit
	}
	// Unit enum variant `None` (Ident) infers its payload from the expected
	// Option<T> type.
	if id, ok := expr.(*ast.Ident); ok && id.Name == "None" {
		if app, ok := expected.(*types.Applied); ok {
			if name := appBaseName(app); name == "Option" && len(app.Args) == 1 {
				c.exprTypes[expr] = expected
				return expected
			}
		}
	}
	call, ok := expr.(*ast.CallExpr)
	if ok && len(call.Args) == 0 {
		if fn, ok := call.Func.(*ast.PathExpr); ok && len(fn.Segments) == 2 {
			// Default::default() and Vec::new()/HashMap::new()/Box::new()/String::new()
			// infer their type parameters from the expected type.
			if fn.Segments[0] == "Default" && fn.Segments[1] == "default" {
				c.exprTypes[expr] = expected
				return expected
			}
			if fn.Segments[1] == "new" {
				if app, ok := expected.(*types.Applied); ok {
					name := ""
					switch b := app.Base.(type) {
					case *types.TypeConstructor:
						name = b.Name
					case *types.Named:
						name = b.Name
					}
					if name == fn.Segments[0] {
						c.exprTypes[expr] = expected
						return expected
					}
				}
			}
		}
	}
	// Enum variant construction: Some(x)/None/Ok(x)/Err(x) infer the payload
	// type from the expected Option<T>/Result<T,E>.
	if call != nil {
		if fn, ok := call.Func.(*ast.Ident); ok {
			switch fn.Name {
			case "Some", "None":
				if app, ok := expected.(*types.Applied); ok {
					if name := appBaseName(app); name == "Option" && len(app.Args) == 1 {
						c.exprTypes[expr] = expected
						return expected
					}
				}
			case "Ok", "Err":
				if app, ok := expected.(*types.Applied); ok {
					if name := appBaseName(app); name == "Result" && len(app.Args) == 2 {
						c.exprTypes[expr] = expected
						return expected
					}
				}
			}
		}
		// Generic function call with no arguments: infer type parameters from the
		// expected return type (unification with the declared return type).
		if expected != nil && !isError(expected) && len(call.Args) == 0 {
			if ret, ok := c.inferGenericFromReturn(call, expected); ok {
				c.exprTypes[expr] = ret
				return ret
			}
		}
	}
	return c.checkExpr(expr, env, loans, path)
}

// inferGenericFromReturn tries to infer a generic function's type parameters
// by unifying the expected type with its declared return type. It succeeds
// only when every generic parameter is resolved. Args are left unchecked (the
// caller falls back to checkExpr when inference fails).
func (c *Checker) inferGenericFromReturn(call *ast.CallExpr, expected types.Type) (types.Type, bool) {
	var key string
	switch fn := call.Func.(type) {
	case *ast.Ident:
		key = c.resolveName(fn.Name)
	case *ast.PathExpr:
		key = c.resolvePath(fn.Segments)
	default:
		return nil, false
	}
	if key == "" {
		return nil, false
	}
	info, ok := c.fns[key]
	if !ok || len(info.genParams) == 0 {
		return nil, false
	}
	mapping := make(map[string]types.Type)
	lifetimeMapping := make(map[string]string)
	if !types.Unify(expected, info.ret, mapping, lifetimeMapping) {
		return nil, false
	}
	for _, p := range info.genParams {
		if _, ok := mapping[p]; !ok {
			return nil, false
		}
	}
	return types.Substitute(info.ret, mapping, lifetimeMapping), true
}

func appBaseName(t types.Type) string {
	app, ok := t.(*types.Applied)
	if !ok {
		return ""
	}
	switch b := app.Base.(type) {
	case *types.TypeConstructor:
		return b.Name
	case *types.Named:
		return b.Name
	}
	return ""
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
			fieldMap[name] = types.Substitute(fty, mapping, nil)
		}
	}
	provided := make(map[string]bool)
	for _, init := range e.Fields {
		expected, ok := fieldMap[init.Name]
		if !ok {
			c.errorf(init.Pos, "unknown field `%s` on struct `%s`", init.Name, st.decl.Name)
			continue
		}
		valTy := c.checkExprExpected(init.Value, expected, env, loans, path)
		if !valTy.Equals(expected) && !isError(valTy) {
			c.errorf(PosOf(init.Value), "expected `%s`, found `%s`", typeStr(expected), typeStr(valTy))
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
			c.errorf(PosOf(elem), "expected `%s`, found `%s`", typeStr(elemTy), typeStr(ty))
		}
	}
	return &types.Array{Elem: elemTy, Len: int64(len(e.Elems))}
}

func (c *Checker) checkTupleExpr(e *ast.TupleExpr, env *environment, loans *borrowCtx, path string) types.Type {
	if len(e.Elements) == 0 {
		return types.Unit
	}
	elems := make([]types.Type, len(e.Elements))
	for i, elem := range e.Elements {
		elems[i] = c.checkExpr(elem, env, loans, path)
	}
	return &types.Tuple{Elems: elems}
}

func (c *Checker) checkMacroCall(e *ast.MacroCallExpr, env *environment, loans *borrowCtx, path string) types.Type {
	key := c.resolveName(e.Name)
	m, ok := c.macros[key]
	if !ok {
		c.errorf(e.Pos, "unknown macro `%s`", e.Name)
		return &types.Error{}
	}
	if !c.canAccess(key) {
		c.errorf(e.Pos, "cannot access macro `%s`", e.Name)
		return &types.Error{}
	}
	if m.Body == nil {
		return types.Unit
	}
	return c.checkExpr(m.Body, env, loans, path)
}

func (c *Checker) resolveAlias(key string, info *aliasInfo, args []types.Type) types.Type {
	base := c.resolveAliasBase(key, info)
	if isError(base) {
		return base
	}
	if len(args) != len(info.decl.GenParams) {
		c.errorf(info.decl.Pos, "type alias `%s` expects %d arguments, found %d", info.decl.Name, len(info.decl.GenParams), len(args))
		return &types.Error{}
	}
	return substituteAlias(base, info.decl.GenParams, args)
}

func (c *Checker) resolveAliasBase(key string, info *aliasInfo) types.Type {
	if info.base != nil {
		return info.base
	}
	if info.resolving {
		if !c.aliasCycleReported {
			c.errorf(info.decl.Pos, "cyclic type alias `%s`", key)
			c.aliasCycleReported = true
		}
		return &types.Error{}
	}
	info.resolving = true
	previousIdx := c.currentIdx
	previousPath := c.currentPath
	c.currentIdx = info.fileIdx
	c.currentPath = c.Paths[info.fileIdx]
	info.base = c.resolveType(info.decl.Ty, c.currentPath, info.decl.GenParams)
	c.currentIdx = previousIdx
	c.currentPath = previousPath
	info.resolving = false
	return info.base
}

func substituteAlias(base types.Type, params []string, args []types.Type) types.Type {
	if len(params) == 0 {
		return base
	}
	mapping := make(map[string]types.Type, len(params))
	for i, param := range params {
		if i < len(args) {
			mapping[param] = args[i]
		}
	}
	return types.Substitute(base, mapping, nil)
}

func containsStr(list []string, s string) bool {
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
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
		switch ty.Name {
		case "i32":
			return types.I32
		case "bool":
			return types.Bool
		case "String":
			return types.String
		}
		for _, p := range genParams {
			if p == ty.Name {
				return &types.Generic{Name: ty.Name}
			}
		}
		var args []types.Type
		for _, arg := range ty.Args {
			args = append(args, c.resolveType(arg, path, genParams))
		}
		key := c.resolvePath(strings.Split(ty.Name, "::"))
		if alias, ok := c.aliases[key]; ok {
			return c.resolveAlias(key, alias, args)
		}
		base := types.Type(&types.Named{Name: ty.Name})
		if len(args) > 0 {
			base = &types.Applied{Base: base, Args: args}
		}
		return base
	case *ast.RefType:
		return &types.Ref{Elem: c.resolveType(ty.Elem, path, genParams), IsMut: ty.IsMut, Lifetime: ty.Lifetime}
	case *ast.ArrayType:
		arr := &types.Array{Elem: c.resolveType(ty.Elem, path, genParams), Len: ty.Len}
		if ty.LenName != "" {
			arr.LenName = ty.LenName
			if !containsStr(genParams, ty.LenName) {
				c.errorf(ty.Pos, "const parameter `%s` is not in scope", ty.LenName)
			}
		}
		return arr
	case *ast.ConstIntLitType:
		return &types.ConstInt{Val: ty.Val}
	case *ast.SliceType:
		return &types.Slice{Elem: c.resolveType(ty.Elem, path, genParams)}
	case *ast.TupleType:
		elems := make([]types.Type, len(ty.ElementTypes))
		for i, et := range ty.ElementTypes {
			elems[i] = c.resolveType(et, path, genParams)
		}
		return &types.Tuple{Elems: elems}
	case *ast.ImplTraitType:
		return c.implTraitBase(ty.Trait, path, genParams)
	default:
		return &types.Error{}
	}
}

// implTraitBase resolves an `impl Trait<...>` type to just its base trait
// name, discarding associated type bounds such as `Item = T`. This lets a
// concrete builtin iterator stub (which tracks only the base name) unify with
// a declaration that spells out its Item type.
func (c *Checker) implTraitBase(t ast.Type, path string, genParams []string) types.Type {
	resolved := c.resolveType(t, path, genParams)
	if app, ok := resolved.(*types.Applied); ok {
		return app.Base
	}
	return resolved
}

func (c *Checker) errorf(pos ast.Pos, format string, args ...interface{}) {
	line, col := c.lineCol(c.currentIdx, int(pos))
	c.Reporter.Errorf(c.currentPath, line, col, format, args...)
}

// SetSources provides the raw file contents (aligned with Files/Paths) used
// for translating byte offsets into line:col diagnostics positions.
func (c *Checker) SetSources(sources [][]byte) {
	c.sources = sources
	c.lineCache = make(map[int][]int)
}

// lineCol converts a byte offset (0-based, as produced by the lexer) into a
// 1-based line/column pair. Falls back to 1:1 when no source is available.
func (c *Checker) lineCol(fileIdx int, off int) (int, int) {
	if off < 0 || fileIdx < 0 || fileIdx >= len(c.sources) {
		return 1, 1
	}
	starts, ok := c.lineCache[fileIdx]
	if !ok {
		src := c.sources[fileIdx]
		starts = make([]int, 1, 64)
		for i := 0; i < len(src); i++ {
			if src[i] == '\n' {
				starts = append(starts, i+1)
			}
		}
		if c.lineCache == nil {
			c.lineCache = make(map[int][]int)
		}
		c.lineCache[fileIdx] = starts
	}
	line := sort.Search(len(starts), func(i int) bool { return starts[i] > off }) - 1
	if line < 0 {
		line = 0
	}
	return line + 1, off - starts[line] + 1
}

func PosOf(n ast.Node) ast.Pos {
	if p, ok := n.(interface{ GetPos() ast.Pos }); ok {
		return p.GetPos()
	}
	return 0
}

func isError(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.(*types.Error)
	return ok
}

// typeStr renders t safely even when t is the typed-nil interface value.
func typeStr(t types.Type) string {
	if t == nil {
		return "<unknown>"
	}
	// Avoid calling String() on a typed-nil pointer (e.g. *Builtin(nil) wrapped
	// in a Type interface) since some String() methods panic on nil dereference.
	if v := reflect.ValueOf(t); v.Kind() == reflect.Ptr && v.IsNil() {
		return "<unknown>"
	}
	return t.String()
}

func (c *Checker) checkLifetimes(pos ast.Pos, t types.Type, scope []string) {
	switch ty := t.(type) {
	case *types.Ref:
		if ty.Lifetime != "" && ty.Lifetime != "'static" {
			found := false
			for _, l := range scope {
				if l == ty.Lifetime {
					found = true
					break
				}
			}
			if !found {
				c.errorf(pos, "lifetime `%s` is not in scope", ty.Lifetime)
			}
		} else if ty.Lifetime == "" {
			c.errorf(pos, "cannot return reference with anonymous lifetime")
		}
		c.checkLifetimes(pos, ty.Elem, scope)
	case *types.Applied:
		for _, a := range ty.Args {
			c.checkLifetimes(pos, a, scope)
		}
	}
}

func (c *Checker) checkBlock(block *ast.BlockExpr, env *environment, loans *borrowCtx, ret types.Type, path string) types.Type {
	local := newEnv(env)
	localLoans := newBorrowCtx(loans)
	for i, s := range block.Stmts {
		c.checkStmt(s, local, localLoans, ret, path)
		localLoans.endStatement(block.Stmts[i+1:], block.Result)
	}
	if block.Result != nil {
		if lit, ok := block.Result.(*ast.StructLit); ok {
			if applied, ok := ret.(*types.Applied); ok {
				return c.checkStructLitWithAnnotation(lit, applied, local, localLoans, path)
			}
		}
		// Check the tail expression against the block's expected return type so
		// constructors like Some(x)/Ok(x) infer their payload from the return
		// type (proper Rust semantics).
		if ret != nil && !isError(ret) {
			return c.checkExprExpected(block.Result, ret, local, localLoans, path)
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
		} else if annot != nil {
			valTy = c.checkExprExpected(st.Value, annot, env, loans, path)
			if loans != nil {
				c.move(loans, st.Value, valTy)
				c.registerRefHolder(loans, st.Name, st.Value)
			}
		} else {
			valTy = c.checkExpr(st.Value, env, loans, path)
			if loans != nil {
				c.move(loans, st.Value, valTy)
				c.registerRefHolder(loans, st.Name, st.Value)
			}
		}
		if annot != nil {
			if !annot.Equals(valTy) && !isError(valTy) {
				c.errorf(PosOf(st.Value), "expected `%s`, found `%s`", typeStr(annot), typeStr(valTy))
			}
		}
		ty := valTy
		if annot != nil {
			ty = annot
		}
		if st.Pattern != nil {
			c.checkPattern(st.Pattern, ty, env, st.IsMut, path)
		} else {
			env.set(st.Name, ty, st.IsMut)
		}
	case *ast.AssignStmt:
		if loans != nil {
			// Reassignment ends the old reference held by the target, if any.
			if id, ok := st.Left.(*ast.Ident); ok {
				loans.removeHolder(id.Name)
			}
		}
		c.checkAssign(st, env, loans, path)
		if loans != nil {
			if id, ok := st.Left.(*ast.Ident); ok {
				c.registerRefHolder(loans, id.Name, st.Right)
			}
		}
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
			c.errorf(st.Pos, "expected `%s`, found `%s`", typeStr(ret), typeStr(ty))
		}
	case *ast.WhileStmt:
		cond := c.checkExpr(st.Cond, env, loans, path)
		if !cond.Equals(types.Bool) && !isError(cond) {
			c.errorf(PosOf(st.Cond), "expected `bool`, found `%s`", cond)
		}
		c.checkBlock(st.Body, env, loans, ret, path)
	case *ast.ForStmt:
		iterTy := c.checkExpr(st.Iter, env, loans, path)
		elemTy := c.iteratorElem(iterTy)
		c.checkPattern(st.Pat, elemTy, env, true, path)
		c.checkBlock(st.Body, env, loans, ret, path)
	case *ast.ExprStmt:
		c.checkExpr(st.Expr, env, loans, path)
	}
}

func (c *Checker) checkPattern(pat ast.Pattern, ty types.Type, env *environment, isMut bool, path string) {
	switch p := pat.(type) {
	case *ast.PatIdent:
		if len(p.Name) > 0 && p.Name[0] >= 'A' && p.Name[0] <= 'Z' && c.isVariantPattern(p.Name, ty) {
			// A capitalized name that matches a unit variant (`None`, `Ok`,
			// `Active`) is a path pattern, not a catch-all binding.
			return
		}
		if p.IsRef {
			// `ref x` binds x: &T; `ref mut x` binds x: &mut T.
			env.set(p.Name, &types.Ref{Elem: ty, IsMut: p.IsMut}, false)
			return
		}
		env.set(p.Name, ty, isMut || p.IsMut)
	case *ast.PatWildcard:
		return
	case *ast.PatStruct:
		c.checkStructPattern(p, ty, env, isMut, path)
	case *ast.PatTuple:
		c.checkTuplePattern(p, ty, env, isMut, path)
	case *ast.PatPath:
		c.checkPathPattern(p, ty, env, isMut, path)
	case *ast.PatRange:
		return // integer range literal pattern: no bindings
	case *ast.PatLit:
		c.checkLitPattern(p, ty)
	case *ast.PatSlice:
		c.checkSlicePattern(p, ty, env, isMut, path)
	case *ast.PatOr:
		for _, alt := range p.Alternatives {
			c.checkPattern(alt, ty, env, isMut, path)
		}
	default:
		c.errorf(PosOf(pat), "unsupported pattern")
	}
}

func (c *Checker) checkPathPattern(pat *ast.PatPath, ty types.Type, env *environment, isMut bool, path string) {
	// User enum tuple variants bind their sub-patterns to the declared payload
	// types of the matched variant.
	if len(pat.Path) > 0 {
		if name := c.typeName(ty); name != "" {
			if key := c.resolveName(name); key != "" {
				if ei, ok := c.enums[key]; ok {
					variant := pat.Path[len(pat.Path)-1]
					if fs, known := c.variantFields(key, ei, variant); known {
						if len(fs) == len(pat.Elements) {
							for i, elem := range pat.Elements {
								c.checkPattern(elem, fs[i], env, isMut, path)
							}
							return
						}
						c.errorf(pat.Pos, "pattern has %d subpatterns but variant `%s` has %d field(s)", len(pat.Elements), variant, len(fs))
						return
					}
				}
			}
		}
	}
	if len(pat.Elements) == 0 {
		return
	}
	// Builtin Option/Result variants bind their sub-patterns to payload types.
	if ap, ok := ty.(*types.Applied); ok && len(pat.Path) > 0 {
		base := appBaseName(ap)
		if base == "Option" || base == "Result" {
			variant := pat.Path[len(pat.Path)-1]
			var payload types.Type
			switch {
			case base == "Option" && variant == "Some" && len(ap.Args) == 1:
				payload = ap.Args[0]
			case base == "Result" && variant == "Ok" && len(ap.Args) == 2:
				payload = ap.Args[0]
			case base == "Result" && variant == "Err" && len(ap.Args) == 2:
				payload = ap.Args[1]
			}
			known := payload != nil || (base == "Option" && variant == "None")
			if known {
				want := 0
				if payload != nil {
					want = 1
				}
				if len(pat.Elements) != want {
					c.errorf(pat.Pos, "pattern has %d subpatterns but variant `%s` has %d field(s)", len(pat.Elements), variant, want)
					return
				}
				if want == 1 {
					c.checkPattern(pat.Elements[0], payload, env, isMut, path)
				}
				return
			}
		}
	}
	// HashMap entry variants have concrete field types in the standard library.
	// Keep these types explicit so dereference and method checking remain strict.
	fieldTy := types.Type(&types.Generic{Name: "_"})
	if len(pat.Path) == 2 && pat.Path[0] == "Entry" {
		switch pat.Path[1] {
		case "Occupied":
			fieldTy = &types.Named{Name: "OccupiedEntry"}
		case "Vacant":
			fieldTy = &types.Named{Name: "VacantEntry"}
		}
	}
	for _, elem := range pat.Elements {
		c.checkPattern(elem, fieldTy, env, isMut, path)
	}
}

// checkSlicePattern binds each element of an array/slice pattern to the
// element type of the scrutinee.
func (c *Checker) checkSlicePattern(pat *ast.PatSlice, ty types.Type, env *environment, isMut bool, path string) {
	elemTy := c.sliceElemType(ty)
	for _, elem := range pat.Elements {
		c.checkPattern(elem, elemTy, env, isMut, path)
	}
}

// sliceElemType returns the element type of an array, slice or Vec type.
func (c *Checker) sliceElemType(t types.Type) types.Type {
	switch ty := t.(type) {
	case *types.Array:
		return ty.Elem
	case *types.Slice:
		return ty.Elem
	case *types.Applied:
		if len(ty.Args) == 1 {
			return ty.Args[0]
		}
	case *types.Ref:
		return c.sliceElemType(ty.Elem)
	}
	return &types.Generic{Name: "_"}
}

func (c *Checker) checkTuplePattern(pat *ast.PatTuple, ty types.Type, env *environment, isMut bool, path string) {
	// When the enclosing element type is an unknown generic placeholder (e.g. a
	// builtin iterator whose Item type is not tracked), bind each element to the
	// same placeholder so tuple destructuring in for-loops keeps variables in
	// scope without a spurious type error.
	if g, ok := ty.(*types.Generic); ok && len(pat.Elements) > 0 {
		for _, elem := range pat.Elements {
			c.checkPattern(elem, g, env, isMut, path)
		}
		return
	}
	t, ok := ty.(*types.Tuple)
	if !ok {
		if !isError(ty) {
			c.errorf(pat.Pos, "expected tuple, found `%s`", ty)
		}
		return
	}
	if len(pat.Elements) != len(t.Elems) {
		c.errorf(pat.Pos, "expected tuple with %d elements, found %d", len(t.Elems), len(pat.Elements))
		return
	}
	for i, elem := range pat.Elements {
		c.checkPattern(elem, t.Elems[i], env, isMut, path)
	}
}

func (c *Checker) checkStructPattern(pat *ast.PatStruct, ty types.Type, env *environment, isMut bool, path string) {
	key := c.resolveName(pat.Name)
	st, ok := c.structs[key]
	if !ok {
		if isBuiltinTypeName(pat.Name) {
			// Bind pattern fields loosely for unparsed standard-lib structs.
			for _, b := range pat.Fields {
				v := b.BindName
				if v == "" {
					v = b.Field
				}
				c.checkPattern(&ast.PatIdent{Pos: b.Pos, Name: v}, &types.Generic{Name: "_"}, env, isMut, path)
			}
			return
		}
		c.errorf(pat.Pos, "unknown struct `%s`", pat.Name)
		return
	}
	if !c.canAccess(key) {
		c.errorf(pat.Pos, "cannot access struct `%s`", pat.Name)
		return
	}
	want := &types.Named{Name: st.decl.Name}
	if len(st.genParams) > 0 {
		c.errorf(pat.Pos, "generic struct patterns require explicit type annotation")
		return
	}
	matchTy := ty
	if ref, ok := ty.(*types.Ref); ok {
		// Rust match ergonomics permits a struct pattern to match through
		// a shared reference and binds its fields by reference.
		matchTy = ref.Elem
	}
	if !want.Equals(matchTy) && !isError(matchTy) {
		c.errorf(pat.Pos, "expected `%s`, found `%s`", typeStr(want), typeStr(ty))
		return
	}
	provided := make(map[string]bool)
	for _, f := range pat.Fields {
		fty, ok := st.fields[f.Field]
		if !ok {
			c.errorf(f.Pos, "no field `%s` on struct `%s`", f.Field, st.decl.Name)
			continue
		}
		name := f.Field
		if f.BindName != "" {
			name = f.BindName
		}
		env.set(name, fty, isMut)
		provided[f.Field] = true
	}
	for name := range st.fields {
		if !provided[name] {
			c.errorf(pat.Pos, "missing field `%s` in pattern for struct `%s`", name, st.decl.Name)
		}
	}
}

func (c *Checker) checkAssign(st *ast.AssignStmt, env *environment, loans *borrowCtx, path string) {
	rightTy := c.checkExpr(st.Right, env, loans, path)
	leftTy := c.checkExprNoBorrow(st.Left, env, path)
	if !leftTy.Equals(rightTy) && !isError(leftTy) && !isError(rightTy) {
		c.errorf(PosOf(st.Right), "expected `%s`, found `%s`", typeStr(leftTy), typeStr(rightTy))
	}
	c.useWrite(loans, st.Left, leftTy)
	c.move(loans, st.Right, rightTy)
}

func (c *Checker) checkExprNoBorrow(expr ast.Expr, env *environment, path string) types.Type {
	return c.checkExpr(expr, env, nil, path)
}

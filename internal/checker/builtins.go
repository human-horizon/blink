package checker

import "github.com/humanhorizon/blink/internal/types"

// builtinTypes is the set of type names that have synthetic method/function
// stubs available. Adding entries here suppresses "no method/function" errors
// for the corresponding Bevy/std-lib-style APIs without requiring real
// declarations.
var builtinTypes = map[string]struct{}{
	"Vec":                {},
	"Box":                {},
	"Option":             {},
	"Result":             {},
	"String":             {},
	"HashMap":            {},
	"BTreeMap":           {},
	"HashSet":            {},
	"BTreeSet":           {},
	"VecDeque":           {},
	"Iterator":           {},
	"ExactSizeIterator":  {},
	"Send":               {},
	"Sync":               {},
	"Unpin":              {},
	"RangeFrom":          {},
	"RangeTo":            {},
	"Range":              {},
	"RangeFull":          {},
	"RangeInclusive":     {},
	"slice":              {},
	"tuple":              {},
	"array":              {},
	"ComponentId":        {},
	"BundleId":           {},
	"Entity":             {},
	"EntityLocation":     {},
	"ArchetypeId":        {},
	"TableId":            {},
	"TableRow":           {},
	"ArchetypeRow":       {},
	"ArchetypeFlags":     {},
	"NonMaxU32":          {},
	"NonMaxU64":          {},
	"NonMax":             {},
	"SparseSet":          {},
	"ImmutableSparseSet": {},
	"Archetype":          {},
	"Archetypes":         {},
	"Bundle":             {},
	"Component":          {},
	"Components":         {},
	"Observer":           {},
	"Observers":          {},
	"SparseArray":        {},
	"Event":              {},
	"EventKey":           {},
	"StorageType":        {},
	"World":              {},
	"Query":              {},
	"QueryState":         {},
}

type builtinKey struct {
	typeName   string
	methodName string
}

type builtinSig struct {
	paramTypes []types.Type
	ret        types.Type
	selfType   types.Type
}

var builtinMethods = map[builtinKey]builtinSig{}

func builtinMethod(typeName, methodName string) *fnInfo {
	if _, ok := builtinTypes[typeName]; !ok {
		return nil
	}
	sig, ok := builtinMethods[builtinKey{typeName, methodName}]
	if !ok {
		return nil
	}
	return &fnInfo{
		paramTypes: sig.paramTypes,
		ret:        sig.ret,
		selfType:   sig.selfType,
	}
}

func init() {
	i32 := types.I32
	boolT := types.Bool
	unitT := &types.Tuple{}
	strT := &types.Named{Name: "String"}
	ref := func(t types.Type) types.Type { return &types.Ref{Elem: t} }
	refMut := func(t types.Type) types.Type { return &types.Ref{Elem: t, IsMut: true} }
	register := func(typeName, method string, self types.Type, params []types.Type, ret types.Type) {
		builtinMethods[builtinKey{typeName, method}] = builtinSig{
			selfType:   self,
			paramTypes: params,
			ret:        ret,
		}
	}
	opt := &types.Named{Name: "Option"}
	res := &types.Named{Name: "Result"}
	vec := &types.Named{Name: "Vec"}
	iter := &types.Named{Name: "Iterator"}
	hmap := &types.Named{Name: "HashMap"}

	register("Vec", "new", nil, nil, vec)
	register("Vec", "with_capacity", nil, []types.Type{i32}, vec)
	register("Vec", "len", ref(vec), nil, i32)
	register("Vec", "is_empty", ref(vec), nil, boolT)
	register("Vec", "push", refMut(vec), []types.Type{nil}, unitT)
	register("Vec", "pop", refMut(vec), nil, opt)
	register("Vec", "iter", ref(vec), nil, iter)
	register("Vec", "into_iter", ref(vec), nil, iter)
	register("Vec", "reserve", refMut(vec), []types.Type{i32}, unitT)
	register("Vec", "reserve_exact", refMut(vec), []types.Type{i32}, unitT)
	register("Vec", "clear", refMut(vec), nil, unitT)
	register("Vec", "extend", refMut(vec), []types.Type{iter}, unitT)
	register("Vec", "into_boxed_slice", ref(vec), nil, &types.Named{Name: "Box"})

	register("Box", "new", nil, []types.Type{nil}, &types.Named{Name: "Box"})
	register("Box", "leak", nil, nil, nil)

	register("Option", "is_some", ref(opt), nil, boolT)
	register("Option", "is_none", ref(opt), nil, boolT)
	register("Option", "unwrap", ref(opt), nil, nil)
	register("Option", "unwrap_or", ref(opt), []types.Type{nil}, nil)
	register("Option", "unwrap_or_else", ref(opt), []types.Type{nil}, nil)
	register("Option", "map", ref(opt), []types.Type{nil}, opt)
	register("Option", "and_then", ref(opt), []types.Type{nil}, opt)
	register("Option", "or_else", ref(opt), []types.Type{nil}, opt)
	register("Option", "ok_or", ref(opt), []types.Type{nil}, res)
	register("Option", "take", refMut(opt), nil, opt)
	register("Option", "as_ref", ref(opt), nil, opt)
	register("Option", "expect", ref(opt), []types.Type{strT}, nil)

	register("Result", "is_ok", ref(res), nil, boolT)
	register("Result", "is_err", ref(res), nil, boolT)
	register("Result", "unwrap", ref(res), nil, nil)
	register("Result", "expect", ref(res), []types.Type{strT}, nil)
	register("Result", "ok", ref(res), nil, opt)
	register("Result", "err", ref(res), nil, opt)

	register("Iterator", "next", refMut(iter), nil, opt)
	register("Iterator", "map", ref(iter), []types.Type{nil}, iter)
	register("Iterator", "filter", ref(iter), []types.Type{nil}, iter)
	register("Iterator", "collect", ref(iter), nil, nil)
	register("Iterator", "count", ref(iter), nil, i32)
	register("Iterator", "enumerate", ref(iter), nil, iter)
	register("Iterator", "size_hint", ref(iter), nil, &types.Tuple{Elems: []types.Type{i32, i32}})
	register("Iterator", "sum", ref(iter), nil, nil)
	register("Iterator", "cloned", ref(iter), nil, iter)
	register("Iterator", "copied", ref(iter), nil, iter)
	register("Iterator", "fold", ref(iter), []types.Type{nil, nil}, nil)
	register("Iterator", "for_each", refMut(iter), []types.Type{nil}, unitT)
	register("Iterator", "any", ref(iter), []types.Type{nil}, boolT)
	register("Iterator", "all", ref(iter), []types.Type{nil}, boolT)

	register("String", "new", nil, nil, strT)
	register("String", "with_capacity", nil, []types.Type{i32}, strT)
	register("String", "len", ref(strT), nil, i32)
	register("String", "is_empty", ref(strT), nil, boolT)
	register("String", "push_str", refMut(strT), []types.Type{strT}, unitT)
	register("String", "as_str", ref(strT), nil, strT)
	register("String", "into_bytes", ref(strT), nil, &types.Slice{Elem: types.Bool})
	register("String", "from", nil, []types.Type{strT}, strT)

	register("HashMap", "new", nil, nil, hmap)
	register("HashMap", "with_capacity", nil, []types.Type{i32}, hmap)
	register("HashMap", "len", ref(hmap), nil, i32)
	register("HashMap", "is_empty", ref(hmap), nil, boolT)
	register("HashMap", "get", ref(hmap), []types.Type{nil}, opt)
	register("HashMap", "insert", refMut(hmap), []types.Type{nil, nil}, opt)
	register("HashMap", "remove", refMut(hmap), []types.Type{nil}, opt)
	register("HashMap", "contains_key", ref(hmap), []types.Type{nil}, boolT)
	register("HashMap", "entry", refMut(hmap), []types.Type{nil}, &types.Named{Name: "Entry"})
	register("HashMap", "iter", ref(hmap), nil, iter)

	register("RangeFrom", "start", ref(&types.Named{Name: "RangeFrom"}), nil, nil)

	register("NonMaxU32", "new", nil, []types.Type{i32}, opt)
	register("NonMaxU32", "get", ref(&types.Named{Name: "NonMaxU32"}), nil, i32)
	register("NonMaxU32", "new_unchecked", nil, []types.Type{i32}, &types.Named{Name: "NonMaxU32"})

	register("ArchetypeFlags", "empty", nil, nil, &types.Named{Name: "ArchetypeFlags"})
	register("ArchetypeFlags", "all", nil, nil, &types.Named{Name: "ArchetypeFlags"})
	register("ArchetypeFlags", "contains", ref(&types.Named{Name: "ArchetypeFlags"}), []types.Type{&types.Named{Name: "ArchetypeFlags"}}, boolT)
	register("ArchetypeFlags", "set", refMut(&types.Named{Name: "ArchetypeFlags"}), []types.Type{&types.Named{Name: "ArchetypeFlags"}, boolT}, unitT)

	for _, tn := range []string{"ComponentId", "BundleId", "Entity", "EntityLocation", "ArchetypeId", "TableId", "TableRow", "ArchetypeRow", "StorageType", "ComponentStatus"} {
		register(tn, "new", nil, []types.Type{i32}, &types.Named{Name: tn})
		register(tn, "empty", nil, nil, &types.Named{Name: tn})
		register(tn, "index", ref(&types.Named{Name: tn}), nil, i32)
	}
	register("Entity", "new", nil, []types.Type{i32, i32, i32}, &types.Named{Name: "Entity"})
	register("TableId", "empty", nil, nil, &types.Named{Name: "TableId"})
	register("ArchetypeId", "EMPTY", nil, nil, &types.Named{Name: "ArchetypeId"})

	for _, tn := range []string{"SparseSet", "ImmutableSparseSet", "SparseArray", "Components", "Observers"} {
		register(tn, "new", nil, nil, &types.Named{Name: tn})
		register(tn, "default", nil, nil, &types.Named{Name: tn})
		register(tn, "with_capacity", nil, []types.Type{i32}, &types.Named{Name: tn})
		register(tn, "len", ref(&types.Named{Name: tn}), nil, i32)
		register(tn, "is_empty", ref(&types.Named{Name: tn}), nil, boolT)
		register(tn, "get", ref(&types.Named{Name: tn}), []types.Type{nil}, opt)
		register(tn, "insert", refMut(&types.Named{Name: tn}), []types.Type{nil, nil}, unitT)
		register(tn, "contains", ref(&types.Named{Name: tn}), []types.Type{nil}, boolT)
		register(tn, "indices", ref(&types.Named{Name: tn}), nil, &types.Slice{Elem: nil})
		register(tn, "iter", ref(&types.Named{Name: tn}), nil, iter)
	}

	register("Index", "index", ref(nil), []types.Type{nil}, nil)

	// Slice, tuple, array synthetic names produced by typeName().
	sliceT := &types.Named{Name: "slice"}
	tupleT := &types.Named{Name: "tuple"}
	arrayT := &types.Named{Name: "array"}
	register("slice", "len", ref(sliceT), nil, i32)
	register("slice", "is_empty", ref(sliceT), nil, boolT)
	register("slice", "get", ref(sliceT), []types.Type{i32}, opt)
	register("slice", "iter", ref(sliceT), nil, iter)
	register("slice", "first", ref(sliceT), nil, opt)
	register("slice", "last", ref(sliceT), nil, opt)
	register("slice", "contains", ref(sliceT), []types.Type{nil}, boolT)
	register("slice", "as_ptr", ref(sliceT), nil, nil)
	register("slice", "len_usize", ref(sliceT), nil, i32)
	register("tuple", "0", ref(tupleT), nil, nil)
	register("tuple", "1", ref(tupleT), nil, nil)
	register("array", "len", ref(arrayT), nil, i32)
	register("array", "is_empty", ref(arrayT), nil, boolT)
}

// builtinPath returns true when a fully-qualified path like Vec::new, Some,
// None, Default::default should be treated as resolved without reporting
// "cannot find function/value".
func builtinPath(p []string) bool {
	if len(p) == 0 {
		return false
	}
	last := p[len(p)-1]
	switch last {
	case "Some", "None", "Ok", "Err":
		return true
	case "default", "new", "empty", "with_capacity", "from", "into", "EMPTY":
		return true
	}
	if _, ok := builtinTypes[p[0]]; ok {
		return true
	}
	return false
}

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
	"i32":                {},
	"i64":                {},
	"i8":                 {},
	"u32":                {},
	"u64":                {},
	"usize":              {},
	"f32":                {},
	"f64":                {},
	"bool":               {},
	"Into":               {},
	"Index":              {},
	"IndexMut":           {},
	"ComponentId":        {},
	"BundleId":           {},
	"Entity":             {},
	"EntityLocation":     {},
	"ArchetypeId":        {},
	"TableId":            {},
	"TableRow":           {},
	"ArchetypeRow":       {},
	"ArchetypeFlags":     {},
	"ComponentInfo":      {},
	"NonMaxU32":          {},
	"NonMaxU64":          {},
	"NonMax":             {},
	"SparseSet":          {},
	"ImmutableSparseSet": {},
	"ComponentIndex":     {},
	"Edges":              {},
	"Entry":              {},
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
	"ComponentStatus":    {},
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

func isBuiltinTrait(name string) bool {
	switch name {
	case "Index", "IndexMut", "Into", "From":
		return true
	default:
		return false
	}
}

// isBuiltinTypeName reports whether name is a known std-lib/Bevy type stub.
func isBuiltinTypeName(name string) bool {
	_, ok := builtinTypes[name]
	return ok
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
	register("Vec", "into", ref(vec), nil, &types.Error{})
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
	for _, cn := range []string{"ON_ADD_HOOK", "ON_INSERT_HOOK", "ON_DISCARD_HOOK", "ON_REMOVE_HOOK", "ON_DESPAWN_HOOK", "ON_ADD_OBSERVER", "ON_INSERT_OBSERVER", "ON_DISCARD_OBSERVER", "ON_REMOVE_OBSERVER", "ON_DESPAWN_OBSERVER"} {
		register("ArchetypeFlags", cn, nil, nil, &types.Named{Name: "ArchetypeFlags"})
	}

	for _, tn := range []string{"ComponentId", "BundleId", "Entity", "EntityLocation", "ArchetypeId", "TableId", "TableRow", "ArchetypeRow", "StorageType", "ComponentStatus"} {
		register(tn, "new", nil, []types.Type{i32}, &types.Named{Name: tn})
		register(tn, "empty", nil, nil, &types.Named{Name: tn})
		register(tn, "index", ref(&types.Named{Name: tn}), nil, i32)
	}
	register("Entity", "new", nil, []types.Type{i32, i32, i32}, &types.Named{Name: "Entity"})
	register("TableId", "empty", nil, nil, &types.Named{Name: "TableId"})
	register("ArchetypeId", "EMPTY", nil, nil, &types.Named{Name: "ArchetypeId"})

	// Bits-style accessors on opaque integer-like types.
	register("StorageType", "bits", ref(&types.Named{Name: "StorageType"}), nil, i32)
	register("StorageType", "EMPTY", nil, nil, &types.Named{Name: "StorageType"})
	register("ComponentStatus", "EMPTY", nil, nil, &types.Named{Name: "ComponentStatus"})

	for _, tn := range []string{"SparseSet", "ImmutableSparseSet", "SparseArray", "Components", "Observers", "ComponentIndex", "Edges"} {
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
	// Bevy-specific methods.
	components := &types.Named{Name: "Components"}
	register("Components", "get_info_unchecked", ref(components), []types.Type{nil}, &types.Named{Name: "ComponentInfo"})
	register("Components", "get", ref(components), []types.Type{nil}, opt)
	register("ComponentInfo", "update_archetype_flags", ref(&types.Named{Name: "ComponentInfo"}), []types.Type{nil}, unitT)
	observers := &types.Named{Name: "Observers"}
	register("Observers", "update_archetype_flags", refMut(observers), []types.Type{nil, nil}, unitT)
	register("Observers", "default", nil, nil, observers)
	ci := &types.Named{Name: "ComponentIndex"}
	register("ComponentIndex", "entry", refMut(ci), []types.Type{nil}, &types.Named{Name: "Entry"})
	register("ComponentIndex", "or_default", refMut(ci), []types.Type{nil}, nil)
	register("ComponentIndex", "get", ref(ci), []types.Type{nil}, opt)
	register("ComponentIndex", "get_mut", refMut(ci), []types.Type{nil}, opt)
	register("ComponentIndex", "contains_key", ref(ci), []types.Type{nil}, boolT)
	register("ComponentIndex", "insert", refMut(ci), []types.Type{nil, nil}, opt)
	// HashMap entry API.
	entryT := &types.Named{Name: "Entry"}
	register("Entry", "or_default", refMut(entryT), nil, nil)
	register("Entry", "or_insert", refMut(entryT), []types.Type{nil}, nil)
	register("Entry", "insert", refMut(entryT), []types.Type{nil, nil}, nil)
	register("ArchetypeRow", "index", ref(&types.Named{Name: "ArchetypeRow"}), nil, i32)
	register("ArchetypeId", "index", ref(&types.Named{Name: "ArchetypeId"}), nil, i32)
	vec2 := &types.Named{Name: "Vec"}
	register("Vec", "get", ref(vec2), []types.Type{i32}, opt)
	register("Vec", "get_mut", refMut(vec2), []types.Type{i32}, opt)
	register("Vec", "get_unchecked_mut", refMut(vec2), []types.Type{i32}, nil)
	register("Vec", "get_unchecked", ref(vec2), []types.Type{i32}, nil)
	register("Vec", "get_disjoint_unchecked_mut", refMut(vec2), []types.Type{nil}, nil)
	register("Vec", "swap_remove", refMut(vec2), []types.Type{i32}, nil)
	register("Vec", "first", ref(vec2), nil, opt)
	register("Vec", "last", ref(vec2), nil, opt)
	boxT := &types.Named{Name: "Box"}
	register("Box", "get", ref(boxT), []types.Type{nil}, nil)
	register("Box", "get_unchecked", ref(boxT), []types.Type{nil}, nil)
	register("Box", "downcast", refMut(boxT), []types.Type{nil}, nil)
	optT := &types.Named{Name: "Option"}
	register("Option", "cloned", ref(optT), nil, optT)
	register("Option", "copied", ref(optT), nil, optT)
	register("Option", "transpose", ref(optT), nil, &types.Named{Name: "Result"})

	// Standard conversion trait stubs for source trees without libstd.
	intoT := &types.Named{Name: "Into"}
	register("Into", "into", ref(intoT), nil, &types.Error{})
	register("From", "from", nil, []types.Type{nil}, &types.Error{})

	// ArchetypeGeneration static method.
	archGen := &types.Named{Name: "ArchetypeGeneration"}
	register("ArchetypeGeneration", "initial", nil, nil, archGen)

	// IntoIterator trait stub disabled.
	// register("IntoIterator", "into_iter", ref(nil), nil, iter)

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
	register("array", "iter", ref(arrayT), nil, iter)
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

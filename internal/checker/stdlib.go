package checker

import "github.com/humanhorizon/blink/internal/types"

// stdMethod is a method signature on a std type. Generic parameters of the
// enclosing type (e.g. T in Vec<T>) are referenced as types.Generic{Name:"T"}
// and substituted from the receiver's type arguments at resolution time.
type stdMethod struct {
	name       string
	selfType   types.Type // nil for static methods
	paramTypes []types.Type
	ret        types.Type
}

// stdTrait is a trait with a set of methods.
type stdTrait struct {
	name    string
	methods map[string]*stdMethod
}

// stdType is a generic type constructor with its parameters and the traits it
// implements. Inherent methods are stored per-type in the `inherent` map.
type stdType struct {
	name     string
	params   []string
	traits   []string
	inherent map[string]*stdMethod
}

var stdTypes = map[string]*stdType{}
var stdTraits = map[string]*stdTrait{}

func registerStdType(name string, params []string, traits ...string) *stdType {
	st := &stdType{name: name, params: params, traits: traits, inherent: map[string]*stdMethod{}}
	stdTypes[name] = st
	return st
}

func (st *stdType) method(name string, self types.Type, params []types.Type, ret types.Type) {
	st.inherent[name] = &stdMethod{name: name, selfType: self, paramTypes: params, ret: ret}
}

func registerStdTrait(name string) *stdTrait {
	tr := &stdTrait{name: name, methods: map[string]*stdMethod{}}
	stdTraits[name] = tr
	return tr
}

func (tr *stdTrait) method(name string, self types.Type, params []types.Type, ret types.Type) {
	tr.methods[name] = &stdMethod{name: name, selfType: self, paramTypes: params, ret: ret}
}

// stdMethodInfo resolves a method on a concrete receiver type, substituting the
// type's generic parameters from the receiver's arguments. Returns nil if the
// method is not found.
func (c *Checker) stdMethodInfo(receiver types.Type, method string) *fnInfo {
	baseName := c.typeName(receiver)
	st, ok := stdTypes[baseName]
	if !ok {
		return nil
	}
	mapping := make(map[string]types.Type)
	if app, ok := receiver.(*types.Applied); ok {
		for i, p := range st.params {
			if i < len(app.Args) {
				mapping[p] = app.Args[i]
			}
		}
	}
	// Inherent methods first, then implemented traits.
	if m, ok := st.inherent[method]; ok {
		return c.substituteStdMethod(m, mapping)
	}
	for _, traitName := range st.traits {
		tr, ok := stdTraits[traitName]
		if !ok {
			continue
		}
		m, ok := tr.methods[method]
		if !ok {
			continue
		}
		return c.substituteStdMethod(m, mapping)
	}
	return nil
}

func (c *Checker) substituteStdMethod(m *stdMethod, mapping map[string]types.Type) *fnInfo {
	info := &fnInfo{
		paramTypes: make([]types.Type, len(m.paramTypes)),
		ret:        m.ret,
		selfType:   m.selfType,
		stdlib:     true,
	}
	for i, p := range m.paramTypes {
		info.paramTypes[i] = types.Substitute(p, mapping, nil)
	}
	if m.ret != nil {
		info.ret = types.Substitute(m.ret, mapping, nil)
	}
	if m.selfType != nil {
		info.selfType = types.Substitute(m.selfType, mapping, nil)
	}
	return info
}

func init() {
	// Type constructors.
	vec := registerStdType("Vec", []string{"T"}, "Deref", "IntoIterator")
	opt := registerStdType("Option", []string{"T"})
	res := registerStdType("Result", []string{"T", "E"})
	hm := registerStdType("HashMap", []string{"K", "V"}, "IntoIterator")
	box := registerStdType("Box", []string{"T"}, "Deref")
	str := registerStdType("String", nil)
	registerStdType("Iterator", []string{"Item"})

	// Traits.
	deref := registerStdTrait("Deref")
	intoIter := registerStdTrait("IntoIterator")
	iter := registerStdTrait("Iterator")

	// Deref<Target=[T]> for Vec<T> and Box<T>.
	_ = deref

	// IntoIterator<Item=T> for Vec<T>.
	_ = intoIter

	// Iterator<Item=T> methods.
	iter.method("next", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}, IsMut: true}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "Item"}}})
	iter.method("count", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, nil, types.I32)
	iter.method("size_hint", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, nil,
		&types.Tuple{Elems: []types.Type{types.I32, &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{types.I32}}}})
	iter.method("any", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, []types.Type{nil}, types.Bool)
	iter.method("all", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, []types.Type{nil}, types.Bool)
	iter.method("for_each", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}, IsMut: true}, []types.Type{nil}, &types.Tuple{})
	iter.method("map", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, []types.Type{nil},
		&types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "_"}}})
	iter.method("filter", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, []types.Type{nil},
		&types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}})
	iter.method("enumerate", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "_"}}})
	iter.method("collect", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, nil, nil)
	iter.method("sum", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, nil, nil)
	iter.method("cloned", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "_"}}})
	iter.method("copied", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "_"}}})
	iter.method("fold", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "Item"}}}}, []types.Type{nil, nil}, nil)

	// Vec<T> inherent methods.
	vec.method("new", nil, nil, &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}})
	vec.method("with_capacity", nil, []types.Type{types.I32}, &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}})
	vec.method("len", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil, types.I32)
	vec.method("is_empty", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil, types.Bool)
	vec.method("push", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}, IsMut: true}, []types.Type{&types.Generic{Name: "T"}}, &types.Tuple{})
	vec.method("pop", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}, IsMut: true}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}})
	vec.method("iter", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "T"}}})
	vec.method("into_iter", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "T"}}})
	vec.method("get", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, []types.Type{types.I32},
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Ref{Elem: &types.Generic{Name: "T"}}}})
	vec.method("get_mut", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}, IsMut: true}, []types.Type{types.I32},
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Ref{Elem: &types.Generic{Name: "T"}, IsMut: true}}})
	vec.method("get_unchecked", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, []types.Type{types.I32},
		&types.Ref{Elem: &types.Generic{Name: "T"}})
	vec.method("first", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Ref{Elem: &types.Generic{Name: "T"}}}})
	vec.method("last", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Ref{Elem: &types.Generic{Name: "T"}}}})
	vec.method("clear", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}, IsMut: true}, nil, &types.Tuple{})
	vec.method("reserve", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}, IsMut: true}, []types.Type{types.I32}, &types.Tuple{})
	vec.method("into_boxed_slice", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Vec"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Box"}, Args: []types.Type{&types.Slice{Elem: &types.Generic{Name: "T"}}}})

	// Option<T> inherent methods.
	opt.method("is_some", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil, types.Bool)
	opt.method("is_none", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil, types.Bool)
	opt.method("unwrap", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil, &types.Generic{Name: "T"})
	opt.method("unwrap_or", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, []types.Type{&types.Generic{Name: "T"}}, &types.Generic{Name: "T"})
	opt.method("unwrap_or_else", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, []types.Type{nil}, &types.Generic{Name: "T"})
	opt.method("map", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, []types.Type{nil},
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "_"}}})
	opt.method("and_then", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, []types.Type{nil},
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "_"}}})
	opt.method("or_else", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, []types.Type{nil},
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}})
	opt.method("ok_or", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, []types.Type{nil},
		&types.Applied{Base: &types.TypeConstructor{Name: "Result"}, Args: []types.Type{&types.Generic{Name: "T"}, &types.Generic{Name: "_"}}})
	opt.method("take", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}, IsMut: true}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}})
	opt.method("as_ref", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Ref{Elem: &types.Generic{Name: "T"}}}})
	opt.method("expect", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, []types.Type{types.String}, &types.Generic{Name: "T"})

	// Result<T,E> inherent methods.
	res.method("is_ok", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Result"}, Args: []types.Type{&types.Generic{Name: "T"}, &types.Generic{Name: "E"}}}}, nil, types.Bool)
	res.method("is_err", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Result"}, Args: []types.Type{&types.Generic{Name: "T"}, &types.Generic{Name: "E"}}}}, nil, types.Bool)
	res.method("unwrap", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Result"}, Args: []types.Type{&types.Generic{Name: "T"}, &types.Generic{Name: "E"}}}}, nil, &types.Generic{Name: "T"})
	res.method("expect", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Result"}, Args: []types.Type{&types.Generic{Name: "T"}, &types.Generic{Name: "E"}}}}, []types.Type{types.String}, &types.Generic{Name: "T"})
	res.method("ok", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Result"}, Args: []types.Type{&types.Generic{Name: "T"}, &types.Generic{Name: "E"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "T"}}})
	res.method("err", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Result"}, Args: []types.Type{&types.Generic{Name: "T"}, &types.Generic{Name: "E"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "E"}}})

	// HashMap<K,V> inherent methods.
	hm.method("new", nil, nil, &types.Applied{Base: &types.TypeConstructor{Name: "HashMap"}, Args: []types.Type{&types.Generic{Name: "K"}, &types.Generic{Name: "V"}}})
	hm.method("with_capacity", nil, []types.Type{types.I32}, &types.Applied{Base: &types.TypeConstructor{Name: "HashMap"}, Args: []types.Type{&types.Generic{Name: "K"}, &types.Generic{Name: "V"}}})
	hm.method("len", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "HashMap"}, Args: []types.Type{&types.Generic{Name: "K"}, &types.Generic{Name: "V"}}}}, nil, types.I32)
	hm.method("is_empty", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "HashMap"}, Args: []types.Type{&types.Generic{Name: "K"}, &types.Generic{Name: "V"}}}}, nil, types.Bool)
	hm.method("get", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "HashMap"}, Args: []types.Type{&types.Generic{Name: "K"}, &types.Generic{Name: "V"}}}}, []types.Type{&types.Ref{Elem: &types.Generic{Name: "K"}}},
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Ref{Elem: &types.Generic{Name: "V"}}}})
	hm.method("insert", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "HashMap"}, Args: []types.Type{&types.Generic{Name: "K"}, &types.Generic{Name: "V"}}}, IsMut: true}, []types.Type{&types.Generic{Name: "K"}, &types.Generic{Name: "V"}},
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "V"}}})
	hm.method("remove", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "HashMap"}, Args: []types.Type{&types.Generic{Name: "K"}, &types.Generic{Name: "V"}}}, IsMut: true}, []types.Type{&types.Ref{Elem: &types.Generic{Name: "K"}}},
		&types.Applied{Base: &types.TypeConstructor{Name: "Option"}, Args: []types.Type{&types.Generic{Name: "V"}}})
	hm.method("contains_key", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "HashMap"}, Args: []types.Type{&types.Generic{Name: "K"}, &types.Generic{Name: "V"}}}}, []types.Type{&types.Ref{Elem: &types.Generic{Name: "K"}}}, types.Bool)
	hm.method("iter", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "HashMap"}, Args: []types.Type{&types.Generic{Name: "K"}, &types.Generic{Name: "V"}}}}, nil,
		&types.Applied{Base: &types.TypeConstructor{Name: "Iterator"}, Args: []types.Type{&types.Generic{Name: "_"}}})

	// Box<T> inherent methods.
	box.method("new", nil, []types.Type{&types.Generic{Name: "T"}}, &types.Applied{Base: &types.TypeConstructor{Name: "Box"}, Args: []types.Type{&types.Generic{Name: "T"}}})
	box.method("leak", &types.Ref{Elem: &types.Applied{Base: &types.TypeConstructor{Name: "Box"}, Args: []types.Type{&types.Generic{Name: "T"}}}}, nil, &types.Ref{Elem: &types.Generic{Name: "T"}, IsMut: true})

	// String inherent methods.
	str.method("new", nil, nil, types.String)
	str.method("with_capacity", nil, []types.Type{types.I32}, types.String)
	str.method("len", &types.Ref{Elem: types.String}, nil, types.I32)
	str.method("is_empty", &types.Ref{Elem: types.String}, nil, types.Bool)
	str.method("push_str", &types.Ref{Elem: types.String, IsMut: true}, []types.Type{types.String}, &types.Tuple{})
	str.method("as_str", &types.Ref{Elem: types.String}, nil, types.String)
	str.method("into_bytes", &types.Ref{Elem: types.String}, nil, &types.Slice{Elem: types.Bool})
}

package checker

import "github.com/humanhorizon/blink/internal/types"

// environment is a simple chain of scopes.
type environment struct {
	parent *environment
	vars   map[string]types.Type
	mut    map[string]bool
}

func newEnv(parent *environment) *environment {
	return &environment{
		parent: parent,
		vars:   make(map[string]types.Type),
		mut:    make(map[string]bool),
	}
}

func (e *environment) get(name string) (types.Type, bool) {
	if ty, ok := e.vars[name]; ok {
		return ty, true
	}
	if e.parent != nil {
		return e.parent.get(name)
	}
	return nil, false
}

func (e *environment) set(name string, ty types.Type, mutable bool) {
	e.vars[name] = ty
	e.mut[name] = mutable
}

func (e *environment) isMut(name string) bool {
	if m, ok := e.mut[name]; ok {
		return m
	}
	if e.parent != nil {
		return e.parent.isMut(name)
	}
	return false
}

package dissect

import (
	"fmt"
)

type Environment struct {
	parent *Environment

	block  string
	lookup map[string]int
	values []Value
}

func NewEnvironment(str string) *Environment {
	return NewEnclosedEnvironment(str, nil)
}

func NewEnclosedEnvironment(str string, parent *Environment) *Environment {
	e := Environment{
		block:  str,
		parent: parent,
		lookup: make(map[string]int),
		values: make([]Value, 0, 64),
	}
	return &e
}

func (e *Environment) List() []Value {
	return e.values
}

func (e *Environment) Len() int {
	var i int
	if e.parent != nil {
		i = e.parent.Len()
	}
	return i + len(e.values)
}

func (e *Environment) Path() string {
	var p string
	if e.parent != nil {
		p = e.parent.Path()
	}
	if p != "" {
		p = fmt.Sprintf("%s/%s", p, e.block)
	} else {
		p = e.block
	}
	return p
}

func (e *Environment) Delete(str string, all bool) {
	i, ok := e.lookup[str]
	if ok {
		e.values = append(e.values[:i], e.values[i+1:]...)
	}
	if all && e.parent != nil {
		e.parent.Delete(str, all)
	}
}

func (e *Environment) Resolve(str string) (Value, error) {
	i, ok := e.lookup[str]
	if ok {
		return e.values[i], nil
	}
	if e.parent == nil {
		return nil, fmt.Errorf("%s: value not defined", str)
	}
	return e.parent.Resolve(str)
}

func (e *Environment) Define(v Value) {
	e.lookup[v.String()] = len(e.values)
	e.values = append(e.values, v)
}

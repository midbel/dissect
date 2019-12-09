package dissect

import (
	"fmt"
)

type Environment struct {
  parent *Environment
  
	block  string
	values map[string]Value
}

func NewEnvironment(str string) *Environment {
	return NewEnclosedEnvironment(str, nil)
}

func NewEnclosedEnvironment(str string, parent *Environment) *Environment {
	e := Environment{
		block:  str,
		parent: parent,
		values: make(map[string]Value),
	}
	return &e
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
	delete(e.values, str)
	if all && e.parent != nil {
		e.parent.Delete(str, all)
	}
}

func (e *Environment) Get(str string) (Value, error) {
	v, ok := e.values[str]
	if ok {
		return v, nil
	}
	if e.parent == nil {
		return nil, fmt.Errorf("%s: value not defined", str)
	}
	return e.parent.Get(str)
}

func (e *Environment) Set(v Value) {
	e.values[v.String()] = v
}

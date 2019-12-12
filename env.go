package dissect

import (
	"fmt"
)

type Environment struct {
	parent *Environment

	block  string
	lookup map[string]int // map[string][]int
	values []Field
}

func NewEnvironment(str string) *Environment {
	return NewEnclosedEnvironment(str, nil)
}

func NewEnclosedEnvironment(str string, parent *Environment) *Environment {
	e := Environment{
		block:  str,
		parent: parent,
	}
	e.Reset()
	return &e
}

func (e *Environment) List() []Field {
	return e.values
}

func (e *Environment) Reset() {
	e.values = make([]Field, 0, 256)
	e.lookup = make(map[string]int)
}

func (e *Environment) Len() int {
	var i int
	if e.parent != nil {
		i = e.parent.Len()
	}
	return i + len(e.values)
}

func (e *Environment) Path() string {
	p := e.block
	if e.parent != nil {
		p = fmt.Sprintf("%s/%s", e.parent.Path(), p)
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

func (e *Environment) Resolve(str string) (Field, error) {
	i, ok := e.lookup[str]
	if ok {
		return e.values[i], nil
	}
	if e.parent == nil {
		return Field{}, fmt.Errorf("%s: field not defined", str)
	}
	return e.parent.Resolve(str)
}

func (e *Environment) Define(f Field) {
	e.lookup[f.String()] = len(e.values)
	e.values = append(e.values, f)
}

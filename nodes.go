package main

type Node interface {
	Pos() Position
}

type Expression interface {
	Eval(Set) bool
}

type Value interface{}

type Set struct {
	values map[string]Value
}

type Constant struct {
	id    Token
	value Token
}

func (c Constant) Pos() Position {
	return c.id.pos
}

type Block struct {
	id    Token
	nodes []Node
}

func (b Block) Pos() Position {
	return b.id.pos
}

type Parameter struct {
	id    Token
	props map[Token]Token
}

func (p Parameter) Pos() Position {
	return p.id.pos
}

type Reference struct {
	id Token
}

func (r Reference) Pos() Position {
	return r.id.pos
}

type Include struct {
	pos       Position
	Predicate Expression
	nodes     []Node
}

func (i Include) Pos() Position {
	return i.pos
}

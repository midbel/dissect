package dissect

import (
	"fmt"
)

type Node interface {
	Pos() Position
}

type Expression interface {
	Eval(Set) bool
}

type empty struct{}

func (e empty) Eval(_ Set) bool {
	return true
}

type Value interface{}

type Set struct {
	values map[string]Value
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
	node      Node
}

func (i Include) Pos() Position {
	return i.pos
}

type Constant struct {
	id    Token
	value Token
}

func (c Constant) Pos() Position {
	return c.id.pos
}

type Pair struct {
	id    Token
	kind  Token
	nodes []Node
}

func (p Pair) Pos() Position {
	return p.id.Pos()
}

type Block struct {
	id    Token
	nodes []Node
}

func emptyBlock(id Token) Block {
	return Block{id: id}
}

func (b Block) String() string {
	return b.id.Literal
}

func (b Block) Pos() Position {
	return b.id.pos
}

func (b Block) blockName() string {
	if b.id.Type == Keyword {
		return b.id.Literal
	} else {
		return kwBlock
	}
}

func (b Block) isData() bool {
	return b.id.Type == Keyword && b.id.Literal == kwData
}

func (b Block) Merge() (Node, error) {
	for _, n := range b.nodes {
		bck, ok := n.(Block)
		if !ok {
			continue
		}
		if bck.isData() {
			return traverse(bck, b)
		}
	}
	return nil, fmt.Errorf("data block not found")
}

func (b Block) findParam(str string) (Parameter, error) {
	for _, n := range b.nodes {
		bck, ok := n.(Block)
		if !ok {
			continue
		}
		if bck.id.Literal == kwDeclare && bck.id.Type == Keyword {
			return bck.findParameter(str)
		}
	}
	return Parameter{}, fmt.Errorf("%s: parameter not declared", str)
}

func (b Block) findBlock(str string) (Block, error) {
	for _, n := range b.nodes {
		bck, ok := n.(Block)
		if !ok {
			continue
		}
		if bck.id.Literal == str {
			return bck, nil
		}
	}
	return Block{}, fmt.Errorf("%s: block not declared", str)
}

func (b Block) findParameter(str string) (Parameter, error) {
	for _, n := range b.nodes {
		p, ok := n.(Parameter)
		if !ok {
			continue
		}
		if p.id.Literal == str {
			return p, nil
		}
	}
	return Parameter{}, fmt.Errorf("%s: parameter not declared", str)
}

func traverse(dat Block, root Block) (Node, error) {
	nodes := make([]Node, 0, len(dat.nodes))
	for i, n := range dat.nodes {
		switch n := n.(type) {
		case Parameter:
			nodes = append(nodes, n)
		case Reference:
			p, err := root.findParam(n.id.Literal)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, p)
		case Include:
			x, err := traverseInclude(n, root)
			if err != nil {
				return nil, err
			}
			if n, ok := x.(Block); ok {
				nodes = append(nodes, n.nodes...)
			} else {
				nodes = append(nodes, x)
			}
		case Block:
			dat.nodes[i], _ = traverse(n, root)
		}
	}
	dat.nodes = nodes
	return dat, nil
}

func traverseInclude(i Include, root Block) (Node, error) {
	switch n := i.node.(type) {
	case Reference:
		b, err := root.findBlock(n.id.Literal)
		if err != nil {
			return nil, err
		}
		i.node, err = traverse(b, root)
		if err != nil {
			return nil, err
		}
	case Block:
		x, err := traverse(n, root)
		if err != nil {
			return nil, err
		}
		i.node = x
	default:
		return i, fmt.Errorf("unexpected node type %T", i.node)
	}
	node := i
	if i.Predicate == nil {
		node = i.node
	}
	return node, nil
}

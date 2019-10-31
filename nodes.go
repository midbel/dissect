package dissect

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

func (b Block) blockName() string {
	if b.id.Type == Keyword {
		return b.id.Literal
	} else {
		return kwBlock
	}
}

func (b Block) Pos() Position {
	return b.id.pos
}

func (b Block) isData() bool {
	return b.id.Type == Keyword && b.id.Literal == kwData
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

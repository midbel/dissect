package dissect

import (
	"fmt"
	"strconv"
	"strings"
)

type Negate struct {
	Right Node
}

func (n Negate) String() string {
	return fmt.Sprintf("!(%s)", n.Right)
}

func (n Negate) Pos() Position {
	return n.Right.Pos()
}

type Predicate struct {
	Left     Node
	Right    Node
	operator rune
}

func (p Predicate) String() string {
	var b strings.Builder

	b.WriteRune(lparen)
	if p.Left == nil {
		b.WriteString("left")
	} else {
		b.WriteString(p.Left.String())
	}
	b.WriteRune(space)

	switch p.operator {
	case Equal:
		b.WriteString("==")
	case NotEq:
		b.WriteString("!=")
	case Lesser:
		b.WriteString("<")
	case LessEq:
		b.WriteString("<=")
	case Greater:
		b.WriteString(">")
	case GreatEq:
		b.WriteString(">=")
	case Or:
		b.WriteString("||")
	case And:
		b.WriteString("&&")
	default:
		return "<unknown>"
	}
	b.WriteRune(space)
	if p.Right == nil {
		b.WriteString("right")
	} else {
		b.WriteString(p.Right.String())
	}
	b.WriteRune(rparen)

	return b.String()
}

func (p Predicate) Pos() Position {
	return p.Left.Pos()
}

type DelStmt struct {
	pos   Position
	nodes []Node
}

func (d DelStmt) String() string {
	return "delete"
}

func (d DelStmt) Pos() Position {
	return d.pos
}

type LetStmt struct {
	id   Token
	expr Node
}

func (t LetStmt) String() string {
	return t.id.Literal
}

func (t LetStmt) Pos() Position {
	return t.id.Pos()
}

type Parameter struct {
	id    Token
	props map[string]Token
}

func (p Parameter) String() string {
	return p.id.Literal
}

func (p Parameter) Pos() Position {
	return p.id.pos
}

func (p Parameter) numbit() int {
	var size int
	if z, ok := p.props["numbit"]; ok {
		size, _ = strconv.Atoi(z.Literal)
	}
	if z, ok := p.props["type"]; ok && size == 0 {
		size, _ = strconv.Atoi(z.Literal[1:])
	}
	if size == 0 {
		size++
	}
	return size
}

func (p Parameter) is() byte {
	z, ok := p.props["type"]
	if !ok {
		return 'u'
	}
	return z.Literal[0]
}

func (p Parameter) offset() int {
	var offset int
	if z, ok := p.props["offset"]; ok {
		offset, _ = strconv.Atoi(z.Literal)
	}
	return offset
}

type Reference struct {
	id Token
}

func (r Reference) String() string {
	return r.id.Literal
}

func (r Reference) Pos() Position {
	return r.id.pos
}

type Include struct {
	pos       Position
	Predicate Node
	node      Node
}

func (i Include) String() string {
	return fmt.Sprintf("include(%s)", i.node.String())
}

func (i Include) Pos() Position {
	return i.pos
}

type Constant struct {
	id    Token
	value Token
}

func (c Constant) String() string {
	return fmt.Sprintf("%s(%s)", c.id.Literal, c.value.Literal)
}

func (c Constant) Pos() Position {
	return c.id.pos
}

type Pair struct {
	id    Token
	kind  Token
	nodes []Node
}

func (p Pair) String() string {
	return fmt.Sprintf("%s(%s)", p.id.Literal, p.kind.Literal)
}

func (p Pair) Pos() Position {
	return p.id.Pos()
}

type Let struct {
	id   Token
	node Node
}

func (t Let) String() string {
	return t.node.String()
}

func (t Let) Pos() Position {
	return t.id.Pos()
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

func (b Block) ResolveData() (Block, error) {
	dat, err := b.ResolveBlock(kwData)
	if err != nil {
		return dat, err
	}
	if !dat.isData() {
		err = fmt.Errorf("data block not found")
	}
	return dat, err
}

func (b Block) ResolveBlock(block string) (Block, error) {
	for _, n := range b.nodes {
		b, ok := n.(Block)
		if !ok {
			continue
		}
		if b.id.Literal == block {
			return b, nil
		}
	}
	return Block{}, fmt.Errorf("%s: block not defined", block)
}

func (b Block) ResolveParameter(param string) (Parameter, error) {
	def, err := b.ResolveBlock(kwDeclare)
	if err != nil {
		return Parameter{}, err
	}
	for _, n := range def.nodes {
		p, ok := n.(Parameter)
		if !ok {
			continue
		}
		if p.id.Literal == param {
			return p, nil
		}
	}
	return Parameter{}, fmt.Errorf("%s: parameter not defined")
}

func (b Block) ResolvePair(pair string) (Pair, error) {
	for _, n := range b.nodes {
		p, ok := n.(Pair)
		if !ok {
			continue
		}
		if p.id.Literal == pair {
			return p, nil
		}
	}
	return Pair{}, fmt.Errorf("%s: pair not defined")
}

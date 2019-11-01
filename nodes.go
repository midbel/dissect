package dissect

import (
	"fmt"
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
	Left    Node
	Right   Node
	operand rune
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

	switch p.operand {
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

type Parameter struct {
	id    Token
	props map[Token]Token
}

func (p Parameter) String() string {
	return p.id.Literal
}

func (p Parameter) Pos() Position {
	return p.id.pos
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

package dissect

import (
	"fmt"
	"strconv"
	"strings"
)

type Literal struct {
	id Token
}

func (t Literal) String() string {
	return t.id.Literal
}

func (t Literal) Pos() Position {
	return t.id.Pos()
}

func (t Literal) exprNode() Node {
	return t
}

type Identifier struct {
	id Token
}

func (i Identifier) String() string {
	return i.id.String()
}

func (i Identifier) Pos() Position {
	return i.id.Pos()
}

func (i Identifier) exprNode() Node {
	return i
}

type Unary struct {
	Right Expression
}

func (u Unary) String() string {
	return fmt.Sprintf("!(%s)", u.Right)
}

func (u Unary) Pos() Position {
	n := u.Right.exprNode()
	return n.Pos()
}

func (u Unary) exprNode() Node {
	return u
}

type Binary struct {
	Left     Expression
	Right    Expression
	operator rune
}

func (b Binary) String() string {
	var str strings.Builder

	str.WriteRune(lparen)
	if b.Left == nil {
		str.WriteString("left")
	} else {
		str.WriteString(b.Left.String())
	}
	str.WriteRune(space)

	switch b.operator {
	case Equal:
		str.WriteString("==")
	case NotEq:
		str.WriteString("!=")
	case Lesser:
		str.WriteString("<")
	case LessEq:
		str.WriteString("<=")
	case Greater:
		str.WriteString(">")
	case GreatEq:
		str.WriteString(">=")
	case Or:
		str.WriteString("||")
	case And:
		str.WriteString("&&")
	default:
		return "<unknown>"
	}
	str.WriteRune(space)
	if b.Right == nil {
		str.WriteString("right")
	} else {
		str.WriteString(b.Right.String())
	}
	str.WriteRune(rparen)

	return str.String()
}

func (b Binary) Pos() Position {
	n := b.Left.exprNode()
	return n.Pos()
}

func (b Binary) exprNode() Node {
	return b
}

type SeekStmt struct {
	pos    Position
	offset Token
}

func (s SeekStmt) String() string {
	return "seek"
}

func (s SeekStmt) Pos() Position {
	return s.pos
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

type Repeat struct {
	pos    Position
	repeat Token
	node   Node
}

func (r Repeat) Pos() Position {
	return r.pos
}

func (r Repeat) String() string {
	return fmt.Sprintf("repeat(%s)", r.node.String())
}

type Include struct {
	pos       Position
	Predicate Expression
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

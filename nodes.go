package dissect

import (
	"fmt"
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

func (t Literal) isBoolean() bool {
	return false
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

func (i Identifier) isBoolean() bool {
	return false
}

type Unary struct {
	operator rune
	Right    Expression
}

func (u Unary) String() string {
	switch u.operator {
	case Not:
		return fmt.Sprintf("!(%s)", u.Right)
	case Min:
		return fmt.Sprintf("-(%s)", u.Right)
	default:
		return "<unknown>"
	}
}

func (u Unary) Pos() Position {
	n := u.Right.exprNode()
	return n.Pos()
}

func (u Unary) exprNode() Node {
	return u
}

func (u Unary) isBoolean() bool {
	return u.operator == Not
}

type Assignment struct {
	left  Identifier
	right Expression
}

func (a Assignment) Pos() Position {
	return a.left.Pos()
}

func (a Assignment) String() string {
	var b strings.Builder

	b.WriteRune(lparen)
	b.WriteString(a.left.String())
	b.WriteRune(space)
	b.WriteRune(equal)
	b.WriteRune(space)
	b.WriteString(a.right.String())
	b.WriteRune(rparen)

	return b.String()
}

func (a Assignment) exprNode() Node {
	return a
}

func (a Assignment) isBoolean() bool {
	return false
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
	case Add:
		str.WriteString("+")
	case Min:
		str.WriteString("-")
	case Div:
		str.WriteString("/")
	case Mul:
		str.WriteString("*")
	case BitOr:
		str.WriteString("|")
	case BitAnd:
		str.WriteString("&")
	case ShiftLeft:
		str.WriteString("<<")
	case ShiftRight:
		str.WriteString(">>")
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

func (b Binary) isBoolean() bool {
	switch b.operator {
	case Equal, NotEq, Lesser, Greater, LessEq, GreatEq, And, Or:
		return true
	default:
		return false
	}
}

type Ternary struct {
	pos  Position
	cond Expression
	csq  Expression
	alt  Expression
}

func (t Ternary) Pos() Position {
	return t.pos
}

func (t Ternary) String() string {
	var b strings.Builder

	b.WriteRune(lparen)
	b.WriteString(t.cond.String())
	b.WriteRune(space)
	b.WriteRune(question)
	b.WriteRune(space)
	b.WriteString(t.csq.String())
	b.WriteRune(space)
	b.WriteRune(colon)
	b.WriteRune(space)
	b.WriteString(t.alt.String())
	b.WriteRune(rparen)

	return b.String()
}

func (t Ternary) exprNode() Node {
	return t
}

func (t Ternary) isBoolean() bool {
	return true
}

type Echo struct {
	pos  Position
	file Token
	expr []Expression
}

func (e Echo) Pos() Position {
	return e.pos
}

func (e Echo) String() string {
	var buf strings.Builder
	for _, x := range e.expr {
		switch x := x.(type) {
		case Literal:
			buf.WriteString(x.id.String())
		default:
			buf.WriteRune(modulo)
			buf.WriteRune(lsquare)
			buf.WriteString(x.String())
			buf.WriteRune(rsquare)
		}
	}
	return buf.String() // "echo"
}

// type Note struct {
// 	tok Token
// }
//
// func (n Note) Pos() Position {
// 	return n.tok.Pos()
// }
//
// func (n Note) String() string {
// 	return n.tok.String()
// }

type Member struct {
	ref  Token
	attr Token
}

func (m Member) Pos() Position {
	return m.ref.Pos()
}

func (m Member) String() string {
	return fmt.Sprintf("%s.%s", m.ref.Literal, m.attr.Literal)
}

func (m Member) exprNode() Node {
	return m
}

func (m Member) isBoolean() bool {
	return false
}

type Print struct {
	pos    Position
	file   Token
	method Token // eng, raw, both, debug (default)
	format Token // csv,...
	values []Token
}

func (p Print) Pos() Position {
	return p.pos
}

func (p Print) String() string {
	return p.file.Literal
}

type Continue struct {
	pos  Position
	expr Expression
}

func (c Continue) Pos() Position {
	return c.pos
}

func (c Continue) String() string {
	if c.expr == nil {
		return "continue"
	}
	return fmt.Sprintf("continue(%s)", c.expr)
}

type Break struct {
	pos  Position
	expr Expression
}

func (b Break) Pos() Position {
	return b.pos
}

func (b Break) String() string {
	if b.expr == nil {
		return "break"
	}
	return fmt.Sprintf("break(%s)", b.expr)
}

type Exit struct {
	pos  Position
	code Token
}

func (e Exit) String() string {
	return "exit"
}

func (e Exit) Pos() Position {
	return e.pos
}

type Peek struct {
	pos   Position
	count Expression
}

func (p Peek) Pos() Position {
	return p.pos
}

func (p Peek) String() string {
	return fmt.Sprintf("peek(%s)", p.count)
}

type Seek struct {
	pos      Position
	offset   Expression
	absolute bool
}

func (s Seek) String() string {
	return "seek"
}

func (s Seek) Pos() Position {
	return s.pos
}

type Del struct {
	pos   Position
	nodes []Node
}

func (d Del) String() string {
	return "delete"
}

func (d Del) Pos() Position {
	return d.pos
}

type Let struct {
	id   Token
	expr Expression
}

func (t Let) String() string {
	return t.id.Literal
}

func (t Let) Pos() Position {
	return t.id.Pos()
}

type Parameter struct {
	id     Token
	size   Token
	kind   Token
	endian Token
	apply  Node
	expect Expression
}

func (p Parameter) String() string {
	return p.id.Literal
}

func (p Parameter) Pos() Position {
	return p.id.pos
}

func (p Parameter) is() Kind {
	switch p.kind.Literal {
	default:
		return kindInt
	case kwUint:
		return kindUint
	case kwFloat:
		return kindFloat
	case kwString:
		return kindString
	case kwBytes:
		return kindBytes
	}
}

type Alias struct {
	id    Token
	alias Token
}

func (a Alias) String() string {
	return a.id.Literal
}

func (a Alias) Pos() Position {
	return a.id.pos
}

type Reference struct {
	id    Token
	alias Token
}

func (r Reference) String() string {
	return r.id.Literal
}

func (r Reference) Pos() Position {
	return r.id.pos
}

type MatchCase struct {
	// cond Token
	cond Expression
	node Node
}

func (m MatchCase) isDefault() bool {
	return m.cond == nil
}

func (m MatchCase) Pos() Position {
	if m.cond == nil {
		return Position{}
	}
	n := m.cond.exprNode()
	return n.Pos()
}

func (m MatchCase) String() string {
	return m.cond.String()
}

type Match struct {
	pos   Position
	expr  Expression
	nodes []MatchCase
	alt   MatchCase
}

func (m Match) Pos() Position {
	return m.pos
}

func (m Match) String() string {
	return fmt.Sprintf("match(%s)", m.expr)
}

type If struct {
	pos  Position
	expr Expression
	csq  Node
	alt  Node
}

func (i If) Pos() Position {
	return i.pos
}

func (i If) String() string {
	return i.expr.String()
}

type Repeat struct {
	pos    Position
	repeat Expression
	node   Node
}

func (r Repeat) Pos() Position {
	return r.pos
}

func (r Repeat) String() string {
	return fmt.Sprintf("repeat(%s)", r.node.String())
}

type Include struct {
	pos  Position
	cond Expression
	node Node
}

func (i Include) String() string {
	return fmt.Sprintf("include(%s)", i.node.String())
}

func (i Include) Pos() Position {
	return i.pos
}

type Constant struct {
	id    Token
	value Expression // Token
}

func (c Constant) String() string {
	return fmt.Sprintf("%s(%s)", c.id.Literal, c.value)
}

func (c Constant) Pos() Position {
	return c.id.pos
}

type Pair struct {
	id    Token
	kind  Token
	nodes []Constant
}

func (p Pair) String() string {
	return fmt.Sprintf("%s(%s)", p.id.Literal, p.kind.Literal)
}

func (p Pair) Pos() Position {
	return p.id.Pos()
}

type Data struct {
	Block
	files []Token
}

type Block struct {
	id    Token
	nodes []Node

	pre  Node
	post Node
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

func (b Block) ResolveData() (Data, error) {
	for _, n := range b.nodes {
		if dat, ok := n.(Data); ok {
			return dat, nil
		}
	}
	return Data{}, fmt.Errorf("data block not found")
}

func (b Block) GetAlias() []Alias {
	as := make([]Alias, 0, len(b.nodes))
	for _, n := range b.nodes {
		if a, ok := n.(Alias); ok {
			as = append(as, a)
		}
	}
	return as
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
	return Parameter{}, fmt.Errorf("%s: parameter not defined", param)
}

func (b Block) ResolveConstant(cst string) (Constant, error) {
	def, err := b.ResolveBlock(kwDefine)
	if err != nil {
		return Constant{}, err
	}
	for _, n := range def.nodes {
		c, ok := n.(Constant)
		if !ok {
			continue
		}
		if c.id.Literal == cst {
			return c, nil
		}
	}
	return Constant{}, fmt.Errorf("%s: constant not defined")
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
	return Pair{}, fmt.Errorf("%s: pair not defined", pair)
}

type typedef struct {
	label  Token
	kind   Token
	size   Token
	endian Token
}

func (t typedef) Pos() Position {
	return t.label.Pos()
}

func (t typedef) String() string {
	return fmt.Sprintf("typedef(%s)", t.label.Literal)
}

package dissect

import (
	"fmt"
	"sort"
)

const (
	EOF rune = -(iota + 1)
	Ident
	Text
	Keyword
	Integer
	Float
	Bool
	Comment
	Assign
	Cast
	Equal
	NotEq
	Lesser
	LessEq
	Greater
	GreatEq
	And
	Or
	Not
	Newline
	Illegal
)

const (
	space      = ' '
	tab        = '\t'
	lparen     = '('
	rparen     = ')'
	lsquare    = '['
	rsquare    = ']'
	comma      = ','
	colon      = ':'
	equal      = '='
	newline    = '\n'
	minus      = '-'
	underscore = '_'
	pound      = '#'
	dot        = '.'
	bang       = '!'
	langle     = '<'
	rangle     = '>'
	quote      = '"'
	ampersand  = '&'
	pipe       = '|'
)

func init() {
	sort.Strings(keywords)
}

const (
	kwEnum    = "enum"
	kwPoly    = "polynomial"
	kwPoint   = "pointpair"
	kwBlock   = "block"
	kwImport  = "import"
	kwInclude = "include"
	kwRepeat  = "repeat"
	kwData    = "data"
	kwDeclare = "declare"
	kwDefine  = "define"
	kwInline  = "inline"
	kwLet     = "let"
	kwDel     = "del"
	kwSeek    = "seek"
	kwTrue    = "true"
	kwFalse   = "false"
	kwAno     = "anonymous"
)

var keywords = []string{
	kwEnum,
	kwPoly,
	kwPoint,
	kwBlock,
	kwImport,
	kwInclude,
	kwData,
	kwDeclare,
	kwDefine,
	kwLet,
	kwDel,
	kwSeek,
	kwRepeat,
}

type Expression interface {
	fmt.Stringer
	exprNode() Node
}

type Node interface {
	Pos() Position
	fmt.Stringer
}

type Position struct {
	Line   int
	Column int
}

func (p Position) IsValid() bool {
	return p.Line > 0
}

func (p Position) String() string {
	return fmt.Sprintf("%d:%d", p.Line, p.Column)
}

type Token struct {
	Literal string
	Type    rune
	pos     Position
}

func (t Token) Pos() Position {
	return t.pos
}

func (t Token) String() string {
	switch t.Type {
	case EOF:
		return "eof"
	case And:
		return "&&"
	case Or:
		return "||"
	case Assign:
		return "="
	case Not:
		return "!"
	case Equal:
		return "=="
	case NotEq:
		return "!="
	case Lesser:
		return "<"
	case LessEq:
		return "<="
	case Greater:
		return ">"
	case GreatEq:
		return ">="
	case Cast:
		return "cast"
	case Ident, Float, Integer, Bool, Keyword:
		return t.Literal
	default:
		return string(t.Type)
	}
}

func (t Token) isBool() bool {
	return t.Type == Bool
}

func (t Token) isNumber() bool {
	return t.Type == Integer || t.Type == Float
}

func (t Token) isIdent() bool {
	return t.Type == Ident || t.Type == Text
}

func (t Token) isLogical() bool {
	return t.Type == And || t.Type == Or || t.Type == Not
}

func (t Token) isComparison() bool {
	switch t.Type {
	case Equal, NotEq, Lesser, LessEq, Greater, GreatEq:
	default:
		return false
	}
	return true
}

func TokenString(t Token) string {
	var (
		str string
		lit = t.Literal
	)
	switch t.Type {
	case Not:
		return "<not>"
	case And:
		return "<and>"
	case Or:
		return "<or>"
	case EOF:
		return "<eof>"
	case Ident:
		str = "ident"
	case Keyword:
		str = "keyword"
	case Text:
		str = "text"
	case Integer:
		str = "integer"
	case Float:
		str = "float"
	case Bool:
		str = "bool"
	case Comment:
		str = "comment"
	case Assign:
		return "assignment"
	case Equal:
		return "<equal>"
	case NotEq:
		return "<notequal>"
	case Lesser:
		return "<lesser>"
	case LessEq:
		return "<lesseq>"
	case Greater:
		return "<greater>"
	case GreatEq:
		return "<greateq>"
	case Cast:
		return "<cast>"
	case Newline:
		return "<newline>"
	case Illegal:
		str = "illegal"
	default:
		str = "punct"
		lit = string(t.Type)
	}
	return fmt.Sprintf("<%s(%s)>", str, lit)
}

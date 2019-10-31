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
	Equal
	NotEq
	Lesser
	LessEq
	Greater
	GreatEq
	And
	Or
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
	kwData    = "data"
	kwDeclare = "declare"
	kwDefine  = "define"
	kwInline  = "inline"
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
	var (
		str string
		lit = t.Literal
	)
	switch t.Type {
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

func (t Token) isIdent() bool {
	return t.Type == Ident || t.Type == Text
}

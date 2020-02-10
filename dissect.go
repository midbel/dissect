package dissect

import (
	"fmt"
	"sort"
	"strconv"
)

const (
	EOF rune = -(iota + 1)
	Ident
	Text
	Keyword
	Internal
	Integer
	Float
	Bool
	Comment
	Assign
	Equal
	NotEq
	Lesser
	LessEq
	Greater
	GreatEq
	Add
	Mul
	Div
	Min
	Modulo
	And
	Or
	Not
	Cond
	ShiftLeft
	ShiftRight
	BitAnd
	BitOr
	Newline
	Illegal
)

const (
	dollar     = '$'
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
	underscore = '_'
	pound      = '#'
	dot        = '.'
	bang       = '!'
	langle     = '<'
	rangle     = '>'
	quote      = '"'
	ampersand  = '&'
	pipe       = '|'
	minus      = '-'
	add        = '+'
	mul        = '*'
	div        = '/'
	question   = '?'
	modulo     = '%'
)

func init() {
	sort.Strings(keywords)
}

type ExitError struct {
	code int64
}

func (e *ExitError) Error() string {
	return strconv.Itoa(int(e.code))
}

type Kind uint8

func (k Kind) String() string {
	switch k {
	default:
		return "<kind:unknown>"
	case kindInt:
		return kwInt
	case kindUint:
		return kwUint
	case kindFloat:
		return kwFloat
	case kindString:
		return kwString
	case kindBytes:
		return kwBytes
	case kindTime:
		return kwTime
	case kindGPS:
		return fmt.Sprintf("time(%s)", kwGPS)
	case kindUnix:
		return fmt.Sprintf("time(%s)", kwUnix)
	}
}

const (
	methRaw   = "raw"
	methEng   = "eng"
	methBoth  = "both"
	methDebug = "debug"
	methId    = "id"
	methPos   = "pos"
)

const (
	fmtCSV   = "csv"
	fmtTuple = "tuple"
	fmtSexp  = "sexp"
)

const (
	kindNull Kind = iota
	kindInt
	kindUint
	kindFloat
	kindString
	kindBytes
	kindTime
	kindGPS
	kindUnix
)

const (
	kwEnum     = "enum"
	kwPoly     = "polynomial"
	kwPoint    = "pointpair"
	kwBlock    = "block"
	kwTypdef   = "typedef"
	kwAlias    = "alias"
	kwInclude  = "include"
	kwRepeat   = "repeat"
	kwData     = "data"
	kwDeclare  = "declare"
	kwDefine   = "define"
	kwBreak    = "break"
	kwContinue = "continue"
	kwPrint    = "print"
	kwEcho     = "echo"
	kwInline   = "inline"
	kwLet      = "let"
	kwDel      = "del"
	kwSeek     = "seek"
	kwPeek     = "peek"
	kwTrue     = "true"
	kwFalse    = "false"
	kwAno      = "anonymous"
	kwExit     = "exit"
	kwInt      = "int"
	kwUint     = "uint"
	kwFloat    = "float"
	kwString   = "string"
	kwBytes    = "bytes"
	kwTime     = "time"
	kwMatch    = "match"
	kwWith     = "with"
	kwAs       = "as"
	kwAt       = "at"
	kwTo       = "to"
	kwBig      = "big"
	kwLittle   = "little"
	kwUnix     = "unix"
	kwGPS      = "gps"
	kwIf       = "if"
	kwElse     = "else"
	kwCopy     = "copy"
	kwPush     = "push"
)

var keywords = []string{
	kwEnum,
	kwPoly,
	kwPoint,
	kwAlias,
	kwBlock,
	kwTypdef,
	kwInclude,
	kwData,
	kwDeclare,
	kwDefine,
	kwPrint,
	kwEcho,
	kwBreak,
	kwContinue,
	kwLet,
	kwDel,
	kwSeek,
	kwPeek,
	kwRepeat,
	kwExit,
	kwInt,
	kwUint,
	kwFloat,
	kwString,
	kwBytes,
	kwTime,
	kwMatch,
	kwWith,
	kwAs,
	kwAt,
	kwTo,
	kwBig,
	kwLittle,
	kwUnix,
	kwGPS,
	kwIf,
	kwElse,
	kwCopy,
	kwPush,
}

type Expression interface {
	fmt.Stringer
	exprNode() Node
	isBoolean() bool
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
	case ShiftLeft:
		return "<<"
	case ShiftRight:
		return ">>"
	case BitOr:
		return "|"
	case BitAnd:
		return "&"
	case Cond:
		return "?:"
	case Add:
		return "+"
	case Min:
		return "-"
	case Mul:
		return "*"
	case Div:
		return "/"
	case Modulo:
		return "%"
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
	case Ident, Text, Float, Integer, Bool, Keyword, Internal:
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

func (t Token) isArithmetic() bool {
	return t.Type == Add || t.Type == Min || t.Type == Div || t.Type == Mul
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
	if lit == "" {
		lit = "???"
	}
	switch t.Type {
	case Modulo:
		return "<modulo>"
	case BitOr:
		return "<bor>"
	case BitAnd:
		return "<band>"
	case ShiftLeft:
		return "<shift left>"
	case ShiftRight:
		return "<shift right>"
	case Cond:
		return "<conditional>"
	case Add:
		return "<add>"
	case Min:
		return "<subtract>"
	case Div:
		return "<divide>"
	case Mul:
		return "<multiply>"
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
	case Internal:
		str = "internal"
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

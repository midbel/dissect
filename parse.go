package dissect

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
)

var (
	ErrUnexpected = errors.New("unexpected token")
	ErrSyntax     = errors.New("syntax error")
)

const (
	bindLowest int = iota
	bindAssign
	bindCond
	bindOr
	bindAnd
	bindBitOr
	bindBitAnd
	bindEq
	bindRel
	bindShift
	bindSum
	bindMul
	bindUnary
)

var bindings = map[rune]int{
	Assign:     bindAssign,
	Equal:      bindEq,
	NotEq:      bindEq,
	Lesser:     bindRel,
	LessEq:     bindRel,
	Greater:    bindRel,
	GreatEq:    bindRel,
	And:        bindAnd,
	Or:         bindOr,
	Add:        bindSum,
	Min:        bindSum,
	Mul:        bindMul,
	Div:        bindMul,
	Modulo:     bindMul,
	Cond:       bindCond,
	ShiftLeft:  bindShift,
	ShiftRight: bindShift,
}

func bindPower(tok Token) int {
	pw := bindLowest
	if p, ok := bindings[tok.Type]; ok {
		pw = p
	}
	return pw
}

type Parser struct {
	frames []*frame

	curr Token
	peek Token

	typedef map[string]typedef

	stmts  map[string]func() (Node, error)
	kwords map[string]func() (Node, error)
	blocks []string

	inline int
}

func Parse(r io.Reader) (Node, error) {
	var p Parser
	p.kwords = map[string]func() (Node, error){
		kwInclude: p.parseImport,
		kwData:    p.parseData,
		kwBlock:   p.parseBlock,
		kwEnum:    p.parsePair,
		kwPoint:   p.parsePair,
		kwPoly:    p.parsePair,
		kwDeclare: p.parseDeclare,
		kwDefine:  p.parseDefine,
		kwTypdef:  p.parseTypedef,
		kwAlias:   p.parseAlias,
	}
	p.stmts = map[string]func() (Node, error){
		kwInclude:  p.parseInclude,
		kwLet:      p.parseLet,
		kwDel:      p.parseDel,
		kwSeek:     p.parseSeek,
		kwPeek:     p.parsePeek,
		kwRepeat:   p.parseRepeat,
		kwExit:     p.parseExit,
		kwMatch:    p.parseMatch,
		kwBreak:    p.parseBreak,
		kwContinue: p.parseContinue,
		kwPrint:    p.parsePrint,
		kwEcho:     p.parseEcho,
		kwIf:       p.parseIf,
		kwCopy:     p.parseCopy,
		kwPush:     p.parsePush,
	}
	p.typedef = make(map[string]typedef)
	if err := p.pushFrame(r); err != nil {
		return nil, err
	}

	p.nextToken()
	p.nextToken()

	return p.Parse()
}

func (p *Parser) Parse() (Node, error) {
	var root Block
	for {
		p.skipComment()
		if p.isDone() {
			break
		}
		if p.curr.Type != Keyword {
			return nil, p.unexpectedError()
		}
		parse, ok := p.kwords[p.curr.Literal]
		if !ok {
			return nil, p.unexpectedError()
		}
		p.pushBlock(p.curr.Literal)
		n, err := parse()
		if err != nil {
			return nil, err
		}
		p.popBlock()
		if n != nil {
			root.nodes = append(root.nodes, n)
		}
	}
	return root, nil
}

func (p *Parser) parsePush() (Node, error) {
	h := Push{
		pos: p.curr.Pos(),
	}
	p.nextToken()
	if !p.curr.isIdent() {
		return nil, p.expectedError("ident")
	}
	h.id = p.curr
	p.nextToken()
	if p.curr.Type == Keyword {
		if p.curr.Literal != kwIf {
			return nil, p.unexpectedError()
		}
		p.nextToken()
		e, err := p.parsePredicate()
		if err != nil {
			return nil, err
		}
		h.expr = e
	}
	return h, nil
}

func (p *Parser) parseCopy() (Node, error) {
	c := Copy{
		pos:    p.curr.Pos(),
		file:   Token{Literal: "-", Type: Ident},
		format: Token{Literal: kwBytes, Type: Keyword},
	}
	p.nextToken()
	if p.curr.Type != lsquare {
		return nil, p.expectedError("[")
	}
	p.nextToken()
	e, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	c.count = e

	switch p.curr.Type {
	case Keyword:
		if kw := p.curr.Literal; kw == kwTo {
			err = p.parseCopyTo(&c)
		} else if kw == kwAs {
			err = p.parseCopyAs(&c)
		} else if kw == kwIf {
			err = p.parseCopyIf(&c)
		} else {
			err = p.unexpectedError()
		}
	case Newline:
	default:
		err = p.unexpectedError()
	}
	return c, err
}

func (p *Parser) parseCopyTo(c *Copy) error {
	if p.curr.Literal != kwTo {
		return p.expectedError(kwTo)
	}
	p.nextToken()
	if !p.curr.isIdent() {
		return p.expectedError("ident")
	}
	c.file = p.curr
	p.nextToken()

	switch p.curr.Type {
	case Keyword:
		if kw := p.curr.Literal; kw == kwAs {
			return p.parseCopyAs(c)
		} else if kw == kwIf {
			return p.parseCopyIf(c)
		} else {
			return p.unexpectedError()
		}
	case Newline:
	default:
		return p.unexpectedError()
	}
	return nil
}

func (p *Parser) parseCopyAs(c *Copy) error {
	if p.curr.Literal != kwTo {
		return p.expectedError(kwTo)
	}
	p.nextToken()
	if p.curr.Type != Keyword {
		return p.unexpectedError()
	}
	switch p.curr.Literal {
	case kwString, kwBytes:
		c.format = Token{Literal: p.curr.Literal}
	default:
		return p.unexpectedError()
	}
	p.nextToken()

	if p.curr.Type == Keyword {
		return p.parseCopyIf(c)
	}
	return nil
}

func (p *Parser) parseCopyIf(c *Copy) error {
	if p.curr.Literal != kwIf {
		return p.expectedError(kwIf)
	}
	p.nextToken()
	e, err := p.parsePredicate()
	if err == nil {
		c.predicate = e
	}
	return err
}

func (p *Parser) isClosed() error {
	if p.curr.Type != rparen {
		return p.expectedError(")")
	}
	p.nextToken()
	return nil
}

func (p *Parser) parseAlias() (Node, error) {
	p.nextToken()
	if !p.curr.isIdent() {
		return nil, p.expectedError("ident")
	}
	r := Reference{id: p.curr}
	p.nextToken()
	if p.curr.Type != Assign {
		return nil, p.expectedError("=")
	}
	p.nextToken()
	if !p.curr.isIdent() {
		return nil, p.expectedError("ident")
	}
	r.alias = p.curr
	p.nextToken()
	return r, nil
}

func (p *Parser) parseEcho() (Node, error) {
	e := Echo{
		pos:  p.curr.Pos(),
		file: Token{Literal: "-"},
	}
	p.nextToken()
	if p.curr.Type != Text {
		return nil, p.expectedError("string")
	}
	es, err := p.parseEchoString()
	if err != nil {
		return nil, err
	}
	e.expr = es

	p.nextToken()
	return e, nil
}

func (p *Parser) parseEchoString() ([]Expression, error) {
	var (
		expr     []Expression
		offset   int
		template = p.curr.Literal
	)
	for {
		i := strings.IndexByte(template[offset:], lsquare)
		if i < 0 {
			break
		}
		offset += i
		if i > 0 && template[offset-1] != modulo {
			continue
		}
		tok := Token{
			Literal: template[offset-i : offset-1],
			Type:    Text,
		}
		j := strings.IndexByte(template[offset:], rsquare)
		if j < 0 {
			return nil, fmt.Errorf("echo: expression not closed %s (%s)", template, p.curr.Pos())
		}
		if j <= 1 {
			return nil, fmt.Errorf("echo: empty expression %s (%s)", template, p.curr.Pos())
		}
		e, err := parseString(template[offset+1 : offset+j])
		if err != nil {
			return nil, err
		}
		offset += j + 1
		expr = append(expr, Literal{id: tok}, e)
	}
	if str := template[offset:]; len(str) > 0 {
		tok := Token{
			Literal: template[offset:],
			Type:    Text,
		}
		expr = append(expr, Literal{id: tok})
	}
	return expr, nil
}

func (p *Parser) parsePrint() (Node, error) {
	f := Print{
		pos:    p.curr.Pos(),
		file:   Token{Literal: "-", Type: Ident},
		format: Token{Literal: fmtCSV, Type: Ident},
		method: Token{Literal: methDebug, Type: Ident},
	}
	p.nextToken()
	if p.curr.isIdent() {
		switch p.curr.Literal {
		case methBoth, methRaw, methEng, methDebug:
		default:
			return nil, p.unexpectedError()
		}
		f.method = p.curr
		p.nextToken()
	}
	if p.curr.Type == Newline {
		return f, nil
	}
	var err error
	switch p.curr.Type {
	case Keyword:
		if kw := p.curr.Literal; kw == kwTo {
			err = p.parsePrintTo(&f)
		} else if kw == kwAs {
			err = p.parsePrintAs(&f)
		} else if kw == kwWith {
			err = p.parsePrintWith(&f)
		} else if kw == kwIf {
			err = p.parsePrintIf(&f)
		} else {
			err = p.unexpectedError()
		}
	case Newline:
	default:
		err = p.unexpectedError()
	}
	return f, err
}

func (p *Parser) parsePrintTo(f *Print) error {
	if p.curr.Literal != kwTo {
		return p.expectedError(kwTo)
	}
	p.nextToken()
	if !p.curr.isIdent() {
		return p.expectedError("ident")
	}
	f.file = p.curr
	p.nextToken()
	switch p.curr.Type {
	case Keyword:
		if kw := p.curr.Literal; kw == kwAs {
			return p.parsePrintAs(f)
		} else if kw == kwWith {
			return p.parsePrintWith(f)
		} else if kw == kwIf {
			return p.parsePrintIf(f)
		} else {
			return p.unexpectedError()
		}
	case Newline:
	default:
		return p.unexpectedError()
	}
	return nil
}

func (p *Parser) parsePrintAs(f *Print) error {
	if p.curr.Literal != kwAs {
		return p.expectedError(kwAs)
	}
	p.nextToken()
	if p.curr.Type != Ident {
		return p.expectedError("ident")
	}
	switch p.curr.Literal {
	case fmtCSV, fmtTuple, fmtSexp:
		f.format = p.curr
	default:
		return fmt.Errorf("print: unknown format %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	switch p.curr.Type {
	case Keyword:
		if kw := p.curr.Literal; kw == kwWith {
			return p.parsePrintWith(f)
		} else if kw == kwIf {
			return p.parsePrintIf(f)
		} else {
			return p.unexpectedError()
		}
	case Newline:
	default:
		return p.unexpectedError()
	}
	return nil
}

func (p *Parser) parsePrintWith(f *Print) error {
	if p.curr.Literal != kwWith {
		return p.expectedError(kwWith)
	}
	p.nextToken()
	for !p.isDone() {
		if p.curr.Type == Newline || p.curr.Type == Keyword {
			break
		}
		if p.curr.Type != Ident {
			return p.expectedError("ident")
		}
		f.values = append(f.values, p.curr)
		p.nextToken()
	}
	if p.curr.Type == Keyword {
		return p.parsePrintIf(f)
	}
	return nil
}

func (p *Parser) parsePrintIf(f *Print) error {
	if p.curr.Literal != kwIf {
		return p.expectedError(kwIf)
	}
	p.nextToken()
	e, err := p.parsePredicate()
	if err == nil {
		f.predicate = e
	}
	return err
}

func (p *Parser) parseContinue() (Node, error) {
	if !p.inBlock(kwRepeat) {
		return nil, fmt.Errorf("continue: unexpected outside of repeat block (%s)", p.curr.Pos())
	}
	c := Continue{
		pos: p.curr.Pos(),
	}
	p.nextToken()
	if p.curr.Type != lsquare {
		return nil, p.expectedError("[")
	}
	p.nextToken()
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	c.expr = expr
	if p.curr.Type != Newline {
		return nil, p.expectedError("newline")
	}
	p.nextToken()
	return c, nil
}

func (p *Parser) parseBreak() (Node, error) {
	if !p.inBlock(kwRepeat) {
		return nil, fmt.Errorf("break: unexpected outside of repeat block (%s)", p.curr.Pos())
	}
	b := Break{
		pos: p.curr.Pos(),
	}
	p.nextToken()
	if p.curr.Type != lsquare {
		return nil, p.expectedError("[")
	}
	p.nextToken()
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	b.expr = expr
	if p.curr.Type != Newline {
		return nil, p.expectedError("newline")
	}
	p.nextToken()
	return b, nil
}

func (p *Parser) parseStatements() ([]Node, error) {
	if p.curr.Type != lparen {
		return nil, p.expectedError("(")
	}
	p.nextToken()

	var ns []Node
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		var (
			err  error
			node Node
		)
		switch pos := p.curr.Pos(); p.curr.Type {
		case Keyword:
			parse, ok := p.stmts[p.curr.Literal]
			if !ok {
				return nil, p.unexpectedError()
			}
			p.pushBlock(p.curr.Literal)
			node, err = parse()
			p.popBlock()
		case Ident, Text:
			node, err = p.parseField()
		case lparen:
			xs, err := p.parseStatements()
			if err != nil {
				return nil, err
			}
			id, err := p.parseBlockId()
			if err != nil {
				return nil, err
			}
			if !id.pos.IsValid() {
				id.pos = pos
			}
			b := Block{
				id:    id,
				nodes: xs,
			}
			ns = append(ns, b)
		default:
			err = p.unexpectedError()
		}
		if err != nil {
			return nil, err
		}
		if node != nil {
			ns = append(ns, node)
		}
	}
	return ns, p.isClosed()
}

func (p *Parser) parseIf() (Node, error) {
	i := If{pos: p.curr.Pos()}
	p.nextToken()
	if p.curr.Type != lsquare {
		return nil, p.expectedError("[")
	}
	p.nextToken()
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	i.expr = expr
	n, err := p.parseBody()
	if err != nil {
		return nil, err
	}
	i.csq = n

	if p.curr.Type == Keyword {
		if p.curr.Literal != kwElse {
			return nil, p.expectedError(kwElse)
		}
		p.nextToken()
		if p.curr.Literal == kwIf {
			node, err := p.parseIf()
			if err != nil {
				return nil, err
			}
			i.alt = node
		} else {
			n, err := p.parseBody()
			if err != nil {
				return nil, err
			}
			i.alt = n
		}
	}
	return i, nil
}

func (p *Parser) parseBody() (Node, error) {
	var node Node
	switch pos := p.curr.Pos(); p.curr.Type {
	case lparen:
		ns, err := p.parseStatements()
		if err != nil {
			return nil, err
		}
		id, err := p.parseBlockId()
		if err != nil {
			return nil, err
		}
		if !id.pos.IsValid() {
			id.pos = pos
		}
		node = Block{id: id, nodes: ns}
	case Ident, Text:
		n, err := p.parseReference()
		if err != nil {
			return nil, err
		}
		node = n
	default:
		return nil, p.unexpectedError()
	}
	return node, nil
}

func (p *Parser) parseRepeat() (Node, error) {
	r := Repeat{pos: p.curr.Pos()}
	p.nextToken()
	if p.curr.Type != lsquare {
		return nil, p.expectedError("[")
	}
	p.nextToken()
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	r.repeat = expr

	switch pos := p.curr.Pos(); p.curr.Type {
	case lparen:
		if ns, e := p.parseStatements(); e == nil {
			id, err := p.parseBlockId()
			if err != nil {
				return nil, err
			}
			if !id.pos.IsValid() {
				id.pos = pos
			}
			r.node = Block{id: id, nodes: ns}
		} else {
			err = e
		}
	case Ident, Text:
		r.node, err = p.parseReference()
	default:
		err = p.unexpectedError()
	}
	if err == nil {
		p.nextToken()
	}
	return r, err
}

func (p *Parser) parsePeek() (Node, error) {
	k := Peek{pos: p.curr.Pos()}
	p.nextToken()
	if p.curr.Type != lsquare {
		return nil, p.expectedError("[")
	}
	p.nextToken()
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	k.count = expr
	p.nextToken()
	return k, nil
}

func (p *Parser) parseSeek() (Node, error) {
	k := Seek{pos: p.curr.Pos()}
	p.nextToken()
	if p.curr.Type == Keyword {
		if p.curr.Literal != kwAt {
			return nil, p.expectedError(kwAt)
		}
		k.absolute = true
		p.nextToken()
	}
	if p.curr.Type != lsquare {
		return nil, p.expectedError("[")
	}
	p.nextToken()
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}

	k.offset = expr
	p.nextToken()
	return k, nil
}

func (p *Parser) parseLet() (Node, error) {
	n := Let{id: p.peek}
	p.nextToken()
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	n.expr = expr
	return n, nil
}

func (p *Parser) parseDel() (Node, error) {
	d := Del{pos: p.curr.Pos()}
	for !p.isDone() {
		p.nextToken()
		if p.curr.Type == Newline {
			break
		}
		if !p.curr.isIdent() {
			return nil, p.expectedError("ident")
		}
		d.nodes = append(d.nodes, Reference{id: p.curr})
	}
	return d, nil
}

func (p *Parser) parseData() (Node, error) {
	b := emptyBlock(p.curr)
	p.nextToken()

	var pre, post Node
	if p.curr.Type == Lesser {
		e, o, err := p.parseDiamond()
		if err != nil {
			return nil, err
		}
		pre, post = e, o
	}

	var files []Token
	for p.curr.Type != lparen {
		if !p.curr.isIdent() {
			return nil, p.expectedError("ident")
		}
		files = append(files, p.curr)
		p.nextToken()
	}

	ns, err := p.parseStatements()
	if err != nil {
		return nil, err
	}
	b.nodes = append(b.nodes, ns...)
	d := Data{
		Block: b,
		pre:   pre,
		post:  post,
		files: files,
	}
	return d, nil
}

func (p *Parser) parsePredicate() (Expression, error) {
	expr, err := p.parseExpression(bindLowest)
	if err == nil && p.peek.Type != colon {
		p.nextToken()
	}
	return expr, err
}

func (p *Parser) parseExpression(pow int) (Expression, error) {
	expr, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}
	checkPeek := func(peek rune) bool {
		return peek == rsquare || peek == Newline || peek == Comment || peek == colon
	}
	for !checkPeek(p.peek.Type) && pow < bindPower(p.peek) {
		p.nextToken()
		switch p.curr.Type {
		case Cond:
			expr, err = p.parseTernary(expr)
		case Assign:
			expr, err = p.parseAssign(expr)
		default:
			expr, err = p.parseInfix(expr)
		}
		if err != nil {
			return nil, err
		}
	}
	if p.peek.Type == rsquare {
		p.nextToken()
	}
	return expr, nil
}

func (p *Parser) parseAssign(left Expression) (Expression, error) {
	p.nextToken()
	right, err := p.parseExpression(bindLowest)
	if err != nil {
		return nil, err
	}

	switch left := left.(type) {
	case Identifier:
		a := Assignment{
			left:  left,
			right: right,
		}
		return a, nil
	default:
		return nil, p.unexpectedError()
	}
}

func (p *Parser) parseTernary(left Expression) (Expression, error) {
	t := Ternary{
		cond: left,
	}
	p.nextToken()
	csq, err := p.parseExpression(bindLowest)
	if err != nil {
		return nil, err
	}
	p.nextToken()
	if p.curr.Type != colon {
		return nil, p.expectedError(":")
	}
	p.nextToken()
	alt, err := p.parseExpression(bindLowest)
	if err != nil {
		return nil, err
	}

	t.csq, t.alt = csq, alt
	return t, nil
}

func (p *Parser) parsePrefix() (Expression, error) {
	var expr Expression
	switch p.curr.Type {
	case Not, Min:
		op := p.curr.Type
		p.nextToken()
		right, err := p.parseExpression(bindUnary)
		if err != nil {
			return nil, err
		}
		expr = Unary{
			Right:    right,
			operator: op,
		}
	case lparen:
		p.nextToken()
		n, err := p.parseExpression(bindLowest)
		if err != nil {
			return nil, err
		}
		if p.peek.Type != rparen {
			return nil, p.expectedError(")")
		}
		p.nextToken()
		expr = n
	case Integer, Float, Bool, Text:
		expr = Literal{id: p.curr}
	case Ident:
		id := p.curr
		if p.peek.Type == dot {
			p.nextToken()
			p.nextToken()
			if p.curr.Type != Ident {
				return nil, p.expectedError("ident")
			}
			expr = Member{
				id:   id,
				attr: p.curr,
			}
		} else {
			expr = Identifier{id: id}
		}
	case Internal:
		expr = Identifier{id: p.curr}
	default:
		return nil, p.unexpectedError()
	}
	return expr, nil
}

func (p *Parser) parseInfix(left Expression) (Expression, error) {
	isComparison := func(op rune) bool {
		return op == Lesser || op == Greater || op == LessEq || op == GreatEq
	}
	expr := Binary{
		Left:     left,
		operator: p.curr.Type,
	}
	var (
		op  = p.curr
		pow = bindPower(p.curr)
	)
	p.nextToken()

	right, err := p.parseExpression(pow)
	if err == nil {
		expr.Right = right
	}
	if isComparison(op.Type) && isComparison(p.peek.Type) {
		p.nextToken()
		if right, err = p.parseInfix(right); err != nil {
			return nil, err
		}
		expr = Binary{
			Left:     expr,
			Right:    right,
			operator: And,
		}
	}
	return expr, err
}

func (p *Parser) parseExit() (Node, error) {
	e := Exit{pos: p.curr.Pos()}
	p.nextToken()
	if p.curr.Type != Integer {
		return nil, p.expectedError("integer")
	}
	e.code = p.curr
	if p.peek.Type != Newline {
		return nil, p.unexpectedError()
	}
	p.nextToken()
	return e, nil
}

func (p *Parser) parseMatch() (Node, error) {
	var (
		comma bool
		match = Match{pos: p.curr.Pos()}
	)

	p.nextToken()
	if p.curr.isIdent() {
		match.expr, comma = Identifier{id: p.curr}, true
		p.nextToken()
	}

	if p.curr.Type != Keyword && p.curr.Literal != kwWith {
		return nil, p.expectedError(kwWith)
	}
	p.nextToken()
	if p.curr.Type != lparen {
		return nil, p.expectedError("(")
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}

		mcs, alt, err := p.parseMatchCase(!comma)
		if err != nil {
			return nil, err
		}
		if alt {
			if pos := match.alt.Pos(); pos.IsValid() {
				return nil, fmt.Errorf("match: default case already set (%s)", pos)
			}
			match.alt = mcs[0]
		} else {
			match.nodes = append(match.nodes, mcs...)
		}
	}
	return match, p.isClosed()
}

func (p *Parser) parseMatchCase(nocomma bool) ([]MatchCase, bool, error) {
	var (
		mcs []MatchCase
		alt bool
	)
	for !p.isDone() {
		if p.curr.Type == colon {
			break
		}
		if p.curr.Type == underscore {
			if len(mcs) > 0 {
				return nil, alt, fmt.Errorf("match: default case should be alone %s (%s)", TokenString(p.curr), p.curr.Pos())
			}
			mcs, alt = append(mcs, MatchCase{}), true
			p.nextToken()
			break
		}
		expr, err := p.parsePredicate()
		if err != nil {
			return nil, alt, err
		}

		mcs = append(mcs, MatchCase{cond: expr})
		p.nextToken()
		if p.curr.Type == comma {
			if nocomma {
				return nil, alt, p.unexpectedError()
			}
			p.nextToken()
		}
	}

	if p.curr.Type != colon {
		return nil, alt, p.expectedError(":")
	}
	p.nextToken()

	var node Node
	switch pos := p.curr.Pos(); p.curr.Type {
	case Ident, Text:
		ref, err := p.parseReference()
		if err != nil {
			return nil, alt, err
		}
		node = ref
		p.nextToken()
	case lparen:
		ns, err := p.parseStatements()
		if err != nil {
			return nil, alt, err
		}
		id, err := p.parseBlockId()
		if err != nil {
			return nil, alt, err
		}
		if !id.pos.IsValid() {
			id.pos = pos
		}
		node = Block{
			id:    id,
			nodes: ns,
		}
	default:
		return nil, alt, p.unexpectedError()
	}

	for i := range mcs {
		mcs[i].node = node
	}

	return mcs, alt, nil
}

func (p *Parser) parseInclude() (Node, error) {
	i := Include{pos: p.curr.Pos()}

	p.nextToken()
	if p.curr.Type == lsquare {
		p.nextToken()
		expr, err := p.parsePredicate()
		if err != nil {
			return nil, err
		}
		i.cond = expr
	}
	var err error
	switch pos := i.Pos(); p.curr.Type {
	case Ident, Text:
		i.node, err = p.parseReference()
	case lparen:
		if ns, e := p.parseStatements(); e == nil {
			id, err := p.parseBlockId()
			if err != nil {
				return nil, err
			}
			if !id.pos.IsValid() {
				id.pos = pos
			}
			i.node = Block{id: id, nodes: ns}
		} else {
			err = e
		}
	default:
		err = p.unexpectedError()
	}
	if err == nil {
		p.nextToken()
	}
	return i, err
}

func (p *Parser) parseFieldLong(id Token) (Node, error) {
	var (
		typok bool
		lenok bool
		a     = Parameter{id: id}
	)
	if p.curr.Type == Keyword && p.curr.Literal == kwAs {
		p.nextToken()
		switch p.curr.Literal {
		default:
		case kwInt, kwUint, kwFloat, kwString, kwBytes:
			a.kind, typok = p.curr, true
			p.nextToken()
		}
	}
	if p.curr.Type != Keyword {
		return nil, p.expectedError("keyword")
	}
	if p.curr.Literal != kwWith {
		return nil, p.expectedError(kwWith)
	}
	p.nextToken()
	if !(p.curr.isIdent() || p.curr.isNumber()) {
		return nil, p.expectedError("ident/number")
	}
	a.size, lenok = p.curr, true
	if !typok && !lenok {
		return nil, fmt.Errorf("field: type and length not set %s (%s)", TokenString(a.id), a.Pos())
	}
	p.nextToken()
	return a, nil
}

func (p *Parser) parseTypedef() (Node, error) {
	p.nextToken()
	if p.curr.Type != lparen {
		return nil, p.expectedError("(")
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		var (
			td    typedef
			typok bool
			lenok bool
		)
		if !p.curr.isIdent() {
			return nil, p.expectedError("ident")
		}
		td.label = p.curr
		p.nextToken()
		if p.curr.Type != Assign {
			return nil, p.expectedError("=")
		}
		p.nextToken()
		if p.curr.Type == Keyword {
			switch lit := p.curr.Literal; lit {
			case kwInt, kwUint, kwFloat, kwBytes, kwString:
				td.kind, typok = p.curr, true
				p.nextToken()
			default:
				return nil, p.unexpectedError()
			}
		}
		if p.curr.Type == Integer {
			td.size, lenok = p.curr, true
			p.nextToken()
		}
		if p.curr.Type == Keyword {
			if p.curr.Literal == kwBig || p.curr.Literal == kwLittle {
				td.endian = p.curr
			} else {
				return nil, p.unexpectedError()
			}
			p.nextToken()
		}
		if !typok && !lenok {
			return nil, fmt.Errorf("typdef: type and length not set %s (%s)", TokenString(td.label), td.Pos())
		}
		p.typedef[td.label.String()] = td
	}
	return nil, p.isClosed()
}

func (p *Parser) parseFieldShort(id Token) (Node, error) {
	var (
		typok bool
		lenok bool
		a     = Parameter{id: id}
	)
	p.nextToken()
	if p.curr.Type == Keyword {
		switch lit := p.curr.Literal; lit {
		case kwInt, kwUint, kwFloat, kwBytes, kwString, kwTime:
			a.kind, typok = p.curr, true
			if lit == kwTime && p.peek.Type == lparen {
				p.nextToken()
				p.nextToken()
				switch lit := p.curr.Literal; lit {
				case kwUnix, kwGPS:
					a.kind = p.curr
				default:
					return nil, p.unexpectedError()
				}
				p.nextToken()
				if p.curr.Type != rparen {
					return nil, p.unexpectedError()
				}
			}
			p.nextToken()
		default:
			return nil, p.unexpectedError()
		}
	} else if p.curr.Type == Ident {
		if td, ok := p.typedef[p.curr.Literal]; ok {
			a.kind = td.kind
			a.size = td.size
			a.endian = td.endian
		} else {
			return nil, p.unexpectedError()
		}
		p.nextToken()
		return a, nil
	}
	if p.curr.Type == Integer {
		a.size, lenok = p.curr, true
		p.nextToken()
	}
	if p.curr.Type == Keyword {
		if p.curr.Literal == kwBig || p.curr.Literal == kwLittle {
			a.endian = p.curr
		} else {
			return nil, p.unexpectedError()
		}
		p.nextToken()
	}
	if !typok && !lenok {
		return nil, fmt.Errorf("field: type and length not set %s (%s)", TokenString(a.id), a.Pos())
	}
	return a, nil
}

func (p *Parser) parseField() (node Node, err error) {
	if !p.curr.isIdent() {
		return nil, p.expectedError("ident")
	}

	id := p.curr
	p.nextToken()

	switch p.curr.Type {
	case Newline:
		node = Reference{id: id}
	case colon:
		node, err = p.parseFieldShort(id)
	case Keyword:
		node, err = p.parseFieldLong(id)
	default:
		err = p.unexpectedError()
	}
	if err != nil {
		return
	}
	if n, ok := node.(Parameter); ok {
		if p.curr.Type == comma {
			p.nextToken()
			switch p.curr.Type {
			case Text, Ident:
				n.apply = p.curr
				p.nextToken()
			case Keyword:
				apply, err := p.parsePairInline(true)
				if err != nil {
					return nil, err
				}
				n.apply = apply
			default:
				return nil, p.expectedError("ident")
			}
		}
		if p.curr.Type == Assign {
			p.nextToken()
			p.nextToken()
			expr, err := p.parsePredicate()
			if err != nil {
				return nil, err
			}
			n.expect = expr
		}
		node = n
	}
	if p.curr.Type != Newline {
		return nil, p.expectedError("newline")
	}
	return
}

func (p *Parser) parseDeclare() (Node, error) {
	b := emptyBlock(p.curr)

	p.nextToken()
	if p.curr.Type != lparen {
		return nil, p.expectedError("(")
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		n, err := p.parseField()
		if err != nil {
			return nil, err
		}
		b.nodes = append(b.nodes, n)
	}
	return b, p.isClosed()
}

func (p *Parser) parseAssignment() (Node, error) {
	node := Constant{
		id: p.curr,
	}
	p.nextToken()
	if p.curr.Type != Assign {
		return nil, p.expectedError("=")
	}
	p.nextToken()
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	switch expr.(type) {
	case Unary, Literal:
		node.value = expr
	default:
		return nil, p.expectedError("expression")
	}
	return node, nil
}

func (p *Parser) parseDefine() (Node, error) {
	b := emptyBlock(p.curr)

	p.nextToken()
	if p.curr.Type != lparen {
		return nil, p.expectedError("(")
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if !p.curr.isIdent() {
			return nil, p.unexpectedError()
		}
		n, err := p.parseAssignment()
		if err != nil {
			return nil, err
		}
		b.nodes = append(b.nodes, n.(Constant))
	}
	return b, p.isClosed()
}

func (p *Parser) parseImport() (Node, error) {
	p.nextToken()
	if p.curr.Type != lparen {
		return nil, p.expectedError("(")
	}
	p.nextToken()

	files := make([]string, 0, 64)
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if !p.curr.isIdent() {
			return nil, p.expectedError("ident")
		}
		files = append(files, p.curr.Literal)

		p.nextToken()
		switch p.curr.Type {
		case Comment:
			p.skipComment()
		case Newline:
			p.nextToken()
		default:
			return nil, p.unexpectedError()
		}
	}
	for i := 0; i < len(files); i++ {
		if infos, err := ioutil.ReadDir(files[i]); err == nil {
			for _, j := range infos {
				files = append(files, filepath.Join(files[i], j.Name()))
			}
		} else {
			r, err := os.Open(files[i])
			if err != nil {
				return nil, err
			}
			err = p.pushFrame(r)
			r.Close()
			if err != nil {
				return nil, err
			}
		}
	}
	return nil, p.isClosed()
}

func (p *Parser) parseBlock() (Node, error) {
	p.nextToken()
	if !p.curr.isIdent() {
		return nil, p.unexpectedError()
	}
	b := emptyBlock(p.curr)
	p.nextToken()

	if p.curr.Type == Lesser {
		pre, post, err := p.parseDiamond()
		if err != nil {
			return nil, err
		}
		b.pre, b.post = pre, post
	}

	ns, err := p.parseStatements()
	if err != nil {
		return nil, err
	}
	b.nodes = ns
	return b, nil
}

func (p *Parser) parseDiamond() (Node, Node, error) {
	var (
		pre  Node
		post Node
		err  error
	)
	next := func() (Node, error) {
		p.nextToken()
		if !p.curr.isIdent() {
			return nil, p.expectedError("ident")
		}
		n := Reference{
			id:    p.curr,
			alias: p.curr,
		}
		p.nextToken()
		return n, nil
	}
	p.nextToken()
	switch p.curr.Type {
	case Ident, Text:
		pre = Reference{
			id:    p.curr,
			alias: p.curr,
		}
		p.nextToken()
		if p.curr.Type != comma {
			return pre, post, p.expectedError("comma")
		}
		post, err = next()
	case comma:
		post, err = next()
	case Greater:
	default:
		err = p.unexpectedError()
	}
	if p.curr.Type != Greater {
		err = p.expectedError(">")
	}
	p.nextToken()
	return pre, post, err
}

func (p *Parser) parsePair() (Node, error) {
	return p.parsePairInline(false)
}

func (p *Parser) parsePairInline(inline bool) (Node, error) {
	kw := p.curr.Literal
	if !(kw == kwEnum || kw == kwPoly || kw == kwPoint) {
		return nil, p.unexpectedError()
	}
	a := Pair{kind: p.curr}
	p.nextToken()
	if !inline {
		if !p.curr.isIdent() {
			return nil, p.unexpectedError()
		}
		a.id = p.curr
		p.nextToken()
	}
	if p.curr.Type != lparen {
		return nil, p.expectedError("(")
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		n, err := p.parseAssignment()
		if err != nil {
			return nil, err
		}
		a.nodes = append(a.nodes, n.(Constant))
	}
	if err := p.isClosed(); err != nil {
		return nil, err
	}
	if !inline {
		return a, nil
	}
	id, err := p.parseBlockId()
	if err == nil {
		if !id.pos.IsValid() {
			id.Literal = fmt.Sprintf("%s-%s", a.kind.Literal, id.Literal)
			id.pos = a.kind.Pos()
		}
		a.id = id
	}
	return a, err
}

func (p *Parser) parseReference() (Node, error) {
	ref := Reference{id: p.curr, alias: p.curr}
	if p.peek.Type == Keyword {
		p.nextToken()
		if p.curr.Literal != kwAs {
			return nil, p.expectedError(kwAs)
		}
		p.nextToken()
		ref.alias = p.curr
	}
	p.nextToken()
	return ref, nil
}

func (p *Parser) parseBlockId() (Token, error) {
	var id Token
	if p.curr.Type == Keyword && p.curr.Literal == kwAs {
		p.nextToken()
		if !p.curr.isIdent() {
			return Token{}, p.expectedError("ident")
		}
		id = p.curr
		p.nextToken()
	} else {
		id = Token{
			Literal: fmt.Sprintf("%s-%d", kwInline, p.inline),
			Type:    Keyword,
		}
		p.inline++
	}
	return id, nil
}

func (p *Parser) isDone() bool {
	return len(p.frames) == 0 || p.curr.Type == EOF
}

func (p *Parser) skipComment() {
	p.skipToken(Newline)
	p.skipToken(Comment)
	p.skipToken(Newline)
}

func (p *Parser) skipToken(typ rune) {
	for p.curr.Type == typ {
		p.nextToken()
	}
}

func (p *Parser) inBlock(id string) bool {
	for i := len(p.blocks) - 1; i >= 0; i-- {
		if p.blocks[i] == id {
			return true
		}
	}
	return false
}

func (p *Parser) currentBlock() string {
	n := len(p.blocks)
	if n == 0 {
		return ""
	}
	return p.blocks[n-1]
}

func (p *Parser) pushBlock(id string) {
	p.blocks = append(p.blocks, id)
}

func (p *Parser) popBlock() {
	n := len(p.blocks)
	if n == 0 {
		return
	}
	p.blocks = p.blocks[:n-1]
}

func (p *Parser) nextToken() {
	n := len(p.frames)
	if n == 0 {
		return
	}
	n--

	p.curr = p.peek
	p.peek = p.frames[n].Scan()
	if n -= 1; p.peek.Type == EOF && n >= 0 {
		p.popFrame()
		p.peek = p.frames[n].Scan()
	}
}

func (p *Parser) pushFrame(r io.Reader) error {
	s, err := Scan(r)
	if err == nil {
		f := &frame{
			file:    "<input>",
			Scanner: s,
		}
		if n, ok := r.(interface{ Name() string }); ok {
			f.file = n.Name()
		}
		f.Scan()
		p.frames = append(p.frames, f)
	}
	return err
}

func (p *Parser) popFrame() {
	n := len(p.frames)
	if n == 0 {
		return
	}
	p.frames = p.frames[:n-1]
}

func (p *Parser) currentToken() Token {
	var (
		frm = p.currentFrame()
		tok Token
	)
	if frm != nil {
		tok = frm.curr
	}
	return tok
}

func (p *Parser) peekToken() Token {
	var (
		frm = p.currentFrame()
		tok Token
	)
	if frm != nil {
		tok = frm.peek
	}
	return tok
}

func (p *Parser) currentFrame() *frame {
	n := len(p.frames)
	if n == 0 {
		return nil
	}
	return p.frames[n-1]
}

func (p *Parser) expectedError(want string) error {
	if want == "" {
		return p.unexpectedError()
	}
	var (
		file  = "<input>"
		where = p.currentBlock()
	)
	if f := p.currentFrame(); f != nil {
		file = f.file
	}
	return fmt.Errorf("(%s) %s(%s): expected %s, got %s", p.curr.Pos(), where, file, want, TokenString(p.curr))
}

func (p *Parser) unexpectedError() error {
	var (
		file  = "<input>"
		where = p.currentBlock()
	)
	if f := p.currentFrame(); f != nil {
		file = f.file
	}
	return fmt.Errorf("(%s) %s(%s): %w %s", p.curr.Pos(), where, file, ErrUnexpected, TokenString(p.curr))
}

type frame struct {
	*Scanner
	file string

	curr Token
	peek Token
}

func (f *frame) Scan() Token {
	tok := f.curr
	f.curr = f.Scanner.Scan()
	return tok
}

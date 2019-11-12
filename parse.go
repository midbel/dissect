package dissect

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
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

	stmts  map[string]func() (Node, error)
	kwords map[string]func() (Node, error)
	blocks []string
}

func Parse(r io.Reader) (Node, error) {
	var p Parser
	p.kwords = map[string]func() (Node, error){
		kwData:    p.parseData,
		kwBlock:   p.parseBlock,
		kwEnum:    p.parsePair,
		kwPoint:   p.parsePair,
		kwPoly:    p.parsePair,
		kwDeclare: p.parseDeclare,
		kwDefine:  p.parseDefine,
	}
	p.stmts = map[string]func() (Node, error){
		kwInclude:  p.parseInclude,
		kwLet:      p.parseLet,
		kwDel:      p.parseDel,
		kwSeek:     p.parseSeek,
		kwRepeat:   p.parseRepeat,
		kwExit:     p.parseExit,
		kwMatch:    p.parseMatch,
		kwBreak:    p.parseBreak,
		kwContinue: p.parseContinue,
		kwPrint:    p.parsePrint,
	}
	if err := p.pushFrame(r); err != nil {
		return nil, err
	}

	p.nextToken()
	p.nextToken()

	return p.Parse()
}

func (p *Parser) Parse() (Node, error) {
	var root Block

	p.skipComment()
	if p.curr.Type == Keyword && p.curr.Literal == kwImport {
		if err := p.parseImport(); err != nil {
			return nil, err
		}
	}

	for {
		p.skipComment()
		if p.isDone() {
			break
		}
		if p.curr.Type != Keyword {
			return nil, fmt.Errorf("parse: unexpected token: %s (%s)", p.curr, p.curr.Pos())
		}
		parse, ok := p.kwords[p.curr.Literal]
		if !ok {
			return nil, fmt.Errorf("parse: unknown keyword: %s (ùs)", p.curr.Literal, p.curr.Pos())
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

func (p *Parser) parsePrint() (Node, error) {
	p.nextToken()

	var f Print
	if p.curr.isIdent() {
		f.file = p.curr
		p.nextToken()
	}
	if p.curr.Type != lparen {
		return nil, fmt.Errorf("print: expected (, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		i, err := p.parseLine()
		if err != nil {
			return nil, err
		}
		f.lines = append(f.lines, i)
	}
	p.nextToken()
	return f, nil
}

func (p *Parser) parseLine() (Line, error) {
	var ns []Node
	for !p.isDone() {
		if p.curr.Type == Newline {
			break
		}
		var n Node
		if p.curr.Type == Ident {
			ref := Reference{id: p.curr}
			p.nextToken()
			if p.curr.Type != dot {
				return Line{}, fmt.Errorf("line: expected ., got %s (%s)", TokenString(p.curr), p.curr.Pos())
			}
			p.nextToken()
			if p.curr.Type != Ident {
				return Line{}, fmt.Errorf("line: expected ident, got %s (%s)", TokenString(p.curr), p.curr.Pos())
			}

			n = Attr{
				ref:  ref,
				attr: p.curr,
			}
		} else {
			n = p.curr
		}
		ns = append(ns, n)
		p.nextToken()
	}
	i := Line{nodes: ns}
	p.nextToken()
	return i, nil
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
		return nil, fmt.Errorf("break: expected [, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	c.expr = expr
	// p.nextToken()
	if p.curr.Type != Newline {
		return nil, fmt.Errorf("break: expected newline, got %s (%s)", TokenString(p.curr), p.curr.Pos())
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
		return nil, fmt.Errorf("break: expected [, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	b.expr = expr
	// p.nextToken()
	if p.curr.Type != Newline {
		return nil, fmt.Errorf("break: expected newline, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	return b, nil
}

func (p *Parser) parseStatements() ([]Node, error) {
	if p.curr.Type != lparen {
		return nil, fmt.Errorf("statement: expected (, got %s (%s)", p.curr, p.curr.Pos())
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
		switch p.curr.Type {
		case Keyword:
			parse, ok := p.stmts[p.curr.Literal]
			if !ok {
				fmt.Errorf("statement: unexpected keyword %s (%s)", p.curr, p.curr.Pos())
			}
			p.pushBlock(p.curr.Literal)
			node, err = parse()
			p.popBlock()
		case Ident, Text:
			node, err = p.parseField()
		default:
			err = fmt.Errorf("statement: unexpected token %s (%s)", p.curr, p.curr.Pos())
		}
		if err != nil {
			return nil, err
		}
		if node != nil {
			ns = append(ns, node)
		}
	}
	p.nextToken()
	return ns, nil
}

func (p *Parser) parseRepeat() (Node, error) {
	r := Repeat{pos: p.curr.Pos()}
	p.nextToken()
	if !(p.curr.isNumber() || p.curr.isIdent()) {
		return nil, fmt.Errorf("repeat: expected ident/number, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	r.repeat = p.curr
	p.nextToken()

	var err error
	switch p.curr.Type {
	case lparen:
		pos := p.curr.Pos()
		if ns, e := p.parseStatements(); e == nil {
			tok := Token{
				Literal: kwInline,
				Type:    Keyword,
				pos:     pos,
			}
			r.node = Block{id: tok, nodes: ns}
		} else {
			err = e
		}
	case Ident, Text:
		r.node = Reference{id: p.curr}
	default:
		err = fmt.Errorf("repeat: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	if err == nil {
		p.nextToken()
	}
	return r, err
}

func (p *Parser) parseSeek() (Node, error) {
	k := SeekStmt{pos: p.curr.Pos()}
	p.nextToken()
	if !p.curr.isNumber() {
		return nil, fmt.Errorf("seek: expected number, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	k.offset = p.curr
	p.nextToken()
	return k, nil
}

func (p *Parser) parseLet() (Node, error) {
	n := LetStmt{id: p.peek}
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	n.expr = expr
	return n, nil
}

func (p *Parser) parseDel() (Node, error) {
	d := DelStmt{pos: p.curr.Pos()}
	for !p.isDone() {
		p.nextToken()
		if p.curr.Type == Newline {
			break
		}
		if !p.curr.isIdent() {
			return nil, fmt.Errorf("del: expected ident, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		d.nodes = append(d.nodes, Reference{id: p.curr})
	}
	return d, nil
}

func (p *Parser) parseData() (Node, error) {
	b := emptyBlock(p.curr)
	p.nextToken()

	var files []Token
	for p.curr.Type != lparen {
		if !p.curr.isIdent() {
			return nil, fmt.Errorf("data: expected ident, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		files = append(files, p.curr)
		p.nextToken()
	}

	ns, err := p.parseStatements()
	if err != nil {
		return nil, err
	}
	b.nodes = append(b.nodes, ns...)
	// return b, nil
	return Data{Block: b, files: files}, nil
}

func (p *Parser) parsePredicate() (Expression, error) {
	p.nextToken()
	expr, err := p.parseExpression(bindLowest)
	if err == nil {
		p.nextToken()
	}
	return expr, err
}

func (p *Parser) parseExpression(pow int) (Expression, error) {
	expr, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}
	for !(p.peek.Type == rsquare || p.peek.Type == Newline) && pow < bindPower(p.peek) {
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
		return nil, fmt.Errorf("assign: unexpected token")
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
		return nil, fmt.Errorf("ternary: expected :, got %s (%s)", TokenString(p.curr), p.curr.Pos())
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
		p.nextToken()
		right, err := p.parseExpression(bindUnary)
		if err != nil {
			return nil, err
		}
		expr = Unary{
			Right:    right,
			operator: p.curr.Type,
		}
	case lparen:
		p.nextToken()
		n, err := p.parseExpression(bindLowest)
		if err != nil {
			return nil, err
		}
		if p.peek.Type != rparen {
			return nil, fmt.Errorf("expression: expected ), got %s (%s)", TokenString(p.peek), p.peek.Pos())
		} else {
			p.nextToken()
		}
		expr = n
	case Integer, Float, Bool:
		expr = Literal{id: p.curr}
	case Ident, Text:
		expr = Identifier{id: p.curr}
	default:
		return nil, fmt.Errorf("expression: unexpected token type %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	return expr, nil
}

func (p *Parser) parseInfix(left Expression) (Expression, error) {
	expr := Binary{
		Left:     left,
		operator: p.curr.Type,
	}
	pow := bindPower(p.curr)
	p.nextToken()

	right, err := p.parseExpression(pow)
	if err == nil {
		expr.Right = right
	}
	return expr, err
}

func (p *Parser) parseExit() (Node, error) {
	e := ExitStmt{pos: p.curr.Pos()}
	p.nextToken()
	if p.curr.Type != Integer {
		return nil, fmt.Errorf("exit: expected integer, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	e.code = p.curr
	if p.peek.Type != Newline {
		return nil, fmt.Errorf("exit: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	return e, nil
}

func (p *Parser) parseMatch() (Node, error) {
	m := Match{pos: p.curr.Pos()}

	p.nextToken()
	if !p.curr.isIdent() {
		return nil, fmt.Errorf("match: expected ident, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	m.id = p.curr

	p.nextToken()
	if p.curr.Type != Keyword && p.curr.Literal != kwWith {
		return nil, fmt.Errorf("match: expected with, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	if p.curr.Type != lparen {
		return nil, fmt.Errorf("match: expected (, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}

		var mcs []MatchCase
		for !p.isDone() {
			if p.curr.Type == Assign {
				break
			}
			if p.curr.Type != Integer {
				return nil, fmt.Errorf("match: expected integer, got %s (%s)", TokenString(p.curr), p.curr.Pos())
			}
			mcs = append(mcs, MatchCase{cond: p.curr})
			p.nextToken()
			if p.curr.Type == comma {
				p.nextToken()
			}
		}

		if p.curr.Type != Assign {
			return nil, fmt.Errorf("match: expected =, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		p.nextToken()

		var node Node
		switch p.curr.Type {
		case Ident, Text:
			node = Reference{id: p.curr}
			p.nextToken()
		case lparen:
			ns, err := p.parseStatements()
			if err != nil {
				return nil, err
			}
			tok := Token{
				Literal: kwInline,
				Type:    Keyword,
				pos:     m.Pos(),
			}
			node = Block{
				id:    tok,
				nodes: ns,
			}
		default:
			return nil, fmt.Errorf("match: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		for i := range mcs {
			mcs[i].node = node
		}
		m.nodes = append(m.nodes, mcs...)
	}
	p.nextToken()
	return m, nil
}

func (p *Parser) parseInclude() (Node, error) {
	i := Include{pos: p.curr.Pos()}

	p.nextToken()
	if p.curr.Type == lsquare {
		expr, err := p.parsePredicate()
		if err != nil {
			return nil, err
		}
		i.Predicate = expr
	}
	var err error
	switch p.curr.Type {
	case Ident, Text:
		i.node = Reference{id: p.curr}
	case lparen:
		if ns, e := p.parseStatements(); e == nil {
			tok := Token{
				Literal: kwInline,
				Type:    Keyword,
				pos:     i.Pos(),
			}
			i.node = Block{id: tok, nodes: ns}
		} else {
			err = e
		}
	default:
		err = fmt.Errorf("include: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
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
		return nil, fmt.Errorf("field: expected keyword, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	if p.curr.Literal != kwWith {
		return nil, fmt.Errorf("field: expected with, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	if !(p.curr.isIdent() || p.curr.isNumber()) {
		return nil, fmt.Errorf("field: expected ident/number, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	a.size, lenok = p.curr, true
	if !typok && !lenok {
		return nil, fmt.Errorf("field: type and length not set %s (%s)", TokenString(a.id), a.Pos())
	}
	p.nextToken()
	return a, nil
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
		case kwInt, kwUint, kwFloat, kwBytes, kwString:
			a.kind, typok = p.curr, true
			p.nextToken()
		default:
			return nil, fmt.Errorf("field: unexepected keyword %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
	}
	if p.curr.Type == Integer {
		a.size, lenok = p.curr, true
		p.nextToken()
	}
	if p.curr.Type == Keyword {
		if p.curr.Literal == kwBig || p.curr.Literal == kwLittle {
			a.endian = p.curr
		} else {
			return nil, fmt.Errorf("field: unexpected keyword %s (%s)", TokenString(p.curr), p.curr.Pos())
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
		return nil, fmt.Errorf("field: expected ident, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}

	id := p.curr
	p.nextToken()

	switch p.curr.Type {
	case Newline:
		node = Reference{id: id}
	case colon:
		node, err = p.parseFieldShort(id)
	case Keyword:
		if !p.inBlock(kwData) {
			err = fmt.Errorf("field: unexpected keyword %s (%s)", TokenString(p.curr), p.curr.Pos())
			break
		}
		node, err = p.parseFieldLong(id)
	default:
		err = fmt.Errorf("field: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	if err != nil {
		return
	}
	if n, ok := node.(Parameter); ok {
		if p.curr.Type == comma {
			p.nextToken()
			if !p.curr.isIdent() {
				return nil, fmt.Errorf("field: expected ident, got %s (%s)", TokenString(p.curr), p.curr.Pos())
			}
			n.apply = p.curr
			p.nextToken()
		}
		if p.curr.Type == Assign {
			p.nextToken()
			switch p.curr.Type {
			case Ident, Text, Integer, Float, Bool:
			default:
				return nil, fmt.Errorf("field: expected value, got %s (%s)", TokenString(p.curr), p.curr.Pos())
			}
		}
		node = n
	}
	if p.curr.Type != Newline {
		return nil, fmt.Errorf("field: expected newline, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	return
}

func (p *Parser) parseDeclare() (Node, error) {
	b := emptyBlock(p.curr)

	p.nextToken()
	if p.curr.Type != lparen {
		return nil, fmt.Errorf("declare: expected (, got %s (%s)", TokenString(p.curr), p.curr.Pos())
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
	p.nextToken()
	return b, nil
}

func (p *Parser) parseAssignment() (Node, error) {
	node := Constant{
		id:   p.curr,
		kind: kindInt,
	}

	p.nextToken()
	if p.curr.Type == colon {
		if id := p.currentBlock(); id == kwEnum || id == kwPoly || id == kwPoint {
			return nil, fmt.Errorf("assign: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		p.nextToken()
		if p.curr.Type != Keyword {
			return nil, fmt.Errorf("assign: expected keyword, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		switch lit := p.curr.Literal; lit {
		case kwInt:
		case kwUint:
			node.kind = kindUint
		case kwFloat:
			node.kind = kindFloat
		default:
			return nil, fmt.Errorf("assign: unexpected type %s (%s)", lit, p.curr.Pos())
		}
		p.nextToken()
	}
	if p.curr.Type != Assign {
		return nil, fmt.Errorf("assign: expected =, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	switch p.curr.Type {
	case Integer, Float, Text, Ident, Bool:
		node.value = p.curr
	default:
		return nil, fmt.Errorf("assign: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	switch p.curr.Type {
	case Comment:
		p.skipComment()
	case Newline:
		p.nextToken()
	default:
		return nil, fmt.Errorf("assign: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	return node, nil
}

func (p *Parser) parseDefine() (Node, error) {
	b := emptyBlock(p.curr)

	p.nextToken()
	if p.curr.Type != lparen {
		return nil, fmt.Errorf("defined: expected (, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if !p.curr.isIdent() {
			return nil, fmt.Errorf("define: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		n, err := p.parseAssignment()
		if err != nil {
			return nil, err
		}
		b.nodes = append(b.nodes, n.(Constant))
	}
	p.nextToken()
	return b, nil
}

func (p *Parser) parseImport() error {
	p.nextToken()
	if p.curr.Type != lparen {
		return fmt.Errorf("import: expected (, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()

	var files []string
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if !p.curr.isIdent() {
			return fmt.Errorf("import: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		files = append(files, p.curr.Literal)

		p.nextToken()
		switch p.curr.Type {
		case Comment:
			p.skipComment()
		case Newline:
			p.nextToken()
		default:
			return fmt.Errorf("import: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
	}
	p.nextToken()
	for _, f := range files {
		r, err := os.Open(f)
		if err != nil {
			return err
		}
		err = p.pushFrame(r)
		r.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) parseFile(file string) ([]Node, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	n, err := Parse(r)
	if err != nil {
		return nil, err
	}
	root, ok := n.(Block)
	if !ok {
		return nil, fmt.Errorf("%s: unexpected node type %T", filepath.Base(file), n)
	}
	return root.nodes, nil
}

func (p *Parser) parseBlock() (Node, error) {
	p.nextToken()
	if !p.curr.isIdent() {
		return nil, fmt.Errorf("block: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	b := emptyBlock(p.curr)

	p.nextToken()
	ns, err := p.parseStatements()
	if err != nil {
		return nil, err
	}
	b.nodes = ns
	return b, nil
}

func (p *Parser) parsePair() (Node, error) {
	a := Pair{kind: p.curr}
	p.nextToken()
	if !p.curr.isIdent() {
		return nil, fmt.Errorf("pair: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	a.id = p.curr
	p.nextToken()
	if p.curr.Type != lparen {
		return nil, fmt.Errorf("pair: expected (, got %s (%s)", TokenString(p.curr), p.curr.Pos())
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
	p.nextToken()
	return a, nil
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
		f := &frame{Scanner: s}
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

type frame struct {
	curr Token
	*Scanner
}

func (f *frame) Scan() Token {
	tok := f.curr
	f.curr = f.Scanner.Scan()
	return tok
}

func merge(dat, root Block) (Node, error) {
	var nodes []Node
	for _, n := range dat.nodes {
		switch n := n.(type) {
		case Parameter:
			nodes = append(nodes, n)
		case LetStmt:
			nodes = append(nodes, n)
		case DelStmt:
			nodes = append(nodes, n)
		case SeekStmt:
			nodes = append(nodes, n)
		case Reference:
			p, err := root.ResolveParameter(n.id.Literal)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, p)
		case Repeat:
			r, err := mergeRepeat(n, root)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, r)
		case Match:
			m, err := mergeMatch(n, root)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, m)
		case Include:
			b, err := mergeInclude(n, root)
			if err != nil {
				return nil, err
			}
			if x, ok := b.(Block); ok {
				nodes = append(nodes, x.nodes...)
			} else {
				nodes = append(nodes, b)
			}
		default:
			return nil, fmt.Errorf("unexpected node type %T", n)
		}
	}
	dat = Block{
		id:    dat.id,
		nodes: nodes,
	}
	return dat, nil
}

func mergeRepeat(r Repeat, root Block) (Node, error) {
	if r.repeat.isIdent() {
		return r, nil
	}
	var repeat int64
	switch r.repeat.Type {
	case Integer:
		repeat, _ = strconv.ParseInt(r.repeat.Literal, 0, 64)
	case Float:
		f, _ := strconv.ParseFloat(r.repeat.Literal, 64)
		repeat = int64(f)
	default:
		return nil, fmt.Errorf("unexpected token %s", TokenString(r.repeat))
	}
	var (
		dat Block
		err error
	)
	switch n := r.node.(type) {
	case Reference:
		dat, err = root.ResolveBlock(n.id.Literal)
	case Block:
		dat = n
	}
	if err != nil {
		return nil, err
	}
	if node, err := merge(dat, root); err != nil {
		return nil, err
	} else {
		d, ok := node.(Block)
		if !ok {
			return nil, fmt.Errorf("unexpected node type %s", node)
		}
		dat = d
	}
	tok := Token{
		Type:    Keyword,
		Literal: kwInline,
		pos:     r.Pos(),
	}
	pat := Block{id: tok}
	for i := int64(0); i < repeat; i++ {
		pat.nodes = append(pat.nodes, dat.nodes...)
	}
	return pat, nil
}

func mergeMatch(m Match, root Block) (Node, error) {
	var nodes []MatchCase
	for _, c := range m.nodes {
		var (
			err error
			dat Block
		)
		switch n := c.node.(type) {
		case Reference:
			dat, err = root.ResolveBlock(n.id.Literal)
		case Block:
			dat = n
		default:
			err = fmt.Errorf("unexpected node type %T", n)
		}
		if err != nil {
			return nil, err
		}
		c.node, err = merge(dat, root)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, c)
	}
	m.nodes = nodes
	return m, nil
}

func mergeInclude(i Include, root Block) (Node, error) {
	var (
		err error
		dat Block
	)
	switch n := i.node.(type) {
	case Reference:
		dat, err = root.ResolveBlock(n.id.Literal)
		if err != nil {
			return nil, err
		}
	case Block:
		dat = n
	}
	if i.node, err = merge(dat, root); err != nil {
		return nil, err
	}
	if i.Predicate == nil {
		return i.node, err
	} else {
		return i, err
	}
}

package dissect

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
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

	typedef map[string]typedef

	stmts  map[string]func() (Node, error)
	kwords map[string]func() (Node, error)
	blocks []string

	inline int
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
		kwTypdef:  p.parseTypedef,
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
		kwEcho:     p.parseEcho,
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
			return nil, fmt.Errorf("parse: unknown keyword: %s (%s)", p.curr.Literal, p.curr.Pos())
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

func (p *Parser) isClosed() error {
	if p.curr.Type != rparen {
		return fmt.Errorf("parse: expected ), got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	return nil
}

func (p *Parser) parseEcho() (Node, error) {
	e := Echo{
		pos:  p.curr.Pos(),
		file: Token{Literal: "-"},
	}
	p.nextToken()
	for !p.isDone() {
		if p.curr.Type == Newline || p.curr.Type == Greater {
			break
		}
		curr := p.curr
		var node Node
		if p.peek.Type == dot {
			p.nextToken()
			p.nextToken()

			node = Member{
				ref:  curr,
				attr: p.curr,
			}
		} else {
			node = curr
		}
		e.nodes = append(e.nodes, node)
		p.nextToken()
	}

	if p.curr.Type == Greater {
		p.nextToken()
		if !p.curr.isIdent() {
			return nil, fmt.Errorf("echo: expected ident, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		e.file = p.curr
		p.nextToken()
	}

	p.nextToken()
	return e, nil
}

func (p *Parser) parsePrint() (Node, error) {
	if !p.inBlock(kwData) {
		return nil, fmt.Errorf("print: unexpected outside of data bock (%s)", p.curr.Pos())
	}
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
			return nil, fmt.Errorf("print: unexpected method %s (%s)", TokenString(p.curr), p.curr.Pos())
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
		} else {
			err = fmt.Errorf("print: unexpected keyword %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
	case Newline:
	default:
		err = fmt.Errorf("print: unexpected keyword %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	return f, err
}

func (p *Parser) parsePrintTo(f *Print) error {
	if p.curr.Literal != kwTo {
		return fmt.Errorf("print: expected to, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	if !p.curr.isIdent() {
		return fmt.Errorf("print: expected ident/text, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	f.file = p.curr
	p.nextToken()
	switch p.curr.Type {
	case Keyword:
		if kw := p.curr.Literal; kw == kwAs {
			return p.parsePrintAs(f)
		} else if kw == kwWith {
			return p.parsePrintWith(f)
		} else {
			return fmt.Errorf("print: unexpected keyword %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
	case Newline:
	default:
		return fmt.Errorf("print: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	return nil
}

func (p *Parser) parsePrintAs(f *Print) error {
	if p.curr.Literal != kwAs {
		return fmt.Errorf("print: expected as, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	if p.curr.Type != Ident {
		return fmt.Errorf("print: expected ident, got %s (%s)", TokenString(p.curr), p.curr.Pos())
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
		return p.parsePrintWith(f)
	case Newline:
	default:
		return fmt.Errorf("print: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	return nil
}

func (p *Parser) parsePrintWith(f *Print) error {
	if p.curr.Literal != kwWith {
		return fmt.Errorf("print: expected with, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	for p.curr.Type != Newline {
		if p.curr.Type != Ident {
			return fmt.Errorf("print: expected ident, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		f.values = append(f.values, p.curr)
		p.nextToken()
	}
	return nil
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
				return nil, fmt.Errorf("statement: unexpected keyword %s (%s)", p.curr, p.curr.Pos())
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
			var id Token
			if p.curr.Type == Keyword {
				if p.curr.Literal != kwAs {
					return nil, fmt.Errorf("statement: expected as, got %s (%s)", p.curr, p.curr.Pos())
				}
				p.nextToken()
				id = p.curr
				p.nextToken()
			} else {
				id = Token{
					Literal: fmt.Sprintf("%s-%d", kwInline, p.inline),
					pos:     p.curr.Pos(),
				}
				p.inline++
			}
			b := Block{
				id:    id,
				nodes: xs,
			}
			ns = append(ns, b)
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
	return ns, p.isClosed()
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
				Literal: fmt.Sprintf("%s-%d", kwInline, p.inline),
				Type:    Keyword,
				pos:     pos,
			}
			p.inline++
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
	k := Seek{pos: p.curr.Pos()}
	p.nextToken()
	if p.curr.Type == Keyword {
		if p.curr.Literal != kwAt {
			return nil, fmt.Errorf("seek: expected at, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		k.absolute = true
		p.nextToken()
	}
	if p.curr.Type != lsquare {
		return nil, fmt.Errorf("seek: expected [, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	// if !p.curr.isNumber() {
	// 	return nil, fmt.Errorf("seek: expected number, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	// }
	k.offset = expr
	p.nextToken()
	return k, nil
}

func (p *Parser) parseLet() (Node, error) {
	n := Let{id: p.peek}
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
	checkPeek := func(peek Token) bool {
		return peek.Type == rsquare || peek.Type == Newline || peek.Type == Comment
	}
	for !checkPeek(p.peek) && pow < bindPower(p.peek) {
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
			return nil, fmt.Errorf("expression: expected ), got %s (%s)", TokenString(p.peek), p.peek.Pos())
		}
		p.nextToken()
		expr = n
	case Integer, Float, Bool, Text:
		expr = Literal{id: p.curr}
	case Ident, Internal:
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
	e := Exit{pos: p.curr.Pos()}
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
			if p.curr.Type == underscore {
				if p.peek.Type != Assign {
					return nil, fmt.Errorf("match: expected =, got %s (%s)", TokenString(p.curr), p.curr.Pos())
				}
				if pos := m.alt.Pos(); pos.IsValid() {
					return nil, fmt.Errorf("match: default branch already defined (%s)", p.curr.Pos())
				}
				m.alt = MatchCase{cond: p.curr}
				p.nextToken()
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
				Literal: fmt.Sprintf("%s-%d", kwInline, p.inline),
				Type:    Keyword,
				pos:     m.Pos(),
			}
			p.inline++
			node = Block{
				id:    tok,
				nodes: ns,
			}
		default:
			return nil, fmt.Errorf("match: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		if len(mcs) == 0 {
			m.alt.node = node
		} else {
			for i := range mcs {
				mcs[i].node = node
			}
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
				Literal: fmt.Sprintf("%s-%d", kwInline, p.inline),
				Type:    Keyword,
				pos:     i.Pos(),
			}
			p.inline++
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

func (p *Parser) parseTypedef() (Node, error) {
	p.nextToken()
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		var (
			td           typedef
			typok, lenok bool
		)
		if !p.curr.isIdent() {
			return nil, fmt.Errorf("typedef: expected ident, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		td.label = p.curr
		p.nextToken()
		if p.curr.Type != Assign {
			return nil, fmt.Errorf("typedef: expected =, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		p.nextToken()
		if p.curr.Type == Keyword {
			switch lit := p.curr.Literal; lit {
			case kwInt, kwUint, kwFloat, kwBytes, kwString:
				td.kind, typok = p.curr, true
				p.nextToken()
			default:
				return nil, fmt.Errorf("typedef: unexepected keyword %s (%s)", TokenString(p.curr), p.curr.Pos())
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
				return nil, fmt.Errorf("typdef: unexpected keyword %s (%s)", TokenString(p.curr), p.curr.Pos())
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
		case kwInt, kwUint, kwFloat, kwBytes, kwString:
			a.kind, typok = p.curr, true
			p.nextToken()
		default:
			return nil, fmt.Errorf("field: unexepected keyword %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
	} else if p.curr.Type == Ident {
		if td, ok := p.typedef[p.curr.Literal]; ok {
			a.kind = td.kind
			a.size = td.size
			a.endian = td.endian
		} else {
			return nil, fmt.Errorf("field: unexpected ident %s (%s)", TokenString(p.curr), p.curr.Pos())
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
			expr, err := p.parsePredicate()
			if err != nil {
				return nil, err
			}
			n.expect = expr
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
	return b, p.isClosed()
}

func (p *Parser) parseAssignment() (Node, error) {
	node := Constant{
		id: p.curr,
	}
	p.nextToken()
	if p.curr.Type != Assign {
		return nil, fmt.Errorf("assignment: expected =, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	expr, err := p.parsePredicate()
	if err != nil {
		return nil, err
	}
	switch e := expr.(type) {
	case Unary, Literal:
		node.value = expr
	default:
		return nil, fmt.Errorf("assignment: expected literal or prefix expression, got %T (%s)", e, node.Pos())
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
	return b, p.isClosed()
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
	return p.isClosed()
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
	return a, p.isClosed()
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

	file := "<input>"
	if n, ok := r.(interface{ Name() string }); ok {
		file = n.Name()
	}
	if err == nil {
		f := &frame{
			file:    file,
			Scanner: s,
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

type frame struct {
	file string
	curr Token
	*Scanner
}

func (f *frame) Scan() Token {
	tok := f.curr
	f.curr = f.Scanner.Scan()
	return tok
}

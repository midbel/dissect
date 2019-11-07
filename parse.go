package dissect

import (
	"fmt"
	"io"
	"strconv"
)

const (
	bindLowest int = iota
	bindOr
	bindAnd
	bindEq
	bindRel
	bindNot
)

var bindings = map[rune]int{
	Equal:   bindEq,
	NotEq:   bindEq,
	Lesser:  bindRel,
	LessEq:  bindRel,
	Greater: bindRel,
	GreatEq: bindRel,
	And:     bindAnd,
	Or:      bindOr,
	Not:     bindNot,
}

func bindPower(tok Token) int {
	pw := bindLowest
	if p, ok := bindings[tok.Type]; ok {
		pw = p
	}
	return pw
}

type Parser struct {
	scan *Scanner

	curr Token
	peek Token

	kwords map[string]func() (Node, error)
}

func Parse(r io.Reader) (Node, error) {
	scan, err := Scan(r)
	if err != nil {
		return nil, err
	}
	p := Parser{scan: scan}

	p.kwords = map[string]func() (Node, error){
		kwData:    p.parseData,
		kwBlock:   p.parseBlock,
		kwEnum:    p.parsePair,
		kwPoint:   p.parsePair,
		kwPoly:    p.parsePair,
		kwDeclare: p.parseDeclare,
		kwDefine:  p.parseDefine,
	}

	p.nextToken()
	p.nextToken()

	return p.Parse()
}

func Merge(r io.Reader) (Node, error) {
	n, err := Parse(r)
	if err != nil {
		return nil, err
	}
	root := n.(Block)
	dat, err := root.ResolveData()
	if err != nil {
		return nil, err
	}
	return merge(dat, root)
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
		n, err := parse()
		if err != nil {
			return nil, err
		}
		if n != nil {
			root.nodes = append(root.nodes, n)
		}
	}
	return root, nil
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
			if lit := p.curr.Literal; lit == kwInclude {
				node, err = p.parseInclude()
			} else if lit == kwLet {
				node, err = p.parseLet()
			} else if lit == kwDel {
				node, err = p.parseDel()
			} else if lit == kwSeek {
				node, err = p.parseSeek()
			} else if lit == kwRepeat {
				node, err = p.parseRepeat()
			} else if lit == kwExit {
				node, err = p.parseExit()
			} else if lit == kwMatch {
				node, err = p.parseMatch()
			} else {
				err = fmt.Errorf("statement: unexpected keyword %s (%s)", p.curr, p.curr.Pos())
			}
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
	p.nextToken()
	n := LetStmt{id: p.curr}
	p.nextToken()
	if p.curr.Type != Assign {
		return nil, fmt.Errorf("let: expect =, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	for !p.isDone() {
		if p.curr.Type == Newline {
			break
		}
		p.nextToken()
	}
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

	ns, err := p.parseStatements()
	if err != nil {
		return nil, err
	}
	b.nodes = append(b.nodes, ns...)
	return b, nil
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
	for p.peek.Type != rsquare && pow < bindPower(p.peek) {
		p.nextToken()
		n, err := p.parseInfix(expr)
		if err != nil {
			return nil, err
		}
		expr = n
	}
	if p.peek.Type == rsquare {
		p.nextToken()
	}
	return expr, nil
}

func (p *Parser) parsePrefix() (Expression, error) {
	var expr Expression
	switch p.curr.Type {
	case Not:
		p.nextToken()
		right, err := p.parseExpression(bindNot)
		if err != nil {
			return nil, err
		}
		expr = Unary{Right: right}
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
		if p.curr.Type != Integer {
			return nil, fmt.Errorf("match: expected integer, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		mc := MatchCase{cond: p.curr}

		p.nextToken()
		if p.curr.Type != Assign {
			return nil, fmt.Errorf("match: expected =, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		p.nextToken()

		switch p.curr.Type {
		case Ident, Text:
			mc.node = Reference{id: p.curr}
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
			mc.node = Block{
				id:    tok,
				nodes: ns,
			}
		default:
			return nil, fmt.Errorf("match: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		m.nodes = append(m.nodes, mc)
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
		a := Parameter{id: id}
		p.nextToken()
		if p.curr.Type == Keyword {
			switch lit := p.curr.Literal; lit {
			case kwInt, kwUint, kwFloat, kwBytes, kwString:
			default:
				return nil, fmt.Errorf("field: unexepected keyword %s (%s)", TokenString(p.curr), p.curr.Pos())
			}
			a.kind = p.curr
			p.nextToken()
		}
		if p.curr.Type == Integer {
			a.size = p.curr
			p.nextToken()
		}
		if p.curr.Type == comma {
			p.nextToken()
			if !p.curr.isIdent() {
				return nil, fmt.Errorf("field: expected ident, got %s (%s)", TokenString(p.curr), p.curr.Pos())
			}
			a.apply = p.curr
			p.nextToken()
		}
		if p.curr.Type != Newline {
			return nil, fmt.Errorf("field: expected newline, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		node = a
	default:
		err = fmt.Errorf("field: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	return
}

func (p *Parser) parseProperties() (map[string]Token, error) {
	props := make(map[string]Token)
	for p.curr.Type != rsquare {
		p.nextToken()
		if !p.curr.isIdent() {
			return nil, fmt.Errorf("properties: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		key := p.curr
		p.nextToken()
		if p.curr.Type != Assign {
			return nil, fmt.Errorf("properties: expected =, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		p.nextToken()
		switch p.curr.Type {
		case Ident, Text, Integer, Bool:
			props[key.Literal] = p.curr
		default:
			return nil, fmt.Errorf("properties: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		p.nextToken()
		switch p.curr.Type {
		case rsquare, comma:
		default:
			return nil, fmt.Errorf("properties: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
	}
	p.nextToken()
	return props, nil
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

func (p *Parser) parseAssignment(withkind bool) (Node, error) {
	node := Constant{
		id:   p.curr,
		kind: kindInt,
	}

	p.nextToken()
	if p.curr.Type == colon {
		if !withkind {
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
		n, err := p.parseAssignment(true)
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
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if !p.curr.isIdent() {
			return fmt.Errorf("import: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
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
	return nil
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
		n, err := p.parseAssignment(false)
		if err != nil {
			return nil, err
		}
		a.nodes = append(a.nodes, n.(Constant))
	}
	p.nextToken()
	return a, nil
}

func (p *Parser) isDone() bool {
	return p.curr.Type == EOF
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

func (p *Parser) nextToken() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
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
		node Node
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
	if node, err = merge(dat, root); err != nil {
		return nil, err
	}
	tok := Token{
		Type: Keyword,
		Literal: kwInline,
		pos: r.Pos(),
	}
	pat := Block{id: tok}
	for i := int64(0); i < repeat; i++ {
		n := node
		pat.nodes = append(pat.nodes, n)
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

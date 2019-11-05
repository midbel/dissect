package dissect

import (
	"fmt"
	"io"
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
			return nil, fmt.Errorf("parse: unexpected token: %s", p.curr)
		}
		parse, ok := p.kwords[p.curr.Literal]
		if !ok {
			return nil, fmt.Errorf("parse: unknown keyword: %s", p.curr.Literal)
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
		return nil, fmt.Errorf("parseStatements: expected (, got %s", p.curr)
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
			} else {
				err = fmt.Errorf("parseStatements: unexpected keyword %s", p.curr)
			}
		case Ident, Text:
			node, err = p.parseField()
		default:
			err = fmt.Errorf("parseStatements: unexpected token %s", p.curr)
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
		return nil, fmt.Errorf("parseRepeat: unexpected token %s", TokenString(p.curr))
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
		err = fmt.Errorf("parseRepeat: unexpected token %s", TokenString(p.curr))
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
		return nil, fmt.Errorf("parseSeek: expected number, got %s", TokenString(p.curr))
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
		return nil, fmt.Errorf("parseLet: expect =, got %s", TokenString(p.curr))
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
			return nil, fmt.Errorf("parseDel: expected ident, got %s", TokenString(p.curr))
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
			return nil, fmt.Errorf("parseExpression: expected ), got %s", p.peek)
		} else {
			p.nextToken()
		}
		expr = n
	case Integer, Float, Bool:
		expr = Literal{id: p.curr}
	case Ident, Text:
		expr = Identifier{id: p.curr}
	default:
		return nil, fmt.Errorf("parseExpression: unexpected token type %s", TokenString(p.curr))
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
		err = fmt.Errorf("parseInclude: unexpected token %s", p.curr)
	}
	if err == nil {
		p.nextToken()
	}
	return i, err
}

func (p *Parser) parseField() (node Node, err error) {
	if !p.curr.isIdent() {
		return nil, fmt.Errorf("parseField: unexpected token %s", p.curr)
	}

	id := p.curr
	p.nextToken()

	switch p.curr.Type {
	case Newline:
		node = Reference{id: id}
	case lsquare:
		if p.peek.Type == rsquare {
			node = Reference{id: id}

			p.nextToken()
			p.nextToken()
		} else {
			a := Parameter{id: id}
			a.props, err = p.parseProperties()
			if err == nil {
				node = a
			}
		}
	default:
		err = fmt.Errorf("parseField: unexpected token %s", p.curr)
	}
	return
}

func (p *Parser) parseProperties() (map[string]Token, error) {
	props := make(map[string]Token)
	for p.curr.Type != rsquare {
		p.nextToken()
		if !p.curr.isIdent() {
			return nil, fmt.Errorf("parseProperties: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		key := p.curr
		p.nextToken()
		if p.curr.Type != Assign {
			return nil, fmt.Errorf("parseProperties: expected =, got %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		p.nextToken()
		switch p.curr.Type {
		case Ident, Text, Integer, Bool:
			props[key.Literal] = p.curr
		default:
			return nil, fmt.Errorf("parseProperties: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
		p.nextToken()
		switch p.curr.Type {
		case rsquare, comma:
		default:
			return nil, fmt.Errorf("parseProperties: unexpected token %s (%s)", TokenString(p.curr), p.curr.Pos())
		}
	}
	p.nextToken()
	return props, nil
}

func (p *Parser) parseDeclare() (Node, error) {
	b := emptyBlock(p.curr)

	p.nextToken()
	if p.curr.Type != lparen {
		return nil, fmt.Errorf("parseDeclare: expected (, got %s", p.curr)
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if p.peek.Type != lsquare {
			return nil, fmt.Errorf("parseDeclare: expected [, got %s", p.curr)
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
	node := Constant{id: p.curr}

	p.nextToken()
	if p.curr.Type != Assign {
		return nil, fmt.Errorf("parseAssignment: expected =, got %s", p.curr)
	}
	p.nextToken()
	switch p.curr.Type {
	case Integer, Float, Text, Ident, Bool:
		node.value = p.curr
	default:
		return nil, fmt.Errorf("parseAssignment: unexpected token %s", p.curr)
	}
	p.nextToken()
	switch p.curr.Type {
	case Comment:
		p.skipComment()
	case Newline:
		p.nextToken()
	default:
		return nil, fmt.Errorf("parseAssignment: unexpected token %s", p.curr)
	}
	return node, nil
}

func (p *Parser) parseDefine() (Node, error) {
	b := emptyBlock(p.curr)

	p.nextToken()
	if p.curr.Type != lparen {
		return nil, fmt.Errorf("parseDefine: expected (, got %s", p.curr)
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if !p.curr.isIdent() {
			return nil, fmt.Errorf("parseDefine: unexpected token %s", p.curr)
		}
		n, err := p.parseAssignment()
		if err != nil {
			return nil, err
		}
		b.nodes = append(b.nodes, n)
	}
	p.nextToken()
	return b, nil
}

func (p *Parser) parseImport() error {
	p.nextToken()
	if p.curr.Type != lparen {
		return fmt.Errorf("parseImport: expected (, got %s", p.curr)
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if !p.curr.isIdent() {
			return fmt.Errorf("parseImport: unexpected token %s", p.curr)
		}
		p.nextToken()
		switch p.curr.Type {
		case Comment:
			p.skipComment()
		case Newline:
			p.nextToken()
		default:
			return fmt.Errorf("parseImport: unexpected token %s", p.curr)
		}
	}
	p.nextToken()
	return nil
}

func (p *Parser) parseBlock() (Node, error) {
	p.nextToken()
	if !p.curr.isIdent() {
		return nil, fmt.Errorf("parseBlock: unexpected token %s", p.curr)
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
		return nil, fmt.Errorf("parsePair: unexpected token %s", p.curr)
	}
	a.id = p.curr
	p.nextToken()
	if p.curr.Type != lparen {
		return nil, fmt.Errorf("parsePair: expected (, got %s", p.curr)
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
		a.nodes = append(a.nodes, n)
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

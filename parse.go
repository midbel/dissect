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

func (p *Parser) parseStatements(flat bool) ([]Node, error) {
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
			if lit := p.curr.Literal; lit == kwInclude && !flat {
				node, err = p.parseInclude()
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

func (p *Parser) parseData() (Node, error) {
	b := emptyBlock(p.curr)
	p.nextToken()

	ns, err := p.parseStatements(false)
	if err != nil {
		return nil, err
	}
	b.nodes = append(b.nodes, ns...)
	return b, nil
}

func (p *Parser) parseExpression() error {
	fmt.Println("parseExpression:", p.curr)
	p.nextToken()
	for !p.isDone() {
		fmt.Println("parseExpression", p.curr)
		if p.curr.Type == rsquare {
			break
		}
		p.nextToken()
	}
	p.nextToken()
	return nil
}

func (p *Parser) parseInclude() (Node, error) {
	i := Include{pos: p.curr.Pos()}

	p.nextToken()
	if p.curr.Type == lsquare {
		if err := p.parseExpression(); err != nil {
			return nil, err
		}
		// i.Predicate = expr
	}
	var err error
	switch p.curr.Type {
	case Ident, Text:
		r := Reference{id: p.curr}
		i.nodes = append(i.nodes, r)
	case lparen:
		if ns, e := p.parseStatements(false); e == nil {
			i.nodes = append(i.nodes, ns...)
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
			node = Parameter{id: id}
			err = p.parseProperties()
		}
	default:
		err = fmt.Errorf("parseField: unexpected token %s", p.curr)
	}
	return
}

func (p *Parser) parseProperties() error {
	for p.curr.Type != rsquare {
		p.nextToken()
		if !p.curr.isIdent() {
			return fmt.Errorf("parseProperties: unexpected token %s", p.curr)
		}
		p.nextToken()
		if p.curr.Type != equal {
			return fmt.Errorf("parseProperties: expected =, got %s", p.curr)
		}
		p.nextToken()
		switch p.curr.Type {
		case Ident:
		case Text:
		case Integer:
		case Bool:
		default:
			return fmt.Errorf("parseProperties: unexpected token %s", p.curr)
		}
		p.nextToken()
		switch p.curr.Type {
		case rsquare, comma:
		default:
			return fmt.Errorf("parseProperties: unexpected token %s", p.curr)
		}
	}
	p.nextToken()
	return nil
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
	if p.curr.Type != equal {
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
	if p.curr.Type != lparen {
		return nil, fmt.Errorf("parseBlock: expected (, got %s", p.curr)
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if !p.curr.isIdent() {
			return nil, fmt.Errorf("parseBlock: unexpected token %s", p.curr)
		}
		switch p.peek.Type {
		case lsquare:
			n, err := p.parseField()
			if err != nil {
				return nil, err
			}
			b.nodes = append(b.nodes, n)
		case Newline:
			p.nextToken()
		default:
			return nil, fmt.Errorf("parseBlock: unexpected token %s", p.peek)
		}
	}
	p.nextToken()
	return nil, nil
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

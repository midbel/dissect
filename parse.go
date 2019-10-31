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

	kwords map[string]func() error
}

func Parse(r io.Reader) error {
	scan, err := Scan(r)
	if err != nil {
		return err
	}
	p := Parser{scan: scan}

	p.kwords = map[string]func() error{
		"data":       p.parseData,
		"block":      p.parseBlock,
		"enum":       p.parsePair,
		"pointpair":  p.parsePair,
		"polynomial": p.parsePair,
		"declare":    p.parseDeclare,
		"define":     p.parseDefine,
	}

	p.nextToken()
	p.nextToken()

	return p.Parse()
}

func (p *Parser) Parse() error {
	p.skipComment()
	if p.curr.Type == Keyword && p.curr.Literal == "import" {
		if err := p.parseImport(); err != nil {
			return err
		}
	}

	for {
		p.skipComment()
		if p.isDone() {
			break
		}
		if p.curr.Type != Keyword {
			return fmt.Errorf("parse: unexpected token: %s", p.curr)
		}
		parse, ok := p.kwords[p.curr.Literal]
		if !ok {
			return fmt.Errorf("parse: unknown keyword: %s", p.curr.Literal)
		}
		if err := parse(); err != nil {
			return err
		}
	}
	return nil
}

func (p *Parser) parseStatements() error {
	if p.curr.Type != lparen {
		return fmt.Errorf("parseStatements: expected (, got %s", p.curr)
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		var err error
		switch p.curr.Type {
		case Keyword:
			if lit := p.curr.Literal; lit == "include" {
				err = p.parseInclude()
			} else {
				err = fmt.Errorf("parseStatements: unexpected keyword %s", p.curr)
			}
		case Ident, Text:
			peek := p.peek.Type
			if peek == lsquare {
				err = p.parseField()
			} else if peek == Newline {
				p.nextToken()
			} else {
				err = fmt.Errorf("parseStatements: unexpected token %s", p.curr)
			}
		default:
			err = fmt.Errorf("parseStatements: unexpected token %s", p.curr)
		}
		if err != nil {
			return err
		}
	}
	p.nextToken()
	return nil
}

func (p *Parser) parseData() error {
	fmt.Println("parseData", p.curr)
	p.nextToken()

	return p.parseStatements()
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

func (p *Parser) parseInclude() error {
	fmt.Println("parseInclude:", p.curr)
	p.nextToken()
	fmt.Println(">> include", p.curr)
	if p.curr.Type == lsquare {
		if err := p.parseExpression(); err != nil {
			return err
		}
	}
	var err error
	switch p.curr.Type {
	case Ident, Text:

	case lparen:
		err = p.parseStatements()
	default:
		err = fmt.Errorf("parseInclude: unexpected token %s", p.curr)
	}
	if err == nil {
		p.nextToken()
	}
	return err
}

func (p *Parser) parseField() error {
	if !p.curr.isIdent() {
		return fmt.Errorf("parseField: unexpected token %s", p.curr)
	}

	p.nextToken()
	if p.curr.Type != lsquare {
		return nil
	}
	if p.peek.Type == rsquare {
		p.nextToken()
		p.nextToken()
		return nil
	}
	return p.parseProperties()
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

func (p *Parser) parseDeclare() error {
	fmt.Println("parseDeclare:", p.curr)
	p.nextToken()
	if p.curr.Type != lparen {
		return fmt.Errorf("parseDeclare: expected (, got %s", p.curr)
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if p.peek.Type != lsquare {
			return fmt.Errorf("parseDeclare: expected [, got %s", p.curr)
		}
		if err := p.parseField(); err != nil {
			return err
		}
	}
	p.nextToken()
	return nil
}

func (p *Parser) parseAssignment() (key, val Token, err error) {
	key = p.curr
	p.nextToken()
	if p.curr.Type != equal {
		err = fmt.Errorf("parseAssignment: expected =, got %s", p.curr)
		return
	}
	p.nextToken()
	switch p.curr.Type {
	case Integer, Float, Text, Ident:
		val = p.curr
	default:
		err = fmt.Errorf("parseAssignment: unexpected token %s", p.curr)
		return
	}
	p.nextToken()
	switch p.curr.Type {
	case Comment:
		p.skipComment()
	case Newline:
		p.nextToken()
	default:
		err = fmt.Errorf("parseAssignment: unexpected token %s", p.curr)
	}
	return
}

func (p *Parser) parseDefine() error {
	fmt.Println("parseDefine:", p.curr)
	p.nextToken()
	if p.curr.Type != lparen {
		return fmt.Errorf("parseDefine: expected (, got %s", p.curr)
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if !p.curr.isIdent() {
			return fmt.Errorf("parseDefine: unexpected token %s", p.curr)
		}
		_, _, err := p.parseAssignment()
		if err != nil {
			return err
		}
	}
	p.nextToken()
	return nil
}

func (p *Parser) parseImport() error {
	fmt.Println("parseImport:", p.curr)
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

func (p *Parser) parseBlock() error {
	fmt.Println("parseBlock:", p.curr)
	p.nextToken()
	if !p.curr.isIdent() {
		return fmt.Errorf("parseBlock: unexpected token %s", p.curr)
	}
	p.nextToken()
	if p.curr.Type != lparen {
		return fmt.Errorf("parseBlock: expected (, got %s", p.curr)
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if !p.curr.isIdent() {
			return fmt.Errorf("parseBlock: unexpected token %s", p.curr)
		}
		switch p.peek.Type {
		case lsquare:
			if err := p.parseField(); err != nil {
				return err
			}
		case Newline:
			p.nextToken()
		default:
			return fmt.Errorf("parseBlock: unexpected token %s", p.peek)
		}
	}
	p.nextToken()
	return nil
}

func (p *Parser) parsePair() error {
	fmt.Println("parsePair:", p.curr)
	p.nextToken()
	if !p.curr.isIdent() {
		return fmt.Errorf("parsePair: unexpected token %s", p.curr)
	}
	p.nextToken()
	if p.curr.Type != lparen {
		return fmt.Errorf("parsePair: expected (, got %s", p.curr)
	}
	p.nextToken()
	for !p.isDone() {
		p.skipComment()
		if p.curr.Type == rparen {
			break
		}
		if _, _, err := p.parseAssignment(); err != nil {
			return err
		}
	}
	p.nextToken()
	return nil
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

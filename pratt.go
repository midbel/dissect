package dissect

import (
	"fmt"
	"strings"
)

type pratt struct {
	scan *Scanner

	prefix map[rune]func() (Expression, error)
	infix  map[rune]func(Expression) (Expression, error)

	peek Token
	curr Token
}

func parseString(str string) (Expression, error) {
	s, err := Scan(strings.NewReader(str))
	if err != nil {
		return nil, err
	}
	p := pratt{scan: s}
	p.prefix = map[rune]func() (Expression, error){}
	p.infix = map[rune]func(Expression) (Expression, error){}

	p.nextToken()
	p.nextToken()

	e, err := p.parseExpression(bindLowest)
	return e, err
}

func (p *pratt) parseExpression(pow int) (Expression, error) {
	expr, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}
	checkPeek := func(peek Token) bool {
		return peek.Type == rsquare || peek.Type == Newline || peek.Type == Comment || peek.Type == EOF
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

func (p *pratt) parsePrefix() (Expression, error) {
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
			return nil, fmt.Errorf("pratt: expected ), got %s (%s)", TokenString(p.peek), p.peek.Pos())
		}
		p.nextToken()
		expr = n
	case Integer, Float, Bool, Text:
		expr = Literal{id: p.curr}
	case Ident, Internal:
		expr = Identifier{id: p.curr}
	default:
		return nil, fmt.Errorf("pratt: unexpected token type %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	return expr, nil
}

func (p *pratt) parseInfix(left Expression) (Expression, error) {
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

func (p *pratt) parseAssign(left Expression) (Expression, error) {
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
		return nil, fmt.Errorf("pratt: unexpected token in assignment")
	}
}

func (p *pratt) parseTernary(left Expression) (Expression, error) {
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
		return nil, fmt.Errorf("pratt: expected :, got %s (%s)", TokenString(p.curr), p.curr.Pos())
	}
	p.nextToken()
	alt, err := p.parseExpression(bindLowest)
	if err != nil {
		return nil, err
	}

	t.csq, t.alt = csq, alt
	return t, nil
}

func (p *pratt) nextToken() {
	p.curr = p.peek
	p.peek = p.scan.Scan()
}

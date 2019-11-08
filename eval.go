package dissect

import (
	"fmt"
	"strconv"
)

func eval(e Expression, root *state) (Value, error) {
	var (
		v   Value
		err error
	)

	switch e := e.(type) {
	case Ternary:
		v, err = evalTernary(e, root)
	case Binary:
		v, err = evalBinary(e, root)
	case Unary:
		v, err = evalUnary(e, root)
	case Literal:
		v, err = evalLiteral(e, root)
	case Identifier:
		v, err = evalIdentifier(e, root)
	default:
		err = fmt.Errorf("unsupported expression type %T", e)
	}
	return v, err
}

func evalTernary(t Ternary, root *state) (Value, error) {
	v, err := eval(t.cond, root)
	if err != nil {
		return nil, err
	}
	if isTrue(v) {
		return eval(t.csq, root)
	} else {
		return eval(t.alt, root)
	}
}

func evalBinary(b Binary, root *state) (Value, error) {
	switch b.operator {
	case Equal, NotEq, Lesser, LessEq, Greater, GreatEq:
		return evalRelational(b, root)
	case And, Or:
		return evalLogical(b, root)
	case Add, Mul, Div, Min:
		return evalArithmetic(b, root)
	default:
		return evalBitwise(b, root)
	}
}

func evalUnary(u Unary, root *state) (Value, error) {
	var val Value
	switch u.operator {
	case Not:
		v, err := eval(u.Right, root)
		if err != nil {
			return nil, err
		}
		val = anonymousBool(asBool(v))
	case Min:
		val, err := eval(u.Right, root)
		if err != nil {
			return nil, err
		}
		val, err = val.reverse()
	default:
		return nil, fmt.Errorf("unsupported unary operator")
	}
	return val, nil
}

func evalLiteral(i Literal, _ *state) (Value, error) {
	var (
		val  Value
		meta = Meta{Id: kwAno}
	)
	switch i.id.Type {
	case Integer:
		i, err := strconv.ParseInt(i.id.Literal, 0, 64)
		if err != nil {
			return nil, err
		}
		val = &Int{
			Meta: meta,
			Raw:  i,
		}
	case Float:
		i, err := strconv.ParseFloat(i.id.Literal, 64)
		if err != nil {
			return nil, err
		}
		val = &Real{
			Meta: meta,
			Raw:  i,
		}
	case Bool:
		i, err := strconv.ParseBool(i.id.Literal)
		if err != nil {
			return nil, err
		}
		val = &Boolean{
			Meta: meta,
			Raw:  i,
		}
	case Text:
		val = &String{
			Meta: meta,
			Raw:  i.id.Literal,
		}
	default:
		return nil, fmt.Errorf("unsupported token type %s", TokenString(i.id))
	}
	return val, nil
}

func evalIdentifier(i Identifier, root *state) (Value, error) {
	return root.ResolveValue(i.id.Literal)
}

func evalArithmetic(b Binary, root *state) (Value, error) {
	left, err := eval(b.Left, root)
	if err != nil {
		return nil, err
	}
	right, err := eval(b.Right, root)
	if err != nil {
		return nil, err
	}
	var val Value
	switch b.operator {
	case Add:
		val, err = left.add(right)
	case Min:
		val, err = left.subtract(right)
	case Mul:
		val, err = left.multiply(right)
	case Div:
		val, err = left.divide(right)
	case Modulo:
		val, err = left.modulo(right)
	default:
		err = fmt.Errorf("unsupported arithmetic operator")
	}
	return val, err
}

func evalLogical(b Binary, root *state) (Value, error) {
	left, err := eval(b.Left, root)
	if err != nil {
		return nil, err
	}
	right, err := eval(b.Right, root)
	if err != nil {
		return nil, err
	}
	var ok bool
	switch b.operator {
	case And:
		ok = asBool(left) && asBool(right)
	case Or:
		ok = asBool(left) || asBool(right)
	default:
		return nil, fmt.Errorf("unsupported logical operator")
	}
	return anonymousBool(ok), nil
}

func evalRelational(b Binary, root *state) (Value, error) {
	left, err := eval(b.Left, root)
	if err != nil {
		return nil, err
	}
	right, err := eval(b.Right, root)
	if err != nil {
		return nil, err
	}

	var (
		cmp = left.Cmp(right)
		ok  bool
	)
	switch b.operator {
	case Equal:
		ok = cmp == 0
	case NotEq:
		ok = cmp != 0
	case Lesser:
		ok = cmp < 0
	case LessEq:
		ok = cmp == 0 || cmp < 0
	case Greater:
		ok = cmp > 0
	case GreatEq:
		ok = cmp == 0 || cmp > 0
	default:
		return nil, fmt.Errorf("unsupported relational operator")
	}
	return anonymousBool(ok), nil
}

func evalBitwise(b Binary, root *state) (Value, error) {
	return nil, nil
}

func anonymousBool(ok bool) Value {
	v := Boolean{
		Meta: Meta{Id: kwAno},
		Raw:  ok,
	}
	return &v
}

func isTrue(v Value) bool {
	return asBool(v)
}

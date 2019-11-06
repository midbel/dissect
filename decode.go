package dissect

import (
	"errors"
	"fmt"
	"io"
	"math"
	"strconv"
)

var (
	ErrSkip = errors.New("skip block")
	ErrDone = errors.New("done")
)

const numbit = 8

// type Value struct {
// 	Name  string
// 	Pos   int
// 	Raw   interface{}
// 	Value interface{}
// }

type Decoder struct {
	root Block
	data Block
}

func NewDecoder(r io.Reader) (*Decoder, error) {
	n, err := Parse(r)
	// n, err := Merge(r)
	if err != nil {
		return nil, err
	}
	root, ok := n.(Block)
	if !ok {
		return nil, fmt.Errorf("root node is not a block")
	}
	data, err := root.ResolveData()
	if err != nil {
		return nil, err
	}
	d := Decoder{
		root: root,
		data: data,
	}
	return &d, nil
}

func (d Decoder) Decode(buf []byte) ([]Value, error) {
	s := state{
		Block: d.root,
		Size:  len(buf),
	}
	s.buffer = append(s.buffer, buf...)
	err := s.decodeBlock(d.data)
	if err != nil && !errors.Is(err, ErrDone) {
		return nil, err
	}
	return s.Values, nil
}

type state struct {
	Block
	Values []Value

	buffer []byte
	Pos    int
	Size   int
}

func (s *state) ResolveValue(n string) (Value, error) {
	for i := len(s.Values) - 1; i >= 0; i-- {
		v := s.Values[i]
		if v.String() == n {
			return v, nil
		}
	}
	return nil, fmt.Errorf("%s: value not defined", n)
}

func (s *state) DeleteValue(n string) {
	for i := 0; ; i++ {
		if i >= len(s.Values) {
			break
		}
		if v := s.Values[i]; v.String() == n {
			s.Values = append(s.Values[:i], s.Values[i+1:]...)
		}
	}
}

func (root *state) decodeBlock(data Block) error {
	for _, n := range data.nodes {
		switch n := n.(type) {
		case LetStmt:
			// ignore for now
		case DelStmt:
			for _, n := range n.nodes {
				r, ok := n.(Reference)
				if !ok {
					continue
				}
				root.DeleteValue(r.id.Literal)
			}
		case SeekStmt:
			seek, err := strconv.Atoi(n.offset.Literal)
			if err != nil {
				return fmt.Errorf("invalid seek value given")
			}
			root.Pos += seek
			if root.Pos < 0 || root.Pos >= root.Size {
				return fmt.Errorf("seek outside of buffer range")
			}
		case Repeat:
			// ignore for now
			if err := root.decodeRepeat(n); err != nil {
				return err
			}
		case Reference:
			p, err := root.ResolveParameter(n.id.Literal)
			if err != nil {
				return err
			}
			val, err := root.decodeParameter(p)
			if err != nil {
				return err
			}
			if val != nil {
				root.Values = append(root.Values, val)
			}
		case Parameter:
			val, err := root.decodeParameter(n)
			if err != nil {
				return err
			}
			if val != nil {
				root.Values = append(root.Values, val)
			}
		case Include:
			err := root.decodeInclude(n)
			if err != nil && !errors.Is(err, ErrSkip) {
				return err
			}
		default:
			return fmt.Errorf("unexpected node type %T", n)
		}
	}
	return nil
}

func (root *state) decodeParameter(p Parameter) (Value, error) {
	var (
		bits   = p.numbit()
		offset = root.Pos % numbit
		index  = root.Pos / numbit
	)
	if index >= root.Size {
		return nil, ErrDone
	}

	var (
		need  = numbytes(bits)
		shift = (numbit * need) - (offset + bits)
		mask  = 1
		raw   Value
	)
	if bits > 1 {
		mask = (1 << bits) - 1
	}
	if n := root.Size; n < need {
		return nil, fmt.Errorf("buffer too short (missing %d bytes)", need-n)
	}
	meta := Meta{
		Id:  p.id.Literal,
		Pos: root.Pos,
	}
	dat := btoi(root.buffer[index:index+need], shift, mask)
	switch p.is() {
	case kindInt: // signed integer
		raw = &Int{
			Meta: meta,
			Raw:  int64(dat),
		}
	case kindUint: // unsigned integer
		raw = &Uint{
			Meta: meta,
			Raw:  dat,
		}
	case kindFloat: // float
		// raw = math.Float64frombits(dat)
		raw = &Real{
			Meta: meta,
			Raw:  math.Float64frombits(dat),
		}
	default:
		return nil, fmt.Errorf("unsupported type: %c", p.is())
	}
	if err := evalApply(raw, p.apply, root); err != nil {
		return raw, err
	}
	root.Pos += bits
	return raw, nil
}

func (root *state) decodeRepeat(n Repeat) error {
	var (
		dat    Block
		repeat uint64
		err    error
	)
	switch n.repeat.Type {
	case Integer:
		repeat, err = strconv.ParseUint(n.repeat.Literal, 0, 64)
	case Float:
		if f, e := strconv.ParseFloat(n.repeat.Literal, 64); e == nil {
			repeat = uint64(f)
		} else {
			err = e
		}
	case Ident, Text:
		if v, e := root.ResolveValue(n.repeat.Literal); e == nil {
			repeat = asUint(v)
		} else {
			err = e
		}
	default:
		err = fmt.Errorf("unsupported token type %s", TokenString(n.repeat))
	}
	if err != nil {
		return err
	}
	if repeat == 0 {
		repeat++
	}
	switch n := n.node.(type) {
	case Block:
		dat = n
	case Reference:
		dat, err = root.ResolveBlock(n.id.Literal)
	}
	if err != nil {
		return err
	}
	for i := uint64(0); i < repeat; i++ {
		if err := root.decodeBlock(dat); err != nil {
			break
		}
	}
	return err
}

func (root *state) decodeInclude(n Include) error {
	if n.Predicate != nil && !evalPredicate(n.Predicate, root) {
		return ErrSkip
	}
	var (
		data Block
		err  error
	)
	switch n := n.node.(type) {
	case Block:
		data = n
	case Reference:
		data, err = root.ResolveBlock(n.id.Literal)
	}
	if err == nil {
		err = root.decodeBlock(data)
	}
	return err
}

func evalApply(v Value, n Node, root *state) error {
	tok, ok := n.(Token)
	if !ok {
		v.Set(v)
		return nil
	}
	p, err := root.ResolvePair(tok.Literal)
	if err != nil {
		return err
	}
	var fn func([]Constant, Value) Value
	switch p.kind.Literal {
	case kwEnum:
		fn = evalEnum
	case kwPoly:
		fn = evalPoly
	case kwPoint:
		fn = evalPoint
	}
	x := fn(p.nodes, v)
	v.Set(x)
	return nil
}

func evalPoint(cs []Constant, v Value) Value {
	raw := asInt(v)
	for i := 0; i < len(cs); i++ {
		c := cs[i]
		id, _ := strconv.ParseInt(c.id.Literal, 0, 64)
		if raw == id {
			val, _ := strconv.ParseFloat(c.value.Literal, 64)
			return &Real{
				Meta: Meta{Id: v.String()},
				Raw:  val,
			}
		}
		if j := i + 1; j < len(cs) {
			next, _ := strconv.ParseInt(cs[j].id.Literal, 0, 64)
			if id < raw && raw < next {
				// linear interpolation
				break
			}
		}
	}
	return v
}

func evalEnum(cs []Constant, v Value) Value {
	raw := asInt(v)
	for _, c := range cs {
		id, _ := strconv.ParseInt(c.id.Literal, 0, 64)
		if raw == id {
			v := &String{
				Meta: Meta{Id: v.String()},
				Raw:  c.value.Literal,
			}
			return v
		}
	}
	return nil
}

func evalPoly(cs []Constant, v Value) Value {
	var (
		raw = asReal(v)
		eng float64
	)
	for _, c := range cs {
		pow, _ := strconv.ParseFloat(c.id.Literal, 64)
		mul, _ := strconv.ParseFloat(c.value.Literal, 64)

		eng += mul * math.Pow(raw, pow)
	}
	return &Real{
		Meta: Meta{Id: v.String()},
		Raw:  eng,
	}
}

func evalPredicate(e Expression, root *state) bool {
	switch e := e.(type) {
	case Binary:
		return evalBinaryExpression(e, root)
	case Unary:
		return !evalPredicate(e, root)
	default:
		return false
	}
}

func evalBinaryExpression(b Binary, root *state) bool {
	switch b.operator {
	default:
	case And:
		return evalPredicate(b.Left, root) && evalPredicate(b.Right, root)
	case Or:
		return evalPredicate(b.Left, root) || evalPredicate(b.Right, root)
	}

	left, err := resolveValue(b.Left, root)
	if err != nil {
		return false
	}
	right, err := resolveValue(b.Right, root)
	if err != nil {
		return false
	}

	var (
		ok  bool
		cmp = left.Cmp(right)
	)
	switch b.operator {
	case Equal:
		ok = cmp == 0
	case NotEq:
		ok = cmp != 0
	case Lesser:
		ok = cmp < 0
	case Greater:
		ok = cmp > 0
	case LessEq:
		ok = cmp == 0 || cmp < 0
	case GreatEq:
		ok = cmp == 0 || cmp > 0
	default:
	}
	return ok
}

func resolveValue(e Expression, root *state) (Value, error) {
	var (
		v   Value
		err error
	)
	switch e := e.(type) {
	default:
		err = fmt.Errorf("unexpected expression type %T", e)
	case Literal:
		if id := e.id.Literal; e.id.Type == Float {
			r := &Real{Meta: Meta{Id: id}}
			r.Raw, err = strconv.ParseFloat(e.id.Literal, 64)

			v = r
		} else if e.id.Type == Integer {
			i := &Int{Meta: Meta{Id: id}}
			i.Raw, err = strconv.ParseInt(e.id.Literal, 0, 64)

			v = i
		} else if e.id.Type == Bool {
			b := &Boolean{Meta: Meta{Id: id}}
			b.Raw, err = strconv.ParseBool(e.id.Literal)

			v = b
		} else {
			err = fmt.Errorf("unexpected token type %s", TokenString(e.id))
		}
	case Identifier:
		v, err = root.ResolveValue(e.id.Literal)
	}
	return v, err
}

func btoi(buf []byte, shift, mask int) uint64 {
	var (
		u uint64
		n = len(buf)
	)
	for i := n - 1; i >= 0; i-- {
		n := uint64(buf[i]) << (numbit * (n - (i + 1)))
		u += n
	}
	return (u >> uint64(shift)) & uint64(mask)
}

func numbytes(bits int) int {
	n := numbit - ((bits - 1) % numbit)
	return (bits + n) / numbit
}

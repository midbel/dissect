package dissect

import (
	"fmt"
	"io"
	"strings"
)

type Value interface{}

type Value struct {
	Name  string
	Pos   int
	Raw   interface{}
	Value interface{}
}

func (v Value) Equal(value Value) bool {
	return false
}

func (v Value) Lesser(value Value) bool {
	return false
}

func (v Value) Notequal(value Value) bool {
	return !v.Equal(value)
}

func (v Value) LesserOrEqual(value Value) bool {
	return v.Equal(value) || v.Lesser(value)
}

func (v Value) Greater(value Value) bool {
	return v.NotEqual(value) && !v.Lesser(value)
}

func (v Value) GreaterOrEqual(value Value) bool {
	return v.Equal(value) || v.Greater(value)
}

// type resolver interface {
// 	Resolve([]Value) (Value, error)
// }

type accepter interface {
	Accept([]Value) bool
}

type reverse struct {
	right accepter
}

func (r reverse) Accept(env []Value) bool {
	return !r.right.Accept(env)
}

type relational struct {
	left     accepter
	right    accepter
	operator rune
}

func (r relational) Accept(env []Value) bool {
	switch r.operator {
	case And:
		return r.left.Accept(env) && r.right.Accept(env)
	case Or:
		return r.left.Accept(env) || r.right.Accept(env)
	default:
		return false
	}
}

type logical struct {
	value    Value
	operator rune
}

func (g logical) Accept(env []Value) bool {
	var ok bool
	switch g.operator {
	case Equal:
	case NotEq:
	case Lesser:
	case LessEq:
	case Greater:
	case GreatEq:
	default:
		return false
	}
	return ok
}

type decoder interface {
	Numbit() int
	Decode([]byte, []Value) ([]Value, error)
}

type Decoder struct {
	decoders []decoder
}

func NewDecoder(r io.Reader) (*Decoder, error) {
	n, err := Parse(r)
	if err != nil {
		return nil, err
	}
	root, ok := n.(Block)
	if !ok {
		return nil, fmt.Errorf("root node is not a block")
	}
	dat, err := root.ResolveData()
	if err != nil {
		return nil, err
	}
	ds, err := build(dat, root)
	if err != nil {
		return nil, err
	}
	return &Decoder{decoders: ds}, nil
}

func (d Decoder) Dump() {
	for _, d := range d.decoders {
		dumpDecoder(d, 0)
	}
}

func (d Decoder) Decode(buf []byte) ([]Value, error) {
	var values []Value
	for _, d := range d.decoders {
		vs, err := d.Decode(buf, values)
		if err != nil {
			return nil, err
		}
		if len(vs) > 0 {
			values = append(values, vs...)
		}
	}
	return values, nil
}

type chunk struct {
	name   string
	accept accepter

	decoders []decoder

	offset int
	repeat int
}

func (c chunk) Numbit() int {
	var z int
	for _, d := range c.decoders {
		z += d.Numbit()
	}
	return z
}

func (c chunk) Decode(buf []byte, env []Value) ([]Value, error) {
	if c.accept != nil && !c.accept.Accept(env) {
		return nil, nil
	}
	if c.repeat == 0 {
		c.repeat++
	}
	var values []Value
	for i := 0; i < c.repeat; i++ {
		for _, d := range c.decoders {
			vs, err := d.Decode(buf, env)
			if err != nil {
				return nil, err
			}
			if len(vs) > 0 {
				values = append(values, vs...)
			}
		}
	}
	return values, nil
}

type field struct {
	name string
	kind string

	size   int
	offset int
	repeat int
}

func (f field) Numbit() int {
	return f.size
}

func (f field) Decode(buf []byte, env []Value) ([]Value, error) {
	if f.repeat == 0 {
		f.repeat++
	}
	var values []Value
	for i := 0; i < f.repeat; i++ {
		v, err := f.decodeRaw(buf)
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	return values, nil
}

func (f field) decodeRaw(buf []byte) (Value, error) {
	var v Value
	switch f.kind {
	case "u8":
	case "u16":
	case "u32":
	case "u64":
	case "i8":
	case "i16":
	case "i32":
	case "i64":
	case "f16":
	case "f32":
	case "f64":
	case "bool":
	default:
		return nil, fmt.Errorf("unsupported type %s", f.kind)
	}
	return v, nil
}

func build(dat, root Block) ([]decoder, error) {
	var decs []decoder
	for _, n := range dat.nodes {
		var d decoder
		switch n := n.(type) {
		case Parameter:
			d = field{
				name: n.id.Literal,
				size: n.numbit(),
			}
		case Reference:
			p, err := root.ResolveParameter(n.id.Literal)
			if err != nil {
				return nil, err
			}
			d = field{
				name: p.id.Literal,
				size: p.numbit(),
			}
		case Include:
			var (
				b   Block
				err error
				cdt accepter
			)
			cdt, err = accept(n.Predicate, root)
			if err != nil {
				return nil, err
			}
			if r, ok := n.node.(Reference); ok {
				b, err = root.ResolveBlock(r.id.Literal)
			} else if x, ok := n.node.(Block); ok {
				b = x
			} else {
				err = fmt.Errorf("unexpected block type %T", n.node)
			}
			if err != nil {
				return nil, err
			}
			ds, err := build(b, root)
			if err != nil {
				return nil, err
			}
			d = chunk{
				name:     b.id.Literal,
				accept:   cdt,
				decoders: ds,
			}
		default:
			return nil, fmt.Errorf("unexpected node type %T", n)
		}
		decs = append(decs, d)
	}
	return decs, nil
}

func accept(n Node, root Block) (accepter, error) {
	if n == nil {
		return nil, nil
	}
	switch n := n.(type) {
	case Negate:
		a, err := accept(n.Right, root)
		if err == nil {
			a = reverse{right: a}
		}
		return a, nil
	case Predicate:
		if n.operator == And || n.operator == Or {
			left, err := accept(n.Left, root)
			if err != nil {
				return nil, err
			}
			right, err := accept(n.Right, root)
			if err != nil {
				return nil, err
			}
			return relational{
				left:     left,
				right:    right,
				operator: n.operator,
			}, nil
		}
		left, ok := n.Left.(Token)
		if !ok {
			return nil, fmt.Errorf("unexpected node Type: %T", left)
		}
		right, ok := n.Right.(Token)
		if !ok {
			return nil, fmt.Errorf("unexpected node Type: %T", right)
		}
		return logical{operator: n.operator}, nil
	default:
		return nil, fmt.Errorf("unexpected node type %T", n)
	}
	return nil, nil
}

func dumpDecoder(d decoder, level int) {
	indent := strings.Repeat(" ", level*2)
	switch d := d.(type) {
	case field:
		fmt.Printf("%sfield(id: %s, numbit: %d)\n", indent, d.name, d.Numbit())
	case chunk:
		fmt.Printf("%sblock(id: %s, numbit: %d) (\n", indent, d.name, d.Numbit())
		for _, d := range d.decoders {
			dumpDecoder(d, level+1)
		}
		fmt.Println(indent + ")")
	default:
		return
	}
}

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
)

const numbit = 8

type Value struct {
	Name  string
	Pos   int
	Raw   interface{}
	Value interface{}
}

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
	_, vs, err := decodeBlock(d.data, d.root, buf)
	return vs, err
}

func decodeBlock(data, root Block, buf []byte) (int, []Value, error) {
	var (
		pos    int
		values []Value
	)
	for _, n := range data.nodes {
		ix, offset := pos/numbit, pos%numbit
		if ix >= len(buf) {
			break
		}
		switch n := n.(type) {
		case LetStmt:
			// ignore for now
		case DelStmt:
			// ignore for now
		case SeekStmt:
			seek, err := strconv.Atoi(n.offset.Literal)
			if err != nil {
				return pos, nil, fmt.Errorf("invalid seek value given")
			}
			pos += seek
			if pos < 0 || pos >= len(buf) {
				return pos, nil, fmt.Errorf("seek outside of buffer range")
			}
		case Repeat:
			// ignore for now
		case Reference:
			p, err := root.ResolveParameter(n.id.Literal)
			if err != nil {
				return pos, nil, err
			}
			bits, val, err := decodeParameter(p, root, offset, buf[ix:])
			if err != nil {
				return pos, nil, err
			}
			val.Pos = pos + p.offset()

			values = append(values, val)
			pos += bits
		case Parameter:
			bits, val, err := decodeParameter(n, root, offset, buf[ix:])
			if err != nil {
				return pos, nil, err
			}
			val.Pos = pos + n.offset()

			values = append(values, val)
			pos += bits
		case Include:
			bits, vs, err := decodeInclude(n, root, buf[ix:])

			if err != nil && !errors.Is(err, ErrSkip) {
				return pos, nil, err
			}
			if len(vs) > 0 {
				values = append(values, vs...)
			}
			pos += bits
		default:
			return pos, nil, fmt.Errorf("unexpected node type %T", n)
		}
	}
	return pos, values, nil
}

func decodeParameter(p Parameter, root Block, offset int, buf []byte) (int, Value, error) {
	bits := p.numbit()
	offset += p.offset()

	var (
		need  = numbytes(bits)
		shift = (numbit * need) - (offset + bits)
		mask  = 1
		raw   interface{}
	)
	if bits > 1 {
		mask = (1 << bits) - 1
	}
	if n := len(buf); n < need {
		return bits, Value{}, fmt.Errorf("buffer too short (missing %d bytes)", need-n)
	}
	dat := btoi(buf[:need], shift, mask)
	switch p.is() {
	case 'i': // signed integer
		raw = int64(dat)
	case 'u': // unsigned integer
		raw = dat
	case 'f': // float
		raw = math.Float64frombits(dat)
	case 'b': // boolean
		if dat == 0 {
			raw = false
		} else {
			raw = true
		}
	default:
		raw = dat
	}
	val := Value{
		Name:  p.id.Literal,
		Raw:   raw,
		Value: raw,
	}

	return bits, val, nil
}

func decodeInclude(n Include, root Block, buf []byte) (int, []Value, error) {
	if n.Predicate != nil && !evalPredicate(n.Predicate, root) {
		return 0, nil, ErrSkip
	}
	var (
		bits   int
		data   Block
		err    error
		values []Value
	)
	switch n := n.node.(type) {
	case Block:
		data = n
	case Reference:
		data, err = root.ResolveBlock(n.id.Literal)
	}
	if err == nil {
		bits, values, err = decodeBlock(data, root, buf)
	}
	return bits, values, err
}

func evalPredicate(e Expression, root Block) bool {
	switch e := e.(type) {
	case Binary:
		return evalBinaryExpression(e, root)
	case Unary:
		return !evalPredicate(e, root)
	default:
		return false
	}
}

func evalBinaryExpression(b Binary, root Block) bool {
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
	_, _ = left, right

	var ok bool
	switch b.operator {
	case Equal:
	case NotEq:
	case Lesser:
	case Greater:
	case LessEq:
	case GreatEq:
	default:
	}
	return ok
}

func resolveValue(e Expression, root Block) (Value, error) {
	var (
		v   Value
		err error
	)
	switch e := e.(type) {
	default:
		err = fmt.Errorf("unexpected expression type %T", e)
	case Literal:
		if e.id.Type == Float {
			v.Raw, err = strconv.ParseFloat(e.id.Literal, 64)
		} else if e.id.Type == Integer {
			v.Raw, err = strconv.ParseInt(e.id.Literal, 0, 64)
		} else if e.id.Type == Bool {
			v.Raw, err = strconv.ParseBool(e.id.Literal)
		} else {
			err = fmt.Errorf("unexpected token type %s", TokenString(e.id))
		}
		v.Name = "anonymous"
	case Identifier:
		v.Name = e.id.Literal
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

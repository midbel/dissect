package dissect

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

var (
	ErrIncompatible = errors.New("incompatible type")
	ErrUnsupported  = errors.New("unsupported operation")
)

type Value interface {
	fmt.Stringer
	Offset() int
	Cmp(v Value) int
	Set(v Value)

	add(Value) (Value, error)
	subtract(Value) (Value, error)
	multiply(Value) (Value, error)
	divide(Value) (Value, error)
	modulo(Value) (Value, error)
	reverse() (Value, error)

	leftshift(Value) (Value, error)
	rightshift(Value) (Value, error)
	and(Value) (Value, error)
	or(Value) (Value, error)

	setId(string)
	eng() Value
	skip() bool
}

type Meta struct {
	Id  string
	Pos int
	Eng Value
}

func (m *Meta) Set(v Value) {
	m.Eng = v
}

func (m *Meta) Offset() int {
	return m.Pos
}

func (m *Meta) String() string {
	return m.Id
}

func (m *Meta) setId(s string) {
	m.Id = s
}

func (m *Meta) skip() bool {
	return len(m.Id) == 0 || m.Id[0] == underscore
}

func (m *Meta) eng() Value {
	if m.Eng == nil {
		return &Null{}
	}
	return m.Eng
}

type Null struct {
	Meta
}

func (n *Null) Cmp(v Value) int {
	if _, ok := v.(*Null); ok {
		return 0
	}
	return -1
}

func (n *Null) add(v Value) (Value, error) {
	return null2null(v)
}

func (n *Null) subtract(v Value) (Value, error) {
	return null2null(v)
}

func (n *Null) multiply(v Value) (Value, error) {
	return null2null(v)
}

func (n *Null) divide(v Value) (Value, error) {
	return null2null(v)
}

func (n *Null) modulo(v Value) (Value, error) {
	return null2null(v)
}

func (n *Null) reverse() (Value, error) {
	return n, nil
}

func (n *Null) leftshift(v Value) (Value, error) {
	return null2null(v)
}

func (n *Null) rightshift(v Value) (Value, error) {
	return null2null(v)
}

func (n *Null) and(v Value) (Value, error) {
	return null2null(v)
}

func (n *Null) or(v Value) (Value, error) {
	return null2null(v)
}

type Boolean struct {
	Meta
	Raw bool
}

func (b *Boolean) Cmp(v Value) int {
	o, ok := v.(*Boolean)
	if !ok {
		return -1
	}
	if o.Raw == b.Raw {
		return 0
	}
	if b.Raw == false {
		return -1
	}
	return 1
}

func (b *Boolean) add(_ Value) (Value, error)        { return nil, ErrUnsupported }
func (b *Boolean) subtract(_ Value) (Value, error)   { return nil, ErrUnsupported }
func (b *Boolean) multiply(_ Value) (Value, error)   { return nil, ErrUnsupported }
func (b *Boolean) divide(_ Value) (Value, error)     { return nil, ErrUnsupported }
func (b *Boolean) modulo(_ Value) (Value, error)     { return nil, ErrUnsupported }
func (b *Boolean) reverse() (Value, error)           { return nil, ErrUnsupported }
func (b *Boolean) leftshift(_ Value) (Value, error)  { return nil, ErrUnsupported }
func (b *Boolean) rightshift(_ Value) (Value, error) { return nil, ErrUnsupported }
func (b *Boolean) and(_ Value) (Value, error)        { return nil, ErrUnsupported }
func (b *Boolean) or(_ Value) (Value, error)         { return nil, ErrUnsupported }

type Int struct {
	Meta
	Raw int64
}

func (i *Int) Cmp(v Value) int {
	if x := asInt(v); i.Raw > x {
		return 1
	} else if i.Raw < x {
		return -1
	} else {
		return 0
	}
}

func (i *Int) add(v Value) (Value, error) {
	if v, ok := v.(*String); ok {
		return concatValues(i, v)
	}
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw += asInt(v)
	return &x, nil
}

func (i *Int) subtract(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw -= asInt(v)
	return &x, nil
}

func (i *Int) multiply(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw *= asInt(v)
	return &x, nil
}

func (i *Int) divide(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw /= asInt(v)
	return &x, nil
}

func (i *Int) modulo(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw %= asInt(v)
	return &x, nil
}

func (i *Int) reverse() (Value, error) {
	x := *i
	x.Raw = -x.Raw
	return &x, nil
}

func (i *Int) leftshift(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw = x.Raw << asInt(v)
	return &x, nil
}

func (i *Int) rightshift(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw = x.Raw >> asInt(v)
	return &x, nil
}

func (i *Int) and(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw = x.Raw & asInt(v)
	return &x, nil
}

func (i *Int) or(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw = x.Raw | asInt(v)
	return &x, nil
}

type Uint struct {
	Meta
	Raw uint64
}

func (i *Uint) Cmp(v Value) int {
	if x := asUint(v); i.Raw > x {
		return 1
	} else if i.Raw < x {
		return -1
	} else {
		return 0
	}
}

func (i *Uint) add(v Value) (Value, error) {
	if v, ok := v.(*String); ok {
		return concatValues(i, v)
	}
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw += asUint(v)
	return &x, nil
}

func (i *Uint) subtract(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw -= asUint(v)
	return &x, nil
}

func (i *Uint) multiply(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw *= asUint(v)
	return &x, nil
}

func (i *Uint) divide(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw /= asUint(v)
	return &x, nil
}

func (i *Uint) modulo(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw %= asUint(v)
	return &x, nil
}

func (i *Uint) reverse() (Value, error) { return nil, ErrUnsupported }

func (i *Uint) leftshift(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw = x.Raw << asUint(v)
	return &x, nil
}

func (i *Uint) rightshift(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw = x.Raw >> asUint(v)
	return &x, nil
}

func (i *Uint) and(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw = x.Raw & asUint(v)
	return &x, nil
}

func (i *Uint) or(v Value) (Value, error) {
	if !isCompatible(i, v) {
		return nil, ErrIncompatible
	}
	x := *i
	x.Raw = x.Raw | asUint(v)
	return &x, nil
}

type Real struct {
	Meta
	Raw float64
}

func (r *Real) Cmp(v Value) int {
	if x := asReal(v); r.Raw > x {
		return 1
	} else if r.Raw < x {
		return -1
	} else {
		return 0
	}
}

func (r *Real) add(v Value) (Value, error) {
	if !isCompatible(r, v) {
		return nil, ErrIncompatible
	}
	x := *r
	x.Raw += asReal(v)
	return &x, nil
}

func (r *Real) subtract(v Value) (Value, error) {
	if !isCompatible(r, v) {
		return nil, ErrIncompatible
	}
	x := *r
	x.Raw -= asReal(v)
	return &x, nil
}

func (r *Real) multiply(v Value) (Value, error) {
	if !isCompatible(r, v) {
		return nil, ErrIncompatible
	}
	x := *r
	x.Raw *= asReal(v)
	return &x, nil
}

func (r *Real) divide(v Value) (Value, error) {
	if !isCompatible(r, v) {
		return nil, ErrIncompatible
	}
	x := *r
	x.Raw /= asReal(v)
	return &x, nil
}

func (r *Real) modulo(v Value) (Value, error) { return nil, ErrUnsupported }

func (r *Real) reverse() (Value, error) {
	x := *r
	x.Raw = -x.Raw
	return &x, nil
}

func (r *Real) leftshift(_ Value) (Value, error)  { return nil, ErrUnsupported }
func (r *Real) rightshift(_ Value) (Value, error) { return nil, ErrUnsupported }
func (r *Real) and(_ Value) (Value, error)        { return nil, ErrUnsupported }
func (r *Real) or(_ Value) (Value, error)         { return nil, ErrUnsupported }

type Bytes struct {
	Meta
	Raw []byte
}

func (b *Bytes) Cmp(v Value) int {
	str, ok := v.(*Bytes)
	if !ok {
		return -1
	}
	return bytes.Compare(b.Raw, str.Raw)
}

func (b *Bytes) add(v Value) (Value, error)        { return nil, ErrUnsupported }
func (b *Bytes) subtract(v Value) (Value, error)   { return nil, ErrUnsupported }
func (b *Bytes) multiply(v Value) (Value, error)   { return nil, ErrUnsupported }
func (b *Bytes) divide(v Value) (Value, error)     { return nil, ErrUnsupported }
func (b *Bytes) modulo(v Value) (Value, error)     { return nil, ErrUnsupported }
func (b *Bytes) reverse() (Value, error)           { return nil, ErrUnsupported }
func (b *Bytes) leftshift(_ Value) (Value, error)  { return nil, ErrUnsupported }
func (b *Bytes) rightshift(_ Value) (Value, error) { return nil, ErrUnsupported }
func (b *Bytes) and(_ Value) (Value, error)        { return nil, ErrUnsupported }
func (b *Bytes) or(_ Value) (Value, error)         { return nil, ErrUnsupported }

type String struct {
	Meta
	Raw string
}

func (s *String) Cmp(v Value) int {
	str, ok := v.(*String)
	if !ok {
		return -1
	}
	return strings.Compare(s.Raw, str.Raw)
}

func (s *String) add(v Value) (Value, error) {
	return concatValues(s, v)
}

func (s *String) subtract(v Value) (Value, error)   { return nil, ErrUnsupported }
func (s *String) multiply(v Value) (Value, error)   { return nil, ErrUnsupported }
func (s *String) divide(v Value) (Value, error)     { return nil, ErrUnsupported }
func (s *String) modulo(v Value) (Value, error)     { return nil, ErrUnsupported }
func (s *String) reverse() (Value, error)           { return nil, ErrUnsupported }
func (s *String) leftshift(_ Value) (Value, error)  { return nil, ErrUnsupported }
func (s *String) rightshift(_ Value) (Value, error) { return nil, ErrUnsupported }
func (s *String) and(_ Value) (Value, error)        { return nil, ErrUnsupported }
func (s *String) or(_ Value) (Value, error)         { return nil, ErrUnsupported }

func concatValues(left, right Value) (Value, error) {
	ls, rs := asString(left), asString(right)
	s := String{Raw: ls + rs}
	return &s, nil
}

func appendRaw(buf []byte, v Value, escape bool) []byte {
	switch v := v.(type) {
	case *Int:
		buf = strconv.AppendInt(buf, v.Raw, 10)
	case *Uint:
		buf = strconv.AppendUint(buf, v.Raw, 10)
	case *Real:
		buf = strconv.AppendFloat(buf, v.Raw, 'g', -1, 64)
	case *Boolean:
		buf = strconv.AppendBool(buf, v.Raw)
	case *String:
		strmap := func(r rune) rune {
			if r == utf8.RuneError || !unicode.IsPrint(r) {
				r = '*'
			}
			return r
		}
		buf = bytes.Map(strmap, []byte(v.Raw))
		if escape {
			buf = escapeQuotes(buf)
		}
	case *Bytes:
		x := hex.EncodeToString(v.Raw)
		buf = []byte(x)
	default:
	}
	return buf
}

func appendEng(buf []byte, v Value, escape bool) []byte {
	var eng Value
	switch v := v.(type) {
	case *Int:
		eng = v.Eng
	case *Uint:
		eng = v.Eng
	case *Real:
		eng = v.Eng
	case *Boolean:
		eng = v.Eng
	case *String:
		eng = v.Eng
	case *Bytes:
		eng = v.Eng
	default:
	}
	if eng == nil {
		eng = v
	}
	return appendRaw(buf, eng, escape)
}

func escapeQuotes(buf []byte) []byte {
	return bytes.ReplaceAll(buf, []byte("\""), []byte("\"\""))
}

func asString(v Value) string {
	switch v := v.(type) {
	case *Int:
		return strconv.FormatInt(v.Raw, 10)
	case *Uint:
		return strconv.FormatUint(v.Raw, 10)
	case *Real:
		return strconv.FormatFloat(v.Raw, 'g', -1, 64)
	case *Boolean:
		return strconv.FormatBool(v.Raw)
	case *Bytes:
		return hex.EncodeToString(v.Raw)
	case *String:
		return v.Raw
	default:
		return ""
	}
}

func asReal(v Value) float64 {
	switch v := v.(type) {
	case *Real:
		return v.Raw
	case *Uint:
		return float64(v.Raw)
	case *Int:
		return float64(v.Raw)
	default:
		return 0
	}
}

func asUint(v Value) uint64 {
	switch v := v.(type) {
	case *Uint:
		return v.Raw
	case *Int:
		return uint64(v.Raw)
	case *Real:
		return uint64(v.Raw)
	default:
		return 0
	}
}

func asInt(v Value) int64 {
	switch v := v.(type) {
	case *Int:
		return v.Raw
	case *Uint:
		return int64(v.Raw)
	case *Real:
		return int64(v.Raw)
	default:
		return 0
	}
}

func asBool(v Value) bool {
	switch v := v.(type) {
	case *Boolean:
		return v.Raw
	case *Int:
		return v.Raw != 0
	case *Uint:
		return v.Raw != 0
	case *String:
		return len(v.Raw) > 0
	case *Bytes:
		return len(v.Raw) > 0
	default:
		return false
	}
}

func null2null(v Value) (Value, error) {
	if _, ok := v.(*Null); ok {
		return v, nil
	}
	return nil, ErrIncompatible
}

func isCompatible(left, right Value) bool {
	for _, v := range []Value{left, right} {
		switch v.(type) {
		case *Int, *Uint, *Real:
		default:
			return false
		}
	}
	return true
}

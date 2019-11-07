package dissect

import (
	"bytes"
	"fmt"
	"strings"
)

type Value interface {
	fmt.Stringer
	Cmp(Value) int
	Set(Value)
}

type Meta struct {
	Id  string
	Pos int
	Eng Value
}

func (m *Meta) Set(v Value) {
	m.Eng = v
}

func (m *Meta) String() string {
	return m.Id
}

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
	} else {
		return 1
	}
}

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

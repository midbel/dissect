package dissect

import (
	"fmt"
)

type Value interface {
	fmt.Stringer
	Cmp(Value) int
}

type Boolean struct {
	Id  string
	Pos int
	Raw bool
}

func (b Boolean) Cmp(v Value) int {
	o, ok := v.(Boolean)
	if !ok {
		return -1
	}
	if o.Raw == b.Raw {
		return 0
	}
	return -1
}

func (b Boolean) String() string {
	return b.Id
}

type Int struct {
	Id  string
	Pos int
	Raw int64
}

func (i Int) Cmp(v Value) int {
	if x := asInt(v); i.Raw > x {
		return 1
	} else if i.Raw < x {
		return -1
	} else {
		return 0
	}
}

func (i Int) String() string {
	return i.Id
}

type Uint struct {
	Id  string
	Pos int
	Raw uint64
}

func (i Uint) Cmp(v Value) int {
	if x := asUint(v); i.Raw > x {
		return 1
	} else if i.Raw < x {
		return -1
	} else {
		return 0
	}
}

func (i Uint) String() string {
	return i.Id
}

type Real struct {
	Id  string
	Pos int
	Raw float64
}

func (r Real) Cmp(v Value) int {
	if x := asReal(v); r.Raw > x {
		return 1
	} else if r.Raw < x {
		return -1
	} else {
		return 0
	}
}

func (f Real) String() string {
	return f.Id
}

func asReal(v Value) float64 {
	switch v := v.(type) {
	case Real:
		return v.Raw
	case Uint:
		return float64(v.Raw)
	case Int:
		return float64(v.Raw)
	default:
		return 0
	}
}

func asUint(v Value) uint64 {
	switch v := v.(type) {
	case Uint:
		return v.Raw
	case Int:
		return uint64(v.Raw)
	case Real:
		return uint64(v.Raw)
	default:
		return 0
	}
}

func asInt(v Value) int64 {
	switch v := v.(type) {
	case Int:
		return v.Raw
	case Uint:
		return int64(v.Raw)
	case Real:
		return int64(v.Raw)
	default:
		return 0
	}
}

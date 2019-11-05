package dissect

import (
	"fmt"
)

type Value interface {
	fmt.Stringer
	Equal(Value) bool
	Less(Value) bool
	True() bool
}

type Boolean struct {
	Id  string
	Pos int
	Raw bool
}

func (b Boolean) String() string {
	return b.Id
}

func (b Boolean) True() bool {
	return b.Raw
}

func (b Boolean) Equal(v Value) bool {
	if o, ok := v.(Boolean); ok {
		return o.Raw == b.Raw
	}
	return false
}

func (b Boolean) Less(v Value) bool {
	return false
}

type Int struct {
	Id  string
	Pos int
	Raw int64
}

func (i Int) String() string {
	return i.Id
}

func (i Int) True() bool {
	return i.Raw != 0
}

func (i Int) Equal(v Value) bool {
	return i.Raw == asInt(v)
}

func (i Int) Less(v Value) bool {
	return i.Raw < asInt(v)
}

type Uint struct {
	Id  string
	Pos int
	Raw uint64
}

func (i Uint) String() string {
	return i.Id
}

func (i Uint) True() bool {
	return i.Raw != 0
}

func (i Uint) Equal(v Value) bool {
	return i.Raw == asUint(v)
}

func (i Uint) Less(v Value) bool {
	return i.Raw < asUint(v)
}

type Real struct {
	Id  string
	Pos int
	Raw float64
}

func (f Real) String() string {
	return f.Id
}

func (f Real) True() bool {
	return f.Raw != 0
}

func (f Real) Equal(v Value) bool {
	return f.Raw == asReal(v)
}

func (f Real) Less(v Value) bool {
	return f.Raw < asReal(v)
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

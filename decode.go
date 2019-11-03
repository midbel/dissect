package dissect

import (
	"fmt"
	"io"
)

type Value interface{}

type Expression interface {
	Skip([]Value) bool
}

type decoder interface {
	Decode([]byte, []Value) ([]Value, error)
}

type Decoder struct {
	decoders []decoder
}

func NewDecoder(r io.Reader) (*Decoder, error) {
	_, err := Parse(r)
	if err != nil {
		return nil, err
	}
	return nil, nil
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
	name string
	expr Expression

	decoders []decoder

	offset int
	repeat int
}

func (c chunk) Decode(buf []byte, env []Value) ([]Value, error) {
	if c.expr != nil && c.expr.Skip(env) {
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

func (f field) Decode(buf []byte, env []Value) ([]Value, error) {
	// repeat := f.repeat.Resolve(env)
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

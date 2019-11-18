package dissect

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

var (
	ErrSkip     = errors.New("skip block")
	ErrDone     = errors.New("done")
	errBreak    = errors.New("break")
	errContinue = errors.New("continue")
)

const numbit = 8

type state struct {
	Block
	Values []Value
	files  map[string]*os.File

	buffer []byte
	Pos    int

	currentBlock string
	currentFile  string

	reader *bufio.Reader
}

func (root *state) Close() error {
	var err error
	for _, f := range root.files {
		if e := f.Close(); e != nil {
			err = e
		}
	}
	return err
}

func (root *state) Run(dat Block, r io.Reader) error {
	root.Reset(r)
	for {
		if err := root.growBuffer(0); err != nil {
			return err
		}
		if root.Size() == 0 {
			break
		}
		if err := root.decodeBlock(dat); err != nil {
			if errors.Is(err, ErrDone) {
				break
			}
			return err
		}
		root.Values = root.Values[:0]
		root.buffer = root.buffer[root.Pos/numbit:]
		root.Pos = 0
	}
	return nil
}

func (root *state) Reset(r io.Reader) {
	if n, ok := r.(interface{ Name() string }); ok {
		root.currentFile = n.Name()
	} else {
		root.currentFile = "stream"
	}
	root.reader = bufio.NewReader(r)
	root.buffer = root.buffer[:0]
	root.Pos = 0
}

func (root *state) Size() int {
	return len(root.buffer) * numbit
}

func (root *state) ResolveValue(n string) (Value, error) {
	for i := len(root.Values) - 1; i >= 0; i-- {
		v := root.Values[i]
		if v.String() == n {
			return v, nil
		}
	}
	return nil, fmt.Errorf("%s: value not defined", n)
}

func (root *state) DeleteValue(n string) {
	for i := 0; ; i++ {
		if i >= len(root.Values) {
			break
		}
		if v := root.Values[i]; v.String() == n {
			root.Values = append(root.Values[:i], root.Values[i+1:]...)
		}
	}
}

func (root *state) decodeBlock(data Block) error {
	for _, n := range data.nodes {
		switch n := n.(type) {
		case Break:
			return root.decodeBreak(n)
		case Continue:
			return root.decodeContinue(n)
		case Echo:
			return root.decodeEcho(n)
		case Print:
			return root.decodePrint(n)
		case ExitStmt:
			return root.decodeExit(n)
		case LetStmt:
			val, err := root.decodeLet(n)
			if err != nil {
				return err
			}
			if val != nil {
				root.Values = append(root.Values, val)
			}
		case DelStmt:
			for _, n := range n.nodes {
				r, ok := n.(Reference)
				if !ok {
					continue
				}
				root.DeleteValue(r.id.Literal)
			}
		case SeekStmt:
			if err := root.decodeSeek(n); err != nil {
				return err
			}
		case Repeat:
			// ignore for now
			if err := root.decodeRepeat(n); err != nil {
				return err
			}
		case Match:
			if err := root.decodeMatch(n); err != nil {
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

func (root *state) decodeEcho(e Echo) error {
	return nil
}

func (root *state) decodePrint(p Print) error {
	var w io.Writer
	if f, ok := root.files[p.file.Literal]; ok {
		w = f
	} else {
		if file := p.file.Literal; file == "" || file == "-" {
			w = os.Stdout
		} else {
			f, err := os.Create(p.file.Literal)
			if err != nil {
				return err
			}
			root.files[file], w = f, f
		}
	}
	k := struct {
		Format string
		Method string
	}{
		Format: p.format.Literal,
		Method: p.method.Literal,
	}
	print, ok := printers[k]
	if !ok {
		return fmt.Errorf("print: unsupported method %s for format %s", p.method, p.format)
	}
	return print(w, resolveValues(root, p.values))
}

func (root *state) growBuffer(bits int) error {
	pos := (root.Pos + bits) / numbit
	if bits > 0 && pos < len(root.buffer) {
		return nil
	}

	xs := make([]byte, 4096+(bits/numbit))
	n, err := root.reader.Read(xs)
	if n > 0 {
		root.buffer = append(root.buffer, xs[:n]...)
	}
	if err != nil && err != io.EOF {
		return err
	}
	return nil
}

func (root *state) decodeParameter(p Parameter) (Value, error) {
	var (
		bits   int
		offset = root.Pos % numbit
		index  = root.Pos / numbit
	)

	switch p.size.Type {
	case Ident, Text:
		v, err := root.ResolveValue(p.size.Literal)
		if err != nil {
			return nil, err
		}
		bits = int(asInt(v))
	case Integer:
		v, _ := strconv.ParseInt(p.size.Literal, 0, 64)
		bits = int(v)
	default:
		return nil, fmt.Errorf("unexpected token type")
	}

	var (
		err error
		raw Value
	)

	switch p.is() {
	case kindBytes, kindString:
		if offset != 0 {
			err = fmt.Errorf("bytes/string should start at offset 0")
			break
		}
		if err := root.growBuffer(bits * numbit); err != nil {
			return nil, err
		}
		raw, err = root.decodeBytes(p, bits, index)
		bits *= numbit
	default:
		if err := root.growBuffer(bits * numbit); err != nil {
			return nil, err
		}
		raw, err = root.decodeNumber(p, bits, index, offset)
		if err == nil {
			err = root.evalApply(raw, p.apply)
		}
	}
	if err != nil {
		return raw, err
	}
	if p.expect != nil {
		expect, err := eval(p.expect, root)
		if err != nil {
			return nil, err
		}
		if cmp := raw.Cmp(expect); cmp != 0 {
			return nil, fmt.Errorf("%s expectation failed: want %s, got %s", p, expect, raw)
		}
	}
	root.Pos += bits

	return raw, nil
}

func (root *state) decodeBytes(p Parameter, bits, index int) (Value, error) {
	var (
		meta = Meta{Id: p.id.Literal, Pos: root.Pos}
		raw  Value
	)
	switch p.is() {
	case kindBytes:
		raw = &Bytes{
			Meta: meta,
			Raw:  root.buffer[index : index+bits],
		}
	case kindString:
		str := root.buffer[index : index+bits]
		raw = &String{
			Meta: meta,
			Raw:  strings.Trim(string(str), "\x00"),
		}
	default:
		return nil, fmt.Errorf("unsupported type: %s", p.is())
	}
	return raw, nil
}

func (root *state) decodeNumber(p Parameter, bits, index, offset int) (Value, error) {
	var (
		need  = numbytes(bits)
		shift = (numbit * need) - (offset + bits)
		mask  = 1
		raw   Value
	)
	if bits > 1 {
		mask = (1 << bits) - 1
	}
	if n := root.Size(); n < index+need {
		return nil, fmt.Errorf("buffer too short (missing %d bytes)", need-n)
	}
	meta := Meta{
		Id:  p.id.Literal,
		Pos: root.Pos,
	}
	var (
		buf = swapBytes(root.buffer[index:index+need], p.endian.Literal)
		dat = btoi(buf, shift, mask)
	)
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
		raw = &Real{
			Meta: meta,
			Raw:  math.Float64frombits(dat),
		}
	default:
		return nil, fmt.Errorf("unsupported type: %s", p.is())
	}
	return raw, nil
}

func (root *state) decodeLet(e LetStmt) (Value, error) {
	return eval(e.expr, root)
}

func (root *state) decodeExit(e ExitStmt) error {
	var code int64
	switch e.code.Type {
	case Integer:
		code, _ = strconv.ParseInt(e.code.Literal, 0, 64)
	case Float:
		f, _ := strconv.ParseFloat(e.code.Literal, 64)
		code = int64(f)
	case Ident, Text:
		v, err := root.ResolveValue(e.code.Literal)
		if err != nil {
			return err
		}
		code = asInt(v)
	default:
		return fmt.Errorf("unexpected token type")
	}
	return &ExitError{code}
}

func (root *state) decodeMatch(n Match) error {
	v, err := root.ResolveValue(n.id.Literal)
	if err != nil {
		return err
	}
	var (
		cdt  = asInt(v)
		node Node
	)
	for _, c := range n.nodes {
		var raw int64
		switch c.cond.Type {
		case Integer:
			raw, _ = strconv.ParseInt(c.cond.Literal, 0, 64)
		case Float:
			f, _ := strconv.ParseFloat(c.cond.Literal, 64)
			raw = int64(f)
		case Ident, Text:
			v, err := root.ResolveValue(c.cond.Literal)
			if err != nil {
				return err
			}
			raw = asInt(v)
		default:
			return fmt.Errorf("unsupported type: %s", TokenString(c.cond))
		}
		if cdt == raw {
			node = c.node
			break
		}
	}

	if node == nil {
		if pos := n.alt.Pos(); !pos.IsValid() {
			return nil
		}
		node = n.alt.node
	}

	var dat Block
	switch n := node.(type) {
	case Reference:
		dat, err = root.ResolveBlock(n.id.Literal)
	case Block:
		dat = n
	default:
		return fmt.Errorf("unexpected node type %T", n)
	}
	if err == nil {
		err = root.decodeBlock(dat)
	}
	return err
}

func (root *state) decodeContinue(n Continue) error {
	v, err := eval(n.expr, root)
	if err != nil {
		return err
	}
	if isTrue(v) {
		err = errContinue
	}
	return err
}

func (root *state) decodeBreak(n Break) error {
	v, err := eval(n.expr, root)
	if err != nil {
		return err
	}
	if isTrue(v) {
		err = errBreak
	}
	return err
}

func (root *state) decodeSeek(n SeekStmt) error {
	v, err := eval(n.offset, root)
	if err != nil {
		return err
	}
	seek := int(asInt(v))
	if err := root.growBuffer(seek); err != nil {
		return err
	}
	if n.absolute {
		root.Pos = seek
	} else {
		root.Pos += seek
	}
	if root.Pos < 0 || root.Pos > root.Size() {
		return fmt.Errorf("seek outside of buffer range (%d >= %d)", root.Pos, root.Size())
	}
	return nil
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
		if err = root.decodeBlock(dat); err != nil {
			if errors.Is(err, errContinue) {
				continue
			}
			if errors.Is(err, errBreak) {
				err = nil
			}
			break
		}
	}
	return err
}

func (root *state) decodeInclude(n Include) error {
	if n.Predicate != nil {
		ok, err := eval(n.Predicate, root)
		if err != nil {
			return err
		}
		if !isTrue(ok) {
			return ErrSkip
		}
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

func (root *state) evalApply(v Value, n Node) error {
	tok, ok := n.(Token)
	if !ok {
		v.Set(v)
		return nil
	}
	p, err := root.ResolvePair(tok.Literal)
	if err != nil {
		return err
	}
	var fn func([]Constant, Value) (Value, error)
	switch p.kind.Literal {
	case kwEnum:
		fn = root.evalEnum
	case kwPoly:
		fn = root.evalPoly
	case kwPoint:
		fn = root.evalPoint
	}
	x, err := fn(p.nodes, v)
	if err == nil {
		v.Set(x)
	}
	return err
}

func (root *state) evalPoint(cs []Constant, v Value) (Value, error) {
	raw := asInt(v)
	for i := 0; i < len(cs); i++ {
		c := cs[i]
		id, _ := strconv.ParseInt(c.id.Literal, 0, 64)
		if raw == id {
			val, _ := strconv.ParseFloat(c.value.Literal, 64)
			return &Real{
				Meta: Meta{Id: v.String()},
				Raw:  val,
			}, nil
		}
		if j := i + 1; j < len(cs) {
			next, _ := strconv.ParseInt(cs[j].id.Literal, 0, 64)
			if id < raw && raw < next {
				// linear interpolation
				break
			}
		}
	}
	return v, nil
}

func (root *state) evalEnum(cs []Constant, v Value) (Value, error) {
	raw := asInt(v)
	for _, c := range cs {
		id, _ := strconv.ParseInt(c.id.Literal, 0, 64)
		if raw == id {
			v := &String{
				Meta: Meta{Id: v.String()},
				Raw:  c.value.Literal,
			}
			return v, nil
		}
	}
	return v, nil
}

func (root *state) evalPoly(cs []Constant, v Value) (Value, error) {
	var (
		raw = asReal(v)
		eng float64
	)
	for _, c := range cs {
		mul, _ := strconv.ParseFloat(c.id.Literal, 64)
		pow, _ := strconv.ParseFloat(c.value.Literal, 64)

		eng += mul * math.Pow(raw, pow)
	}
	return &Real{
		Meta: Meta{Id: v.String()},
		Raw:  eng,
	}, nil
}

func swapBytes(buf []byte, e string) []byte {
	if e == kwLittle {
		dat := make([]byte, len(buf))
		if n := len(buf); n <= 8 && n%2 == 0 {
			for i := 0; i < n; i++ {
				dat[n-1-i] = buf[i]
			}
		} else {
			copy(dat, buf)
		}
		buf = dat
	}
	return buf
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

package dissect

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

var (
	ErrSkip     = errors.New("skip block")
	ErrDone     = errors.New("done")
	errBreak    = errors.New("break")
	errContinue = errors.New("continue")
	errShort    = errors.New("short buffer")
)

const numbit = 8

// type Option func(*Interpreter) error
//
// func WithStdout(std io.Writer) Option {
// 	return func(i *Interpreter) error {
// 		i.stdout = std
// 	}
// }
//
// func WithStderr(std io.Writer) Option {
// 	return func(i *Interpreter) error {
// 		i.stdout = std
// 	}
// }
//
// func WithWordLen(n uint8) Option {
// 	return func(i *Interpreter) error {
// 		i.wordlen = int(n)
// 	}
// }
//
// func WithInclude(files []string) Option {
// 	return func(i *Interpreter) error {
// 		return nil
// 	}
// }
//
// type Interpreter struct {
// 	stdout  io.Writer
// 	stderr  io.Writer
//  wordlen int
// }
//
// func New(r io.Reader, opts ...Option) (*Interpreter, error) {
// 	return nil, nil
// }
//
// func (i Interpreter) Run(r io.Reader) error {
// 	return nil
// }

type Field struct {
	Block string
	Id    string
	Pos   int
	Len   int
	Ix    int

	raw Value
	eng Value
}

func (f Field) String() string {
	s := f.Id
	if f.Block != "" {
		s = fmt.Sprintf("%s.%s", f.Block, s)
	}
	return s
}

func (f Field) Offset() int {
	return f.Pos
}

func (f Field) Skip() bool {
	return len(f.Id) == 0 || f.Id[0] == underscore || f.Len == 0
}

func (f Field) Raw() Value {
	return f.raw
}

func (f Field) Eng() Value {
	if f.eng == nil {
		return f.raw
	}
	return f.eng
}

type state struct {
	Block
	data Block

	Fields []Field
	files  map[string]*os.File

	reader *bufio.Reader
	buffer []byte
	Pos    int
	Loop   int
	Iter   int

	blocks      []string
	currentFile string

	stdout io.Writer
	stderr io.Writer
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

func (root *state) Run(r io.Reader) error {
	root.Reset(r)

	for {
		if err := root.growBuffer(4096); err != nil {
			return err
		}
		if root.Size() == 0 {
			break
		}
		if err := root.decodeBlock(root.data); err != nil {
			if errors.Is(err, ErrDone) {
				break
			}
			return fmt.Errorf("%s: %w", root.path(), err)
		}
		root.Loop++
		root.reset()
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
	root.Loop = 0
}

func (root *state) reset() {
	if offset := root.Pos / numbit; offset < len(root.buffer) {
		root.buffer = root.buffer[offset:]
	} else {
		root.buffer = root.buffer[:0]
	}
	root.Fields = root.Fields[:0]
	root.Pos = 0
}

func (root *state) growBuffer(bits int) error {
	pos := (root.Pos + bits) / numbit
	if n := len(root.buffer); bits > 0 && pos < n {
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

func (root *state) Size() int {
	return len(root.buffer) * numbit
}

func (root *state) ResolveInternal(str string) (Field, error) {
	var (
		field = Field{Id: str}
		err   error
	)
	switch str {
	case "Iter":
		field.raw = &Int{
			Raw: int64(root.Iter),
		}
	case "Loop":
		field.raw = &Int{
			Raw: int64(root.Loop),
		}
	case "Time":
		field.raw = &Int{
			Raw: time.Now().Unix(),
		}
	case "Num":
		field.raw = &Int{
			Raw: int64(len(root.Fields)),
		}
	case "Pos":
		field.raw = &Int{
			Raw: int64(root.Pos),
		}
	case "Size":
		field.raw = &Int{
			Raw: int64(root.Size()),
		}
	case "File":
		field.raw = &String{
			Raw: root.currentFile,
		}
	case "Block":
		block := "block"
		if b := root.currentBlock(); b != "" {
			block = b
		}
		field.raw = &String{
			Raw: block,
		}
	case "Path":
		field.raw = &String{
			Raw: root.path(),
		}
	default:
		err = fmt.Errorf("%s: unknown internal value", str)
	}
	return field, err
}

func (root *state) ResolveValue(n string) (Field, error) {
	for i := len(root.Fields) - 1; i >= 0; i-- {
		v := root.Fields[i]
		if v.Id == n {
			return v, nil
		}
	}
	return Field{}, fmt.Errorf("%s: field not defined", n)
}

func (root *state) DeleteValue(n string) {
	for i := 0; ; i++ {
		if i >= len(root.Fields) {
			break
		}
		if v := root.Fields[i]; v.Id == n {
			root.Fields = append(root.Fields[:i], root.Fields[i+1:]...)
		}
	}
}

func (root *state) currentBlock() string {
	n := len(root.blocks)
	if n == 0 {
		return "/"
	}
	return root.blocks[n-1]
}

func (root *state) path() string {
	return "/" + strings.Join(root.blocks, "/")
}

func (root *state) pushBlock(b string) {
	root.blocks = append(root.blocks, b)
}

func (root *state) popBlock() {
	n := len(root.blocks)
	if n > 0 {
		root.blocks = root.blocks[:n-1]
	}
}

func (root *state) decodeBlock(data Block) error {
	root.pushBlock(data.id.Literal)
	defer root.popBlock()

	var err error
	switch n := data.pre.(type) {
	case Block:
		err = root.decodeNodes(n.nodes)
	case Reference:
		p, err := root.ResolveBlock(n.id.Literal)
		if err != nil {
			return err
		}
		err = root.decodeNodes(p.nodes)
	}
	if err != nil {
		return err
	}

	if err := root.decodeNodes(data.nodes); err != nil {
		return err
	}

	switch n := data.post.(type) {
	case Block:
		err = root.decodeNodes(n.nodes)
	case Reference:
		p, err := root.ResolveBlock(n.id.Literal)
		if err != nil {
			return err
		}
		err = root.decodeNodes(p.nodes)
	}
	if err != nil {
		return err
	}
	return nil
}

func (root *state) decodeNodes(nodes []Node) error {
	for _, n := range nodes {
		if n == nil {
			continue
		}
		switch n := n.(type) {
		case Break:
			return root.decodeBreak(n)
		case Continue:
			return root.decodeContinue(n)
		case Copy:
			if err := root.decodeCopy(n); err != nil {
				return err
			}
		case Echo:
			if err := root.decodeEcho(n); err != nil {
				return err
			}
		case Print:
			if err := root.decodePrint(n); err != nil {
				return err
			}
		case Exit:
			return root.decodeExit(n)
		case Let:
			val, err := root.decodeLet(n)
			if err != nil {
				return err
			}
			root.Fields = append(root.Fields, val)
		case Del:
			for _, n := range n.nodes {
				r, ok := n.(Reference)
				if !ok {
					continue
				}
				root.DeleteValue(r.id.Literal)
			}
		case Peek:
			if err := root.decodePeek(n); err != nil {
				return err
			}
		case Seek:
			if err := root.decodeSeek(n); err != nil {
				return err
			}
		case If:
			if err := root.decodeIf(n); err != nil {
				return err
			}
		case Repeat:
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
			root.Fields = append(root.Fields, val)
		case Parameter:
			val, err := root.decodeParameter(n)
			if err != nil {
				return err
			}
			root.Fields = append(root.Fields, val)
		case Block:
			if err := root.decodeBlock(n); err != nil {
				return err
			}
		case Include:
			err := root.decodeInclude(n)
			if err != nil && !errors.Is(err, ErrSkip) {
				return err
			}
		default:
			return fmt.Errorf("decoding block: unexpected node type %T", n)
		}
	}
	return nil
}

func (root *state) openFile(file string, echo bool) (io.Writer, error) {
	if file == "" || file == "-" {
		if echo {
			return root.stderr, nil
		}
		return root.stdout, nil
	}
	if file == "/dev/null" {
		return ioutil.Discard, nil
	}
	w, ok := root.files[root.path()]
	if ok && w.Name() == file {
		return w, nil
	}
	if ok {
		w.Close()
		delete(root.files, root.path())
	}
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, err
	}
	w, err := os.Create(file)
	if err != nil {
		return nil, err
	}
	root.files[root.path()] = w
	return w, nil
}

func (root *state) decodeEcho(e Echo) error {
	w, err := root.openFile(e.file.Literal, true)
	if err != nil {
		return err
	}
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 64)
	)
	for _, e := range e.expr {
		v, err := eval(e, root)
		if err != nil {
			return err
		}
		buf.Write(appendRaw(dat, v, false))
	}
	buf.WriteString("\r\n")
	_, err = io.Copy(w, &buf)
	return err
}

func (root *state) decodeCopy(c Copy) error {
	if c.predicate != nil {
		v, err := eval(c.predicate, root)
		if err != nil {
			return err
		}
		if !isTrue(v) {
			return nil
		}
	}

	v, err := eval(c.count, root)
	if err != nil {
		return err
	}

	file := c.file.Literal
	if c.file.Type == Ident {
		v, err := root.ResolveValue(file)
		if err == nil {
			file = asString(v.Raw())
		}
	}
	w, err := root.openFile(file, false)
	if err != nil {
		return err
	}

	count := int(asInt(v))
	if err := root.growBuffer(count); err != nil {
		return err
	}
	var (
		index = root.Pos / numbit
		buf   = root.buffer[index : index+count]
	)
	switch c.format.Literal {
	case kwString:
		_, err = io.WriteString(w, hex.EncodeToString(buf))
	case kwBytes:
		_, err = w.Write(buf)
	}
	return err
}

func (root *state) decodePrint(p Print) error {
	if p.predicate != nil {
		v, err := eval(p.predicate, root)
		if err != nil {
			return err
		}
		if !isTrue(v) {
			return nil
		}
	}
	file := p.file.Literal
	if p.file.Type == Ident {
		v, err := root.ResolveValue(file)
		if err == nil {
			file = asString(v.Raw())
		}
	}
	w, err := root.openFile(file, false)
	if err != nil {
		return err
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

func (root *state) decodeParameter(p Parameter) (Field, error) {
	var (
		bits   int
		offset = root.Pos % numbit
		index  = root.Pos / numbit
	)

	switch p.size.Type {
	case Ident, Text:
		v, err := root.ResolveValue(p.size.Literal)
		if err != nil {
			return Field{}, err
		}
		bits = int(asInt(v.raw))
	case Integer:
		v, _ := strconv.ParseInt(p.size.Literal, 0, 64)
		bits = int(v)
	default:
		return Field{}, fmt.Errorf("unexpected token type")
	}

	var (
		err error
		raw Field
	)

	switch p.is() {
	case kindBytes, kindString:
		if offset != 0 {
			err = fmt.Errorf("bytes/string should start at offset 0")
			break
		}
		if err := root.growBuffer(bits * numbit); err != nil {
			return Field{}, err
		}
		raw, err = root.decodeBytes(p, bits, index)
		bits *= numbit
	default:
		if err := root.growBuffer(bits * numbit); err != nil {
			return Field{}, err
		}
		raw, err = root.decodeNumber(p, bits, index, offset)
		if err == nil {
			raw, err = root.evalApply(raw, p.apply)
		}
	}
	if err != nil {
		return raw, err
	}
	if p.expect != nil {
		expect, err := eval(p.expect, root)
		if err != nil {
			return Field{}, err
		}
		if cmp := raw.Raw().Cmp(expect); cmp != 0 {
			return Field{}, fmt.Errorf("%s expectation failed: want %s, got %s", p, expect, raw)
		}
	}
	root.Pos += bits
	raw.Block, raw.Ix = root.currentBlock(), root.Iter
	return raw, nil
}

func (root *state) decodeBytes(p Parameter, bits, index int) (Field, error) {
	raw := Field{
		Id:  p.id.Literal,
		Pos: root.Pos,
		Len: bits * numbit,
	}
	if n := root.Size() / numbit; n < index+bits {
		return Field{}, fmt.Errorf("%w: missing %d bytes (decoding %s.%s)", errShort, (index+bits)-n, root.currentBlock(), p)
	}
	switch kind := p.is(); kind {
	case kindBytes:
		raw.raw = &Bytes{
			Raw: root.buffer[index : index+bits],
		}
	case kindString:
		str := root.buffer[index : index+bits]
		raw.raw = &String{
			Raw: strings.Trim(string(str), "\x00"),
		}
	default:
		return Field{}, fmt.Errorf("unsupported type: %s", kind)
	}
	return raw, nil
}

func (root *state) decodeNumber(p Parameter, bits, index, offset int) (Field, error) {
	var (
		need  = numbytes(bits)
		shift = (numbit * need) - (offset + bits)
		mask  = 1
	)
	if bits > 1 {
		mask = (1 << bits) - 1
	}
	if n := root.Size() / numbit; n < index+need {
		return Field{}, fmt.Errorf("%w: missing %d bytes (decoding %s.%s)", errShort, (index+need)-n, root.currentBlock(), p)
	}
	raw := Field{
		Id:  p.id.Literal,
		Pos: root.Pos,
		Len: bits,
	}
	var (
		buf = swapBytes(root.buffer[index:index+need], p.endian.Literal)
		dat = btoi(buf, shift, mask)
	)
	switch kind := p.is(); kind {
	case kindInt: // signed integer
		raw.raw = &Int{
			Raw: int64(dat),
		}
	case kindUint: // unsigned integer
		raw.raw = &Uint{
			Raw: dat,
		}
	case kindFloat: // float
		raw.raw = &Real{
			Raw: math.Float64frombits(dat),
		}
	default:
		return Field{}, fmt.Errorf("unsupported type: %s", kind)
	}
	return raw, nil
}

func (root *state) decodeLet(e Let) (Field, error) {
	v, err := eval(e.expr, root)
	if err != nil {
		return Field{}, err
	}
	f := Field{
		Id:  e.id.Literal,
		raw: v,
		eng: v,
	}
	return f, nil
}

func (root *state) decodeExit(e Exit) error {
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
		code = asInt(v.raw)
	default:
		return fmt.Errorf("unexpected token type")
	}
	return &ExitError{code}
}

func (root *state) decodeIf(i If) error {
	e, err := eval(i.expr, root)
	if err != nil {
		return err
	}
	var node Node
	if isTrue(e) {
		node = i.csq
	} else {
		node = i.alt
	}
	if node == nil {
		return nil
	}
	var dat Block
	switch n := node.(type) {
	case Reference:
		dat, err = root.ResolveBlock(n.id.Literal)
	case Block:
		dat = n
	case If:
		return root.decodeIf(n)
	default:
		return fmt.Errorf("decoding if: unexpected node type %T", n)
	}
	if err == nil {
		err = root.decodeBlock(dat)
	}
	return err
}

func (root *state) decodeMatch(n Match) error {
	var (
		node Node
		err  error
	)
	if n.expr == nil {
		node, err = root.matchExpr(n)
	} else {
		node, err = root.matchIdent(n)
	}
	if err != nil {
		return err
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
		return fmt.Errorf("decoding match: unexpected node type %T", n)
	}
	if err == nil {
		err = root.decodeBlock(dat)
	}
	return err
}

func (root *state) matchIdent(n Match) (Node, error) {
	e, err := eval(n.expr, root)
	if err != nil {
		return nil, err
	}
	for _, c := range n.nodes {
		r, err := eval(c.cond, root)
		if err != nil {
			return nil, err
		}
		if e.Cmp(r) == 0 {
			return c.node, nil
		}
	}
	return nil, nil
}

func (root *state) matchExpr(n Match) (Node, error) {
	for _, c := range n.nodes {
		e, err := eval(c.cond, root)
		if err != nil {
			return nil, err
		}
		if isTrue(e) {
			return c.node, nil
		}
	}
	return nil, nil
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

func (root *state) decodePeek(n Peek) error {
	v, err := eval(n.count, root)
	if err != nil {
		return err
	}
	return root.growBuffer(int(asInt(v)))
}

func (root *state) decodeSeek(n Seek) error {
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
		dat Block
		err error
	)
	switch n := n.node.(type) {
	case Block:
		dat = n
	case Reference:
		dat, err = root.ResolveBlock(n.id.Literal)
	}
	if err != nil {
		return err
	}
	var eval func(Expression, Block) error
	if n.repeat.isBoolean() {
		eval = root.evalRepeatBool
	} else {
		eval = root.evalRepeatUint
	}
	root.Iter = 0
	return eval(n.repeat, dat)
}

func (root *state) evalRepeatBool(expr Expression, dat Block) error {
	var (
		val Value
		err error
	)
	for val, err = eval(expr, root); err == nil && isTrue(val); val, err = eval(expr, root) {
		if err = root.decodeBlock(dat); err != nil {
			if errors.Is(err, errContinue) {
				continue
			}
			if errors.Is(err, errBreak) {
				err = nil
			}
			break
		}
		root.Iter++
	}
	return err
}

func (root *state) evalRepeatUint(expr Expression, dat Block) error {
	v, err := eval(expr, root)
	if err != nil {
		return err
	}
	repeat := asUint(v)
	if repeat == 0 {
		repeat++
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
		root.Iter++
	}
	return err
}

func (root *state) decodeInclude(n Include) error {
	if n.cond != nil {
		ok, err := eval(n.cond, root)
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

func (root *state) evalApply(v Field, n Node) (Field, error) {
	var (
		pair Pair
		err  error
	)
	switch n := n.(type) {
	case Token:
		pair, err = root.ResolvePair(n.Literal)
	case Pair:
		pair = n
	default:
		return v, nil
	}
	if err != nil {
		return Field{}, err
	}
	var fn func([]Constant, Value) (Value, error)
	switch pair.kind.Literal {
	case kwEnum:
		fn = root.evalEnum
	case kwPoly:
		fn = root.evalPoly
	case kwPoint:
		fn = root.evalPoint
	}
	x, err := fn(pair.nodes, v.raw)
	if err == nil {
		v.eng = x
	}
	return v, err
}

func (root *state) evalPoint(cs []Constant, v Value) (Value, error) {
	raw := asInt(v)
	for i := 0; i < len(cs); i++ {
		c := cs[i]
		id, _ := strconv.ParseInt(c.id.Literal, 0, 64)
		if raw == id {
			val, err := eval(c.value, root)
			if err != nil {
				return nil, err
			}
			return &Real{
				Raw: asReal(val),
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
			str, err := eval(c.value, root)
			if err != nil {
				return nil, err
			}
			v := &String{
				Raw: asString(str),
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
		pv, err := eval(c.value, root)
		if err != nil {
			return nil, err
		}
		pow, _ := strconv.ParseFloat(c.id.Literal, 64)
		mul := asReal(pv)

		eng += mul * math.Pow(raw, pow)
	}
	return &Real{
		Raw: eng,
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

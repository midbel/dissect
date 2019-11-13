package dissect

import (
	"bytes"
	"io"
	"sort"
	"strconv"
)

func printDebug(w io.Writer, root *state, vs []Token) error {
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 32)
	)
	for _, v := range resolveValues(root, vs) {
		var (
			offset = v.Offset()
			index  = offset / 8
		)

		buf.WriteString(strconv.Itoa(index))
		buf.WriteRune(comma)
		buf.WriteString(strconv.Itoa(offset))
		buf.WriteRune(comma)
		buf.WriteString(v.String())
		buf.WriteRune(comma)
		buf.Write(appendRaw(dat, v))
		buf.WriteRune(comma)
		buf.Write(appendEng(dat, v))
		buf.WriteString("\r\n")

		if _, err := io.Copy(w, &buf); err != nil {
			return err
		}
	}
	return nil
}

func printRaw(w io.Writer, root *state, vs []Token) error {
	var (
		buf bytes.Buffer
		dat []byte
	)
	for i, v := range resolveValues(root, vs) {
		if i > 0 {
			buf.WriteRune(comma)
		}
		buf.Write(appendRaw(dat, v))
	}
	buf.WriteString("\r\n")
	_, err := io.Copy(w, &buf)
	return err
}

func printEng(w io.Writer, root *state, vs []Token) error {
	var (
		buf bytes.Buffer
		dat []byte
	)
	for i, v := range resolveValues(root, vs) {
		if i > 0 {
			buf.WriteRune(comma)
		}
		buf.Write(appendEng(dat, v))
	}
	buf.WriteString("\r\n")
	_, err := io.Copy(w, &buf)
	return err
}

func printBoth(w io.Writer, root *state, vs []Token) error {
	var (
		buf bytes.Buffer
		dat []byte
	)
	for i, v := range resolveValues(root, vs) {
		if i > 0 {
			buf.WriteRune(comma)
		}
		buf.Write(appendRaw(dat, v))
		buf.WriteRune(comma)
		buf.Write(appendEng(dat, v))
	}
	buf.WriteString("\r\n")
	_, err := io.Copy(w, &buf)
	return err
}

func resolveValues(root *state, vs []Token) []Value {
	if len(vs) == 0 {
		return root.Values
	}
	xs := make([]Value, 0, len(vs))
	for _, v := range vs {
		x, err := root.ResolveValue(v.Literal)
		if err != nil {
			continue
		}
		xs = append(xs, x)
	}
	sort.Slice(xs, func(i, j int) bool {
		return xs[i].Offset() < xs[j].Offset()
	})
	return xs
}

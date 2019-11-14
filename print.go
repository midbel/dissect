package dissect

import (
	"bytes"
	"io"
	"sort"
	"strconv"
)

type printFunc func(io.Writer, []Value) error

 var printers = map[struct{ Format, Method string }]printFunc{
	{Format: fmtCSV, Method: methRaw}:     csvPrintRaw,
	{Format: fmtCSV, Method: methEng}:     csvPrintEng,
	{Format: fmtCSV, Method: methBoth}:    csvPrintBoth,
	{Format: fmtCSV, Method: methDebug}:   csvPrintDebug,
	{Format: fmtTuple, Method: methDebug}: sexpPrintDebug,
	{Format: fmtSexp, Method: methDebug}:  sexpPrintDebug,
}

func sexpPrintDebug(w io.Writer, values []Value) error {
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 32)
	)
	for _, v := range values {
		buf.WriteRune(lparen)

		var (
			offset = v.Offset()
			index  = offset / 8
		)

		buf.WriteString(strconv.Itoa(index))
		buf.WriteRune(colon)
		buf.WriteString(strconv.Itoa(offset))
		buf.WriteRune(colon)
		buf.WriteString(v.String())
		buf.WriteRune(colon)
		buf.Write(appendRaw(dat, v))
		buf.WriteRune(colon)
		buf.Write(appendEng(dat, v))

		buf.WriteRune(rparen)
		if _, err := io.Copy(w, &buf); err != nil {
			return err
		}
	}
	return nil
}

func csvPrintDebug(w io.Writer, values []Value) error {
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 32)
	)
	for _, v := range values {
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

func csvPrintRaw(w io.Writer, values []Value) error {
	var (
		buf bytes.Buffer
		dat []byte
	)
	for i, v := range values {
		if i > 0 {
			buf.WriteRune(comma)
		}
		buf.Write(appendRaw(dat, v))
	}
	buf.WriteString("\r\n")
	_, err := io.Copy(w, &buf)
	return err
}

func csvPrintEng(w io.Writer, values []Value) error {
	var (
		buf bytes.Buffer
		dat []byte
	)
	for i, v := range values {
		if i > 0 {
			buf.WriteRune(comma)
		}
		buf.Write(appendEng(dat, v))
	}
	buf.WriteString("\r\n")
	_, err := io.Copy(w, &buf)
	return err
}

func csvPrintBoth(w io.Writer, values []Value) error {
	var (
		buf bytes.Buffer
		dat []byte
	)
	for i, v := range values {
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

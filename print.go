package dissect

import (
	"bytes"
	"io"
	"strconv"
	"strings"
)

var headersDebug = []string{
	"bytoff",
	"bitoff",
	"block",
	"param",
	"len",
	"raw",
	"eng",
}

type printFunc func(io.Writer, []Field) error

var printers = map[struct{ Format, Method string }]printFunc{
	{Format: fmtCSV, Method: methRaw}:     csvPrintRaw,
	{Format: fmtCSV, Method: methEng}:     csvPrintEng,
	{Format: fmtCSV, Method: methBoth}:    csvPrintBoth,
	{Format: fmtCSV, Method: methDebug}:   csvPrintDebug,
	{Format: fmtTuple, Method: methDebug}: sexpPrintDebug,
	{Format: fmtSexp, Method: methDebug}:  sexpPrintDebug,
	{Format: fmtTuple, Method: methRaw}:   sexpPrintRaw,
	{Format: fmtSexp, Method: methRaw}:    sexpPrintRaw,
	{Format: fmtTuple, Method: methEng}:   sexpPrintEng,
	{Format: fmtSexp, Method: methEng}:    sexpPrintEng,
	{Format: fmtTuple, Method: methBoth}:  sexpPrintBoth,
	{Format: fmtSexp, Method: methBoth}:   sexpPrintBoth,
}

func sexpPrintDebug(w io.Writer, values []Field) error {
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 32)
	)
	buf.WriteRune(lparen)
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
		buf.WriteString(strconv.Itoa(v.Len))
		buf.WriteRune(colon)
		buf.Write(appendRaw(dat, v.Raw(), false))
		buf.WriteRune(colon)
		buf.Write(appendEng(dat, v.Eng(), false))

		buf.WriteRune(rparen)
	}
	buf.WriteRune(rparen)

	_, err := io.Copy(w, &buf)
	return err
}

func sexpPrintRaw(w io.Writer, values []Field) error {
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 32)
	)
	buf.WriteRune(lparen)
	for i, v := range values {
		if v.Skip() {
			continue
		}
		if i > 0 {
			buf.WriteRune(space)
		}
		buf.Write(appendRaw(dat, v.Raw(), true))
	}
	buf.WriteRune(rparen)

	_, err := io.Copy(w, &buf)
	return err
}

func sexpPrintEng(w io.Writer, values []Field) error {
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 32)
	)
	buf.WriteRune(lparen)
	for i, v := range values {
		if v.Skip() {
			continue
		}
		if i > 0 {
			buf.WriteRune(space)
		}
		buf.Write(appendEng(dat, v.Eng(), true))
	}
	buf.WriteRune(rparen)

	_, err := io.Copy(w, &buf)
	return err
}

func sexpPrintBoth(w io.Writer, values []Field) error {
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 32)
	)
	buf.WriteRune(lparen)
	for i, v := range values {
		if v.Skip() {
			continue
		}
		if i > 0 {
			buf.WriteRune(space)
		}
		buf.WriteRune(lparen)
		buf.Write(appendRaw(dat, v.Raw(), true))
		buf.WriteRune(space)
		buf.Write(appendEng(dat, v.Eng(), true))
		buf.WriteRune(rparen)
	}
	buf.WriteRune(rparen)

	_, err := io.Copy(w, &buf)
	return err
}

func csvPrintHeaders(w io.Writer, meth string, values []Field) error {
	var (
		buf     bytes.Buffer
		headers []string
	)
	if meth == methDebug {
		headers = headersDebug
	} else {
		headers = make([]string, 0, len(values))
		for i := 0; i < len(values); i++ {
			if strings.HasPrefix(values[i].Id, "_") {
				continue
			}
			headers = append(headers, values[i].Id)
		}
	}
	for i := 0; i < len(headers); i++ {
		if i > 0 {
			buf.WriteRune(comma)
		}
		buf.WriteRune('"')
		buf.WriteString(headers[i])
		buf.WriteRune('"')
	}
	buf.WriteString("\r\n")

	_, err := io.Copy(w, &buf)
	return err
}

func csvPrintDebug(w io.Writer, values []Field) error {
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 64)
	)
	for _, v := range values {
		var (
			offset = v.Offset()
			index  = offset / numbit
		)

		buf.WriteRune('"')
		buf.WriteString(strconv.Itoa(index))
		buf.WriteRune('"')
		buf.WriteRune(comma)
		buf.WriteRune('"')
		buf.WriteString(strconv.Itoa(offset))
		buf.WriteRune('"')
		buf.WriteRune(comma)
		buf.WriteRune('"')
		buf.WriteString(v.Block)
		buf.WriteRune('"')
		buf.WriteRune(comma)
		buf.WriteRune('"')
		buf.WriteString(v.Id)
		buf.WriteRune('"')
		buf.WriteRune(comma)
		buf.WriteRune('"')
		buf.WriteString(strconv.Itoa(v.Len))
		buf.WriteRune('"')
		buf.WriteRune(comma)
		buf.WriteRune('"')
		buf.Write(appendRaw(dat, v.Raw(), true))
		buf.WriteRune('"')
		buf.WriteRune(comma)
		buf.WriteRune('"')
		buf.Write(appendEng(dat, v.Eng(), true))
		buf.WriteRune('"')
		buf.WriteString("\r\n")

		if _, err := io.Copy(w, &buf); err != nil {
			return err
		}
	}
	return nil
}

func csvPrintRaw(w io.Writer, values []Field) error {
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 64)
	)
	for i, v := range values {
		if v.Skip() {
			continue
		}
		if i > 0 {
			buf.WriteRune(comma)
		}
		buf.WriteRune('"')
		buf.Write(appendRaw(dat, v.Raw(), true))
		buf.WriteRune('"')
	}
	buf.WriteString("\r\n")
	_, err := io.Copy(w, &buf)
	return err
}

func csvPrintEng(w io.Writer, values []Field) error {
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 64)
	)
	for i, v := range values {
		if v.Skip() {
			continue
		}
		if i > 0 {
			buf.WriteRune(comma)
		}
		buf.WriteRune('"')
		buf.Write(appendEng(dat, v.Eng(), true))
		buf.WriteRune('"')
	}
	buf.WriteString("\r\n")
	_, err := io.Copy(w, &buf)
	return err
}

func csvPrintBoth(w io.Writer, values []Field) error {
	var (
		buf bytes.Buffer
		dat = make([]byte, 0, 64)
	)
	for i, v := range values {
		if v.Skip() {
			continue
		}
		if i > 0 {
			buf.WriteRune(comma)
		}
		buf.WriteRune('"')
		buf.Write(appendRaw(dat, v.Raw(), true))
		buf.WriteRune('"')
		buf.WriteRune(comma)
		buf.WriteRune('"')
		buf.Write(appendEng(dat, v.Eng(), true))
		buf.WriteRune('"')
	}
	buf.WriteString("\r\n")
	_, err := io.Copy(w, &buf)
	return err
}

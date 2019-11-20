package dissect

import (
	"fmt"
	"strings"
)

type expander interface {
	expand(*state) (string, error)
}

func expand(str string) (expander, error) {
	var (
		offset int
		es     []expander
	)
	for {
		ix := strings.IndexByte(str[offset:], lparen)
		if ix < 0 {
			break
		}
		if c := str[offset+ix-1]; c != '%' {
			continue
		}
		es = append(es, raw(str[offset:offset+ix-1]))

		offset += ix
		if ix = strings.IndexByte(str[offset:], rparen); ix < 0 || ix == offset+1 {
			return nil, fmt.Errorf("placeholder: invalid syntax (empty or not closed)")
		}
		offset += ix
	}
	return set(es), nil
}

type set []expander

func (es set) expand(root *state) (string, error) {
	var buf strings.Builder
	for _, e := range es {
		str, err := e.expand(root)
		if err != nil {
			return "", err
		}
		buf.WriteString(str)
	}
	return buf.String(), nil
}

type raw string

func (r raw) expand(_ *state) (string, error) {
	return string(r), nil
}

type placeholder struct {
}

func (p placeholder) expand(_ *state) (string, error) {
	return "", nil
}

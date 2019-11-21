package dissect

import (
	"fmt"
	"strconv"
)

func merge(dat, root Block) (Node, error) {
	var nodes []Node
	for _, n := range dat.nodes {
		switch n := n.(type) {
		case Parameter:
			nodes = append(nodes, n)
		case Let:
			nodes = append(nodes, n)
		case Del:
			nodes = append(nodes, n)
		case Seek:
			nodes = append(nodes, n)
		case Reference:
			p, err := root.ResolveParameter(n.id.Literal)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, p)
		case Repeat:
			r, err := mergeRepeat(n, root)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, r)
		case Match:
			m, err := mergeMatch(n, root)
			if err != nil {
				return nil, err
			}
			nodes = append(nodes, m)
		case Include:
			b, err := mergeInclude(n, root)
			if err != nil {
				return nil, err
			}
			if x, ok := b.(Block); ok {
				nodes = append(nodes, x.nodes...)
			} else {
				nodes = append(nodes, b)
			}
		default:
			return nil, fmt.Errorf("unexpected node type %T", n)
		}
	}
	dat = Block{
		id:    dat.id,
		nodes: nodes,
	}
	return dat, nil
}

func mergeRepeat(r Repeat, root Block) (Node, error) {
	if r.repeat.isIdent() {
		return r, nil
	}
	var repeat int64
	switch r.repeat.Type {
	case Integer:
		repeat, _ = strconv.ParseInt(r.repeat.Literal, 0, 64)
	case Float:
		f, _ := strconv.ParseFloat(r.repeat.Literal, 64)
		repeat = int64(f)
	default:
		return nil, fmt.Errorf("unexpected token %s", TokenString(r.repeat))
	}
	var (
		dat Block
		err error
	)
	switch n := r.node.(type) {
	case Reference:
		dat, err = root.ResolveBlock(n.id.Literal)
	case Block:
		dat = n
	}
	if err != nil {
		return nil, err
	}
	node, err := merge(dat, root)
	if err != nil {
		return nil, err
	}
	d, ok := node.(Block)
	if !ok {
		return nil, fmt.Errorf("unexpected node type %s", node)
	}
	dat = d

	tok := Token{
		Type:    Keyword,
		Literal: kwInline,
		pos:     r.Pos(),
	}
	pat := Block{id: tok}
	for i := int64(0); i < repeat; i++ {
		pat.nodes = append(pat.nodes, dat.nodes...)
	}
	return pat, nil
}

func mergeMatch(m Match, root Block) (Node, error) {
	var nodes []MatchCase
	for _, c := range m.nodes {
		var (
			err error
			dat Block
		)
		switch n := c.node.(type) {
		case Reference:
			dat, err = root.ResolveBlock(n.id.Literal)
		case Block:
			dat = n
		default:
			err = fmt.Errorf("unexpected node type %T", n)
		}
		if err != nil {
			return nil, err
		}
		c.node, err = merge(dat, root)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, c)
	}
	m.nodes = nodes
	return m, nil
}

func mergeInclude(i Include, root Block) (Node, error) {
	var (
		err error
		dat Block
	)
	switch n := i.node.(type) {
	case Reference:
		dat, err = root.ResolveBlock(n.id.Literal)
		if err != nil {
			return nil, err
		}
	case Block:
		dat = n
	}
	if i.node, err = merge(dat, root); err != nil {
		return nil, err
	}
	if i.Predicate == nil {
		return i.node, err
	}
	return i, err
}

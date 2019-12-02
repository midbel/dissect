package dissect

import (
	"fmt"
	"io"
)

func Merge(r io.Reader) (Node, error) {
	n, err := Parse(r)
	if err != nil {
		return nil, err
	}
	root, ok := n.(Block)
	if !ok {
		return nil, fmt.Errorf("root node is not a block")
	}
	dat, err := root.ResolveData()
	if err != nil {
		return nil, err
	}
	bck, err := mergeBlock(dat.Block, root)
	if err == nil {
		dat.Block = bck.(Block)
	}
	return dat, err
}

func mergeBlock(dat, root Block) (Node, error) {
	nodes := make([]Node, 0, len(dat.nodes))
	for _, n := range dat.nodes {
		var (
			nx  Node
			err error
		)
		switch x := n.(type) {
		default:
			nx = n
		case Block:
			nx, err = mergeBlock(x, root)
		case Parameter:
			nx, err = mergeParameter(x, root)
		case Include:
			nx, err = mergeInclude(x, root)
		case Repeat:
			nx, err = mergeRepeat(x, root)
		case Match:
			nx, err = mergeMatch(x, root)
		case If:
			nx, err = mergeIf(x, root)
		case Reference:
			p, e := root.ResolveParameter(x.id.Literal)
			if e == nil {
				nx, err = mergeParameter(p, root)
			} else {
				err = e
			}
		}
		if err != nil {
			return nil, err
		}
		if nx == nil {
			continue
		}
		nodes = append(nodes, nx)
	}
	dat.nodes = nodes
	return dat, nil
}

func mergeParameter(p Parameter, root Block) (Node, error) {
	tok, ok := p.apply.(Token)
	if !ok {
		return p, nil
	}
	pair, err := root.ResolvePair(tok.Literal)
	if err == nil {
		p.apply = pair
	}
	return p, err
}

func mergeIf(i If, root Block) (Node, error) {
	var err error
	if i.csq != nil {
		i.csq, err = mergeNode(i.csq, root)
	}
	if i.alt != nil {
		i.alt, err = mergeNode(i.alt, root)
	}
	return i, err
}

func mergeInclude(i Include, root Block) (Node, error) {
	node, err := mergeNode(i.node, root)
	if err != nil {
		return nil, err
	}
	i.node = node

	if i.cond == nil {
		return i.node, nil
	}
	return i, nil
}

func mergeRepeat(r Repeat, root Block) (Node, error) {
	node, err := mergeNode(r.node, root)
	if err == nil {
		r.node = node
	}
	return r, err
}

func mergeMatch(m Match, root Block) (Node, error) {
	for i, c := range m.nodes {
		node, err := mergeNode(c.node, root)
		if err != nil {
			return nil, err
		}
		m.nodes[i].node = node
	}
	if m.alt.node != nil {
		node, err := mergeNode(m.alt.node, root)
		if err != nil {
			return nil, err
		}
		m.alt.node = node
	}
	return m, nil
}

func mergeNode(node Node, root Block) (Node, error) {
	var dat Block
	switch n := node.(type) {
	case Block:
		dat = n
	case Reference:
		b, err := root.ResolveBlock(n.id.Literal)
		if err != nil {
			return nil, err
		}
		dat = b
		if n.alias.Pos().IsValid() {
			dat.id = n.alias
		}
	}
	return mergeBlock(dat, root)
}

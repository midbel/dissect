package dissect

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

func Dump(n Node) error {
	return dumpNode(n, 0)
}

func DumpReader(r io.Reader) error {
	n, err := Parse(r)
	if err != nil {
		return err
	}
	return Dump(n)
}

func dumpNode(n Node, level int) error {
	indent := strings.Repeat(" ", level*2)
	switch n := n.(type) {
	case Block:
		fmt.Printf("%s%s(pos=%s, type=%s)(\n", indent, n.String(), n.Pos(), n.blockName())
		for _, n := range sortNodes(n.nodes) {
			dumpNode(n, level+1)
		}
		fmt.Printf("%s)", indent)
	case Pair:
		fmt.Printf("%s%s(name=%s, pos=%s)\n", indent, n.kind.Literal, n.id.Literal, n.Pos())
		for _, n := range sortNodes(n.nodes) {
			dumpNode(n, level+1)
		}
		fmt.Printf("%s)", indent)
	case Include:
		fmt.Printf("%sinclude(predicate=???, pos=%s)(\n", indent, n.Pos())
		dumpNode(n.node, level+1)
		fmt.Printf("%s)", indent)
	case Reference:
		fmt.Printf("%sreference(name=%s, pos=%s)", indent, n.id.Literal, n.Pos())
	case Parameter:
		fmt.Printf("%sparameter(name=%s, pos=%s)", indent, n.id.Literal, n.Pos())
		if len(n.props) > 0 {
			fmt.Println("[")
			ni := indent + strings.Repeat(" ", level*2)
			for k, v := range n.props {
				fmt.Printf("%sproperty(name=%s, value=%s)\n", ni, k.Literal, v.Literal)
			}
			fmt.Print(indent + "]")
		}
	case Constant:
		fmt.Printf("%sconstant(name=%s, value=%s, pos=%s)", indent, n.id.Literal, n.value.Literal, n.Pos())
	default:
		return fmt.Errorf("unexpected node type: %T", n)
	}
	fmt.Println()
	return nil
}

func sortNodes(nodes []Node) []Node {
	ns := make([]Node, len(nodes))
	copy(ns, nodes)

	sort.Slice(nodes, func(i, j int) bool {
		pi, pj := nodes[i].Pos(), nodes[j].Pos()
		return pi.Line < pj.Line
	})

	return ns
}

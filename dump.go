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
		fmt.Printf("%sblock(name=%s, type=%s, pos=%s) (\n", indent, n.String(), n.blockName(), n.Pos())
		for _, n := range n.nodes {
			dumpNode(n, level+1)
		}
		fmt.Printf("%s)", indent)
	case Pair:
		fmt.Printf("%s%s(name=%s, pos=%s) (\n", indent, n.kind.Literal, n.id.Literal, n.Pos())
		for _, n := range n.nodes {
			dumpNode(n, level+1)
		}
		fmt.Printf("%s)", indent)
	case ExitStmt:
		fmt.Printf("%sexit(code=%s, pos=%s)", indent, n.code.Literal, n.Pos())
	case LetStmt:
		fmt.Printf("%slet(name=%s, predicate=%s, pos=%s)", indent, n.id.Literal, n.expr, n.Pos())
	case DelStmt:
		fmt.Printf("%sdel(pos=%s) (\n", indent, n.Pos())
		for _, n := range n.nodes {
			dumpNode(n, level+1)
		}
		fmt.Printf("%s)", indent)
	case SeekStmt:
		fmt.Printf("%sseek(offset=%s, pos=%s)", indent, n.offset.Literal, n.Pos())
	case Match:
		fmt.Printf("%smatch(name=%s, pos=%s) (\n", indent, n.id.Literal, n.Pos())
		for _, n := range n.nodes {
			dumpNode(n, level+1)
		}
		fmt.Printf("%s)", indent)
	case MatchCase:
		fmt.Printf("%scase(cond=%s) (\n", indent, n.cond.Literal)
		dumpNode(n.node, level+1)
		fmt.Printf("%s)", indent)
	case Repeat:
		fmt.Printf("%srepeat(repeat=%s, pos=%s) (\n", indent, n.repeat.Literal, n.Pos())
		dumpNode(n.node, level+1)
		fmt.Printf("%s)", indent)
	case Include:
		predicate := kwTrue
		if n.Predicate != nil {
			predicate = n.Predicate.String()
		}
		fmt.Printf("%sinclude(predicate=%s, pos=%s) (\n", indent, predicate, n.Pos())
		dumpNode(n.node, level+1)
		fmt.Printf("%s)", indent)
	case Reference:
		fmt.Printf("%sreference(name=%s, pos=%s)", indent, n.id.Literal, n.Pos())
	case Parameter:
		fmt.Printf("%sparameter(name=%s, type=%s, size=%s, pos=%s)", indent, n.id.Literal, n.kind.Literal, n.size.Literal, n.Pos())
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

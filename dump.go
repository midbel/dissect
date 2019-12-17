package dissect

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

func Stat(r io.Reader) error {
	n, err := Parse(r)
	if err != nil {
		return err
	}
	block, ok := n.(Block)
	if !ok {
		return nil
	}
	for _, n := range block.nodes {
		block, ok := n.(Block)
		if !ok {
			continue
		}
		var (
			size int64
			count int
		)
		for _, n := range block.nodes {
			p, ok := n.(Parameter)
			if !ok {
				continue
			}
			z, _ := strconv.ParseInt(p.size.Literal, 0, 64)
			switch p.is() {
			case kindInt, kindUint, kindFloat:
			case kindString, kindBytes:
				z *= numbit
			default:
				continue
			}
			size += z
			count++
		}
		fmt.Printf("%16s: %5d bits, %5d bytes, %3d parameters\n", block.id, size, size/numbit, count)
	}
	return nil
}

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
	case Token:
		fmt.Printf("%stoken(literal=%s, pos=%s)", indent, n.Literal, n.Pos())
	case Print:
		expr := "???"
		if n.expr != nil {
			expr = n.expr.String()
		}
		fmt.Printf("%sprint(file=%s, format=%s, method=%s, expr=%s, pos=%s)", indent, n.file, n.format, n.method, expr, n.Pos())
		if len(n.values) > 0 {
			fmt.Println(" (")
			for _, n := range n.values {
				dumpNode(n, level+1)
			}
			fmt.Printf("%s)", indent)
		}
	case Echo:
		fmt.Printf("%secho(string=%s, pos=%s)", indent, n, n.Pos())
	case Data:
		fs := make([]string, len(n.files))
		for i := 0; i < len(n.files); i++ {
			fs[i] = n.files[i].Literal
		}
		fmt.Printf("%sdata(files=%s, pos=%s) (\n", indent, strings.Join(fs, ", "), n.Pos())
		dumpNode(n.Block, level+1)
		fmt.Printf("%s)", indent)
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
	case Exit:
		fmt.Printf("%sexit(code=%s, pos=%s)", indent, n.code.Literal, n.Pos())
	case Let:
		fmt.Printf("%slet(name=%s, predicate=%s, pos=%s)", indent, n.id.Literal, n.expr, n.Pos())
	case Del:
		fmt.Printf("%sdel(pos=%s) (\n", indent, n.Pos())
		for _, n := range n.nodes {
			dumpNode(n, level+1)
		}
		fmt.Printf("%s)", indent)
	case Seek:
		fmt.Printf("%sseek(offset=%s, pos=%s)", indent, n.offset, n.Pos())
	case Peek:
		fmt.Printf("%speek(count=%s, pos=%s)", indent, n.count, n.Pos())
	case If:
		fmt.Printf("%sif(expr=%s, pos=%s)", indent, n.expr, n.Pos())
		if n.csq != nil {
			fmt.Print(" (\n")
			dumpNode(n.csq, level+1)
			fmt.Printf("%s)", indent)
		}
		if n.alt != nil {
			fmt.Print(" else (\n")
			dumpNode(n.alt, level+1)
			fmt.Printf("%s)", indent)
		}
	case Match:
		expr := "???"
		if n.expr != nil {
			expr = n.expr.String()
		}
		fmt.Printf("%smatch(expr=%s, pos=%s) (\n", indent, expr, n.Pos())
		for _, n := range n.nodes {
			dumpNode(n, level+1)
		}
		if n.alt.Pos().IsValid() {
			dumpNode(n.alt, level+1)
		}
		fmt.Printf("%s)", indent)
	case MatchCase:
		expr := "default"
		if n.cond != nil {
			expr = n.cond.String()
		}
		fmt.Printf("%scase(cond=%s) (\n", indent, expr)
		dumpNode(n.node, level+1)
		fmt.Printf("%s)", indent)
	case Repeat:
		fmt.Printf("%srepeat(repeat=%s, pos=%s) (\n", indent, n.repeat, n.Pos())
		dumpNode(n.node, level+1)
		fmt.Printf("%s)", indent)
	case Break:
		predicate := kwTrue
		if n.expr != nil {
			predicate = n.expr.String()
		}
		fmt.Printf("%sbreak(predicate=%s, pos=%s)", indent, predicate, n.Pos())
	case Continue:
		predicate := kwTrue
		if n.expr != nil {
			predicate = n.expr.String()
		}
		fmt.Printf("%scontinue(predicate=%s, pos=%s)", indent, predicate, n.Pos())
	case Include:
		predicate := kwTrue
		if n.cond != nil {
			predicate = n.cond.String()
		}
		fmt.Printf("%sinclude(predicate=%s, pos=%s) (\n", indent, predicate, n.Pos())
		dumpNode(n.node, level+1)
		fmt.Printf("%s)", indent)
	case Reference:
		fmt.Printf("%sreference(name=%s, alias=%s, pos=%s)", indent, n.alias, n.id, n.Pos())
	case Parameter:
		fmt.Printf("%sparameter(name=%s, type=%s, size=%s, pos=%s)", indent, n.id.Literal, n.kind.Literal, n.size.Literal, n.Pos())
		if p, ok := n.apply.(Pair); ok {
			fmt.Print(" (\n")
			dumpNode(p, level+1)
			fmt.Printf("%s)", indent)
		}
	case Constant:
		fmt.Printf("%sconstant(name=%s, value=%s, pos=%s)", indent, n.id.Literal, n.value, n.Pos())
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

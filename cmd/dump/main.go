package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/dissect"
)

func main() {
	merge := flag.Bool("m", false, "merge")
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(21)
	}
	defer r.Close()

	var n dissect.Node
	if *merge {
		n, err = dissect.Merge(r)
	} else {
		n, err = dissect.Parse(r)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(25)
	}
	if err = dissect.Dump(n); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(23)
	}
}

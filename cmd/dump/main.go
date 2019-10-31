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

	n, err := dissect.Parse(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(23)
	}
	if *merge {
		n, err = dissect.Merge(n)
		if err != nil {
			fmt.Fprintln(os.Stderr)
			os.Exit(25)
		}
	}
	err = dissect.Dump(n)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(23)
	}
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/dissect"
)

func main() {
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
		os.Exit(25)
	}
	if err = dissect.Dump(n); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(23)
	}
}

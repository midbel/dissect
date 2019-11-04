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
		os.Exit(2)
	}
	defer r.Close()

	d, err := dissect.NewDecoder(r)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(3)
	}
	d.Dump()
}

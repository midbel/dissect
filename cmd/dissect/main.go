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
		os.Exit(1)
	}
	defer r.Close()

	var files []string
	for i := 1; i < flag.NArg(); i++ {
		files = append(files, flag.Arg(i))
	}
	if err := dissect.Dissect(r, files); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
}

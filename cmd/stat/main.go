package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/midbel/dissect"
)

func main() {
	flag.Parse()
	for _, a := range flag.Args() {
		if err := stat(a); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}
}

func stat(file string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()
	return dissect.Stat(r)
}

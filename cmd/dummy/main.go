package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/midbel/dissect"
)

func main() {
	lex := flag.Bool("x", false, "lex")
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(21)
	}
	defer r.Close()

	if *lex {
		err = scanFile(r)
	} else {
		err = parseFile(r)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(23)
	}
}

func scanFile(r io.Reader) error {
	s, err := dissect.Scan(r)
	if err != nil {
		return err
	}
	for tok := s.Scan(); tok.Type != dissect.EOF; tok = s.Scan() {
		fmt.Printf("<%s> %s\n", tok.Pos(), tok)
	}
	return nil
}

func parseFile(r io.Reader) error {
	_, err := dissect.Parse(r)
	return err
}

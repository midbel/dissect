package dissect

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/midbel/glob"
)

func Dissect(script io.Reader, r io.Reader) error {
	node, err := Merge(script)
	if err != nil {
		return err
	}
	data, ok := node.(Data)
	if !ok {
		return fmt.Errorf("missing data block")
	}
	if err != nil {
		return err
	}
	s := state{
		data:   data.Block,
		files:  make(map[string]*os.File),
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
	defer s.Close()
	if err = s.decodeNodes([]Node{data.pre}); err != nil {
		return err
	}
	err = s.Run(r)
	if err == nil {
		err = s.decodeNodes([]Node{data.post})
	}
	return err
}

func DissectFiles(script io.Reader, fs []string) error {
	node, err := Merge(script)
	if err != nil {
		return err
	}
	data, ok := node.(Data)
	if !ok {
		return fmt.Errorf("missing data block")
	}
	if err != nil {
		return err
	}
	var files []string
	if len(data.files) > 0 {
		for _, f := range data.files {
			files = append(files, f.Literal)
		}
	} else {
		files = fs
	}
	s := state{
		data:   data.Block,
		files:  make(map[string]*os.File),
		stdout: os.Stdout,
		stderr: os.Stderr,
	}
	defer s.Close()

	if err = s.decodeNodes([]Node{data.pre}); err != nil {
		return err
	}
	for f := range walkFiles(files) {
		r, err := os.Open(f)
		if err != nil {
			continue
		}
		err = s.Run(r)
		r.Close()
		if err != nil {
			return err
		}
	}
	return s.decodeNodes([]Node{data.post})
}

func checkExit(err error) error {
	var exit *ExitError
	if err != nil && errors.As(err, &exit) {
		if exit.code == 0 {
			err = nil
		}
	}
	if err != nil && !errors.Is(err, ErrDone) {
		return err
	}
	return nil
}

func walkFiles(files []string) <-chan string {
	if len(files) == 0 {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			f := s.Text()
			if len(f) == 0 {
				continue
			}
			files = append(files, f)
		}
	}
	queue := make(chan string)
	go func() {
		defer close(queue)
		for _, f := range files {
			i, err := os.Stat(f)
			if err != nil {
				globFiles(f, queue)
				continue
			}
			if i.IsDir() {
				filepath.Walk(f, func(p string, i os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if i.Mode().IsRegular() {
						queue <- p
					}
					return nil
				})
				continue
			}
			queue <- f
		}
	}()
	return queue
}

func globFiles(f string, queue chan<- string) {
	g, err := glob.New("", f)
	if err != nil {
		return
	}
	for n := g.Glob(); n != ""; n = g.Glob() {
		i, err := os.Stat(n)
		if err == nil && i.Mode().IsRegular() {
			queue <- n
		}
	}
}

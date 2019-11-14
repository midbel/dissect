package dissect

import (
	"bufio"
	"io"
	"io/ioutil"
	"os"
)

func Dissect(script io.Reader, fs []string) error {
	n, err := Parse(script)
	if err != nil {
		return err
	}
	root := n.(Block)
	data, err := root.ResolveData()
	if err != nil {
		return err
	}
	var files []string
	// if len(data.files) > 0 {
	// 	for _, f := range data.files {
	// 		files = append(files, f.Literal)
	// 	}
	// } else {
	// 	files = fs
	// }
	s := state{
		Block: root,
		files: make(map[string]*os.File),
	}
	defer s.Close()

	for f := range walkFiles(files) {
		buf, err := ioutil.ReadFile(f)
		if err != nil {
			continue
		}
		if err := s.Run(data.Block, buf); err != nil {
			return err
		}
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
				continue
			}
			if i.IsDir() {
				continue
			}
			queue <- f
		}
	}()
	return queue
}

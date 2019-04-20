package main

import (
	"bytes"
	. "github.com/RokyErickson/goshworker"
	"io"
	"os"
	"sync"
)

func main() {

	m := make(map[string]string)
	p := NewPool(3, "cat", []string{"-"}, m, "/home/")
	p.Start()
	procErr := make(chan error)
	var wg sync.WaitGroup
	for i := 1; i <= 200; i++ {
		wg.Add(1)
		go func(i int) {
			err := p.Run(func(in io.Writer, out, err io.Reader) error {

				var buf bytes.Buffer

				buf.WriteString("hello World")

				go func() {
					io.Copy(in, &buf)
					io.Copy(os.Stdout, out)
					io.Copy(os.Stderr, err)
				}()

				return nil
			})
			wg.Done()
			procErr <- err
		}(i)
	}
	wg.Wait()
	select {
	default:
	case err := <-procErr:
		if err != nil {
			panic("oh no")
		}
	}
	p.Stop()
}

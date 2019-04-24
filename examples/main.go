package main

import (
	"bytes"
	"io"
	"os"
	"sync"

	"github.com/RokyErickson/gosh"
	. "github.com/RokyErickson/goshworker"
)

func main() {
	p := Newpool(7, gosh.Opts{Args: []string{"cat", "-"}})
	procErr := make(chan error)
	var wg sync.WaitGroup
	for i := 0; i < 200; i++ {
		wg.Add(1)
		go func(i int) {
			err := p.Submit(Task1)
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
	p.StopWait()
}

func Task1(in io.Writer, out, err io.Reader) error {
	var buf bytes.Buffer
	buf.WriteString("hello World")
	go func() {
		io.Copy(in, &buf)
		io.Copy(os.Stdout, out)
		io.Copy(os.Stderr, err)
	}()
	return nil
}

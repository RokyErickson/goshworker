package main

import (
	"bytes"
	"io"
	"os"

	. "github.com/RokyErickson/goshworker"
)

func main() {
	NewPoolGlobal(200, []string{"cat", "-"})
	p := NewGoshPool(1)
	q := NewGoshPool(1)

	for i := 0; i < 200; i++ {
		p.Submit(Task1)
		q.Submit(Task1)
	}
	p.StopWait()
	EndPoolGlobal()
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

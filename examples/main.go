package main

import (
	"bytes"
	"io"
	"os"

	. "github.com/RokyErickson/goshworker"
)

func main() {
	NewPoolGlobal(4, []string{"cat", "-"})
	p := NewGoshPool(4)

	for i := 0; i < 200; i++ {
		p.Submit(Task1)
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

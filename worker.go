// Package goshworker implements a process worker
// and process worker pool using a modified verison of the gosh exec API.
package goshworker

import (
	"errors"
	"io"
	"sync/atomic"
)

// Worker represents a worker process that can perform a task.
type Worker interface {
	Submit(Task) error
	Run(Task) error
	Stop()
	StopWait()
}

// A Task provides access to the I/O of a worker process.
type Task func(in io.Writer, out, err io.Reader) error

// Todo! Implement Goshtasks
// type GoshTask func(in gosh.Command, out, err io.Reader) error

type Work struct {
	task    Task
	errchan chan error
}

type common struct {
	state     int32 // atomic
	TaskQueue chan *Work
}

const (
	stateZero int32 = iota
	stateStart
	stateStop
)

func (c *common) start() {
	if atomic.LoadInt32(&c.state) != stateZero {
		panic("start called twice")
	}
	atomic.StoreInt32(&c.state, stateStart)
	c.TaskQueue = make(chan *Work, 1)
}

func (c *common) stopped() {
	if atomic.LoadInt32(&c.state) == stateStop {
		return
	}
	atomic.StoreInt32(&c.state, stateStop)
	close(c.TaskQueue)
}

// Run runs a task in the context of a worker process.
func (c *common) Run(t Task) error {
	if atomic.LoadInt32(&c.state) != stateStart {
		return errors.New("worker: not running")
	}
	w := &Work{t, make(chan error, 1)}
	c.TaskQueue <- w
	e := <-w.errchan
	return e
}

func (c *common) Submit(t Task) error {
	if t != nil {
		w := &Work{t, make(chan error, 1)}
		c.TaskQueue <- w
	}
	return nil
}

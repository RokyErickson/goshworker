// Package goshworker implements a process worker
// and process worker pool using a modified verison of the gosh exec API.
package goshworker

//Todo! Implement recycling workers. Or high performance pool like ants.

import (
	"errors"
	"github.com/RokyErickson/gosh"
	"github.com/eapache/channels"
	"io"
	"sync"
	"sync/atomic"
)

// Worker represents a worker process that
// can perform a task.
type Worker interface {
	// Start starts the worker process.
	Start()

	// Run runs a task in the worker process.
	// It blocks until the task is completed.
	Run(Task) error

	// Stop stops the worker process.
	Stop()
}

// A Task provides access to the I/O of a worker process.
type Task func(in io.Writer, out, err io.Reader) error

// Todo! Implement Goshtasks
type GoshTask func(in gosh.Command, out, err io.Reader) error

type work struct {
	task    Task
	errchan channels.Channel
}

type common struct {
	tasks channels.Channel
	state int32 // atomic
	pause sync.WaitGroup
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
	c.tasks = channels.NewNativeChannel(0)
}

func (c *common) stop() {
	if atomic.LoadInt32(&c.state) == stateStop {
		panic("stop called twice")
	}
	atomic.StoreInt32(&c.state, stateStop)
	c.tasks.Close()
}

// Run runs a task in the context of a worker process.
// If the task returns an error, it will be returned
// from Run to the caller. Run blocks till the task is
// executed.
func (c *common) Run(t Task) error {
	if atomic.LoadInt32(&c.state) != stateStart {
		return errors.New("worker: not running")
	}
	c.pause.Wait()
	w := work{t, channels.NewNativeChannel(1)}
	c.tasks.In() <- w
	e := <-w.errchan.Out()

	if e != nil {
		ERR := e.(error)
		return ERR
	}
	return nil
}

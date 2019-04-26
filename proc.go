package goshworker

import (
	. "github.com/RokyErickson/gosh"
	"github.com/RokyErickson/gosh/iox"
)

// proc represents a worker process.
type proc struct {
	process  Proc
	cmd      Command
	in       *iox.PipeWriter
	out      *iox.PipeReader
	err      *iox.PipeReader
	isActive bool
}

//Makes a new Proc
func newProc(opts []string) *proc {

	buf1 := iox.New(24 * 1024)
	buf2 := iox.New(24 * 1024)
	buf3 := iox.New(24 * 1024)

	r1, w1 := iox.Pipe(buf1)
	r2, w2 := iox.Pipe(buf2)
	r3, w3 := iox.Pipe(buf3)

	cmd := Gosh(Opts{
		Args: opts,
		In:   r1,
		Out:  w2,
		Err:  w3,
	}).Bake()

	return &proc{cmd: cmd, in: w1, out: r2, err: r3}
}

// Start starts the worker process.
func (p *proc) Start() {
	p.process = p.cmd.Start()
}

func (p *proc) Recycle() {
	p.isActive = false
	// Return it back to the global worker pool
	PoolGlobal <- p
}

var testProcRunHook func(*proc)

// Run starts processing tasks from the channel and
// blocks until the tasks channel is closed or the Gosh
// commands completes and it's waitchan closes
func (p *proc) Run(StartReady, RoutinePool chan chan *Work) {
	go func() {
		defer p.process.Wait()
		Tasks := make(chan *Work)
		StartReady <- Tasks
		for {
			select {
			case <-p.process.WaitChan():
				return
			case w, ok := <-Tasks:
				if !ok {
					return
				}
				if testProcRunHook != nil {
					testProcRunHook(p)
				}
				w.errchan <- w.task(p.in, p.out, p.err)
				RoutinePool <- Tasks
			}
		}
	}()
}

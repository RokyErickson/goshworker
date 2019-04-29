package goshworker

import (
	"github.com/RokyErickson/channels"
	. "github.com/RokyErickson/gosh"
	"github.com/RokyErickson/gosh/iox"
)

// goshworker represents a worker goshworkeress.
type goshworker struct {
	process  Proc
	cmd      Command
	in       *iox.PipeWriter
	out      *iox.PipeReader
	err      *iox.PipeReader
	isActive bool
}

//Makes a new Goshworker
func newGoshworker(opts []string) *goshworker {

	buf1 := iox.New(24 * 1024)
	buf2 := iox.New(24 * 1024)
	buf3 := iox.New(24 * 1024)

	r1, w1 := iox.Pipe(buf1)
	r2, w2 := iox.Pipe(buf2)
	r3, w3 := iox.Pipe(buf3)

	cmd := Gosh(Opts{Args: opts, In: r1, Out: w2, Err: w3}).Bake()

	return &goshworker{cmd: cmd, in: w1, out: r2, err: r3}
}

// Start starts the worker goshworkeress.
func (p *goshworker) Start() {
	p.process = p.cmd.Start()
}

func (p *goshworker) Recycle() {
	p.isActive = false
	// Return it back to the global worker pool
	GoshPoolGlobal <- p
}

var testGoshworkerRunHook func(*goshworker)

// Run starts goshworkeressing tasks from the channel and
// blocks until the tasks channel is closed or the Gosh
// commands completes and it's waitchan closes
func (p *goshworker) Run(StartReady, RoutinePool chan channels.Channel) {
	go func() {
		defer p.process.Wait()
		Tasks := channels.NewNativeChannel(0)
		StartReady <- Tasks
		for {
			select {
			case <-p.process.WaitChan():
				return
			case out, ok := <-Tasks.Out():
				w := out.(*Work)
				if !ok {
					return
				}
				if testGoshworkerRunHook != nil {
					testGoshworkerRunHook(p)
				}
				w.errchan <- w.task(p.in, p.out, p.err)
				RoutinePool <- Tasks
			}
		}
	}()
}

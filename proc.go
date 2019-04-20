package goshworker

import (
	"github.com/RokyErickson/gosh"
	"github.com/RokyErickson/gosh/iox"
	"github.com/eapache/channels"
)

// proc represents a worker process.
type proc struct {
	process gosh.Proc
	cmd     gosh.Command
	in      *iox.PipeWriter
	out     *iox.PipeReader
	err     *iox.PipeReader
	wait    channels.Channel
}

//Makes a new Proc
func newProc(path string, args []string, env map[string]string, dir string) *proc {

	buf1 := iox.New(24 * 1024)
	buf2 := iox.New(24 * 1024)
	buf3 := iox.New(24 * 1024)

	r1, w1 := iox.Pipe(buf1)
	r2, w2 := iox.Pipe(buf2)
	r3, w3 := iox.Pipe(buf3)

	cmd := gosh.Gosh(path, gosh.Opts{
		Args: args,
		Env:  env,
		Cwd:  dir,
		In:   r1,
		Out:  w2,
		Err:  w3,
	}).Bake()

	return &proc{
		cmd:  cmd,
		in:   w1,
		out:  r2,
		err:  r3,
		wait: channels.NewNativeChannel(0),
	}
}

// Start starts the worker process.
func (p *proc) Start() {
	p.process = p.cmd.Start()
}

var testProcRunHook func(*proc) // for testing

// Run starts processing tasks from the channel and
// blocks until the tasks channel is closed or the Gosh
// commands completes and it's waitchan closes
func (p *proc) Run(tasks channels.Channel) {
	defer p.exit()
	for {
		select {
		case <-p.process.WaitChan():
			return
		case w, ok := <-tasks.Out():
			erch := w.(work)
			if !ok {
				return
			}
			if testProcRunHook != nil {
				testProcRunHook(p)
			}
			erch.errchan.In() <- erch.task(p.in, p.out, p.err)
		}
	}
}

var testProcExitHook func(*proc) // for testing

func (p *proc) exit() {
	if testProcExitHook != nil {
		testProcExitHook(p)
	}
	p.process.Wait()
}

// Proc represents a worker process.
type Proc struct {
	// Path is the path of the command to run.
	//
	// This is the only field that must be set to a non-zero
	// value. If Path is relative, it is evaluated relative
	// to Dir.
	Path string

	// Args holds command line arguments.
	Args []string

	// Env specifies the environment of the process. Each
	// entry is of the form "key=value". If Env is nil,
	// the new process uses the current process's
	// environment. If Env contains duplicate environment
	// keys, only the last value in the slice for each
	// duplicate key is used.
	Env map[string]string

	// Dir specifies the working directory of the command.
	// If Dir is the empty string, the command runs in the
	// calling process's current directory.
	Dir string

	common
	proc *proc
}

var _ Worker = (*Proc)(nil)

// Start starts the worker process.
func (p *Proc) Start() {
	p.start()
	p.proc = newProc(p.Path, p.Args, p.Env, p.Dir)
	p.proc.Start()
	go p.proc.Run(p.tasks)
}

// Stop stops the worker.
func (p *Proc) Stop() {
	p.stop()
}

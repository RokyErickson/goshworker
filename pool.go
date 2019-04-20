package goshworker

import (
	"fmt"
)

// Pool is a pool of worker processes.
type Pool struct {
	// Size specifies the initial size of the pool.
	// Call CurrentSize() to get the actual size.
	Size int

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
	//Env []string
	Env map[string]string

	// Dir specifies the working directory of the command.
	// If Dir is the empty string, the command runs in the
	// calling process's current directory.
	Dir string

	common
	procs []*proc

	// procsMu protects the procs slice, not the procs
	// themselves. If a pipe is closed, procsMu must be
	// locked as well to prevent a race between closing
	// the pipe and growing the procs.
	//procsMu sync.Mutex
}

var _ Worker = (*Pool)(nil)

func assertSize(size int) {
	if size <= 0 {
		panic(fmt.Sprintf("pool size is invalid (%d)", size))
	}
}

// Start starts the workers in the pool. It returns
// an error if a worker cannot be started.
func (p *Pool) Start() {
	assertSize(p.Size)
	p.start()
	p.procs = make([]*proc, p.Size)
	for i := 0; i < p.Size; i++ {
		w := newProc(p.Path, p.Args, p.Env, p.Dir)
		w.Start()
		go w.Run(p.tasks)
		p.procs[i] = w
	}
}

// Stop stops the workers in the pool and returns if
// there is any error. Only the first worker error
// encountered is returned.
func (p *Pool) Stop() {
	p.stop()
}

//Creates new pool.
func NewPool(size int, path string, args []string, env map[string]string, dir string) *Pool {

	return &Pool{
		Size: size,
		Path: path,
		Args: args,
		Env:  env,
		Dir:  dir,
	}
}

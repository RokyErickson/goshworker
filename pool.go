package goshworker

import (
	"fmt"
	"sync"
	"time"

	"github.com/gammazero/deque"

	. "github.com/RokyErickson/gosh"
)

// Pool is a pool of worker processes.

const (
	RoutinePoolQueueSize = 16

	idleTimeout = 5
)

var Tasks chan *Work

type Pool struct {
	// Size specifies the initial size of the pool.
	Size int
	// Gosh Opts which will take commands args.
	Opts    Opts
	timeout time.Duration
	common
	procs        []*proc
	RoutinePool  chan chan *Work
	StartReady   chan chan *Work
	waitingQueue deque.Deque
	stopMutex    sync.Mutex
}

var _ Worker = (*Pool)(nil)

func assertSize(size int) {
	if size <= 0 {
		panic(fmt.Sprintf("pool size is invalid (%d)", size))
	}
}

//New Pool!
func Newpool(size int, opts Opts) *Pool {

	pool := &Pool{
		Size:        size,
		Opts:        opts,
		timeout:     time.Second * idleTimeout,
		RoutinePool: make(chan chan *Work, RoutinePoolQueueSize),
		StartReady:  make(chan chan *Work),
	}
	assertSize(pool.Size)
	pool.start()
	go pool.send()

	return pool
}

//sends tasks
func (p *Pool) send() {
	timeout := time.NewTimer(p.timeout)
	var (
		workerCount int
		work        *Work
		ok, wait    bool
	)
Cycle:

	for {
		//incoming tasks directly to worker
		if p.waitingQueue.Len() != 0 {
			select {
			case work, ok = <-p.TaskQueue:
				if !ok {
					break Cycle
				}
				if work == nil {
					wait = true
					break Cycle
				}
				p.waitingQueue.PushBack(work)
			case Tasks = <-p.RoutinePool:
				//worker ready, make him work!
				Tasks <- p.waitingQueue.PopFront().(*Work)
			}
			continue
		}
		//working queue empty
		timeout.Reset(p.timeout)
		select {
		case work, ok = <-p.TaskQueue:
			if !ok || work == nil {
				break Cycle
			}
			//got work to do.
			select {
			case Tasks = <-p.RoutinePool:
				//give work
				Tasks <- work
			default:
				//proc not ready make new proc.
				if workerCount < p.Size {
					workerCount++
					w := newProc(p.StartReady, p.RoutinePool, p.Opts)
					p.procs = append(p.procs, w)
					w.Start()
					go func(work *Work) {
						w.Run()
						//Run Worker
						Tasks := <-p.StartReady
						Tasks <- work
					}(work)
				} else {
					//queued task to be executed by worker
					p.waitingQueue.PushBack(work)
				}
			}
		case <-timeout.C:
			//timeout waiting for work
			if workerCount > 0 {
				select {
				case Tasks = <-p.RoutinePool:
					//Kill routine pool proc.
					close(Tasks)
					workerCount--
				default:
					//No work, but all procs busy.
				}
			}
		}
	}
	//If set to wait, push all work to queue.
	if wait {
		for p.waitingQueue.Len() != 0 {
			Tasks = <-p.RoutinePool
			Tasks <- p.waitingQueue.PopFront().(*Work)
		}
	}

	//stop all procs
	for workerCount > 0 {
		Tasks = <-p.RoutinePool
		close(Tasks)
		workerCount--
	}
}

//insidestop
func (p *Pool) stop(wait bool) {
	p.stopMutex.Lock()
	defer p.stopMutex.Unlock()

	if wait {
		p.TaskQueue <- nil
	}
	p.stopped()
}

//Queue Size
func (p *Pool) WaitingQueueSize() int {
	return p.waitingQueue.Len()
}

//stops pool
func (p *Pool) Stop() {
	p.stop(false)
}

//stops and waits for all processes to finish
func (p *Pool) StopWait() {
	p.stop(true)
}

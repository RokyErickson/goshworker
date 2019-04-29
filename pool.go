package goshworker

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/RokyErickson/channels"
	"github.com/gammazero/deque"

	. "github.com/RokyErickson/gosh"
)

// Pool is a pool of worker processes.

const (
	RoutineGoshPoolQueueSize = 16
	idleTimeout              = 5
)

var Tasks channels.Channel

var mutex sync.Mutex

type GoshPool struct {
	Size            int
	Opts            Opts
	timeout         time.Duration
	stopped         bool
	ProcGoshPool    chan *goshworker
	RoutineGoshPool chan channels.Channel
	StartReady      chan channels.Channel
	TaskQueue       chan *Work
	StopChannel     chan struct{}
	waitingQueue    deque.Deque
	poolmutex       sync.Mutex
}

//work = task + chan error
type Work struct {
	task    Task
	errchan chan error
}

//io task
type Task func(in io.Writer, out, err io.Reader) error

//Main interface type
type Worker interface {
	Submit(Task)
	Stop()
	StopWait()
}

var _ Worker = (*GoshPool)(nil)

func assertSize(size int) {
	if size <= 0 {
		panic(fmt.Sprintf("pool size is invalid (%d)", size))
	}
}

//New GoshPool!
func NewGoshPool(size int) *GoshPool {

	pool := &GoshPool{
		Size:            size,
		timeout:         time.Second * idleTimeout,
		ProcGoshPool:    make(chan *goshworker, size),
		RoutineGoshPool: make(chan channels.Channel, RoutineGoshPoolQueueSize),
		StartReady:      make(chan channels.Channel),
		TaskQueue:       make(chan *Work, 1),
		StopChannel:     make(chan struct{}),
	}
	assertSize(pool.Size)
	pool.send()
	return pool
}

//sends tasks
func (p *GoshPool) send() {

	go func() {
		mutex.Lock()
		defer close(p.StopChannel)

		timeout := time.NewTimer(p.timeout)
		var (
			workerCount int
			work        *Work
			ok, wait    bool
		)

		numWorkersTotal, _ := GetNumWorkersTotal()

		if p.Size > numWorkersTotal {
			panic("Cannot obtain more workers than the number in the global worker pool")
		}

		numWorkersAvail, _ := GetNumWorkersAvail()

		if p.Size > numWorkersAvail {
			panic("Not enough workers available at this moment")
		}
		for i := 0; i < p.Size; i++ {
			worker := <-GoshPoolGlobal
			worker.isActive = true
			go func(worker *goshworker) { p.ProcGoshPool <- worker }(worker)
		}
		mutex.Unlock()

		//incoming tasks directly to worker
	Cycle:
		for {
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
					Tasks = <-p.RoutineGoshPool
					//worker ready, make him work!
					Tasks.In() <- p.waitingQueue.PopFront().(*Work)
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
				case Tasks := <-p.RoutineGoshPool:
					//give work
					Tasks.In() <- work
				default:
					//proc not ready make new proc.
					if workerCount < p.Size {
						workerCount++
						worker := <-p.ProcGoshPool
						worker.Start()
						go func(worker *goshworker, work *Work) {
							worker.Run(p.StartReady, p.RoutineGoshPool)
							Tasks := <-p.StartReady
							Tasks.In() <- work
							p.ProcGoshPool <- worker
						}(worker, work)
					} else {
						//queued task to be executed by worker
						p.waitingQueue.PushBack(work)

					}
				}
			case <-timeout.C:
				//timeout waiting for work
				if workerCount > 0 {
					select {
					case Tasks = <-p.RoutineGoshPool:
						//Kill routine pool proc.
						Tasks.Close()
						workerCount--
					default:
						//No work, but all procs busy.
					}
				}
			}
		}
		//if set to wait, push all work to queue.
		if wait {
			for p.waitingQueue.Len() != 0 {
				Tasks = <-p.RoutineGoshPool
				Tasks.In() <- p.waitingQueue.PopFront().(*Work)
			}
		}

		//stop all procs
		for workerCount > 0 {
			Tasks = <-p.RoutineGoshPool
			Tasks.Close()
			workerCount--
		}
	}()
}

//insidestop
func (p *GoshPool) stop(wait bool) {
	p.poolmutex.Lock()
	defer p.poolmutex.Unlock()

	if p.stopped {
		return
	}
	p.stopped = true
	if wait {
		p.TaskQueue <- nil
	}

	workernum := cap(p.ProcGoshPool)
	for i := 0; i < workernum; i++ {
		worker := <-p.ProcGoshPool
		worker.Recycle()
	}
	close(p.TaskQueue)
	<-p.StopChannel
}

//Queue Size
func (p *GoshPool) WaitingQueueSize() int {
	return p.waitingQueue.Len()
}

//stops pool
func (p *GoshPool) Stop() {
	p.stop(false)
}

//stops and waits for all processes to finish
func (p *GoshPool) StopWait() {
	p.stop(true)
}

//submits task
func (p *GoshPool) Submit(t Task) {
	if t != nil {
		w := &Work{t, make(chan error, 1)}
		p.TaskQueue <- w
	}
}

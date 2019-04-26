package goshworker

import (
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/gammazero/deque"

	. "github.com/RokyErickson/gosh"
)

// Pool is a pool of worker processes.

const (
	RoutinePoolQueueSize = 16
	idleTimeout          = 5
)

var Tasks chan *Work
var mutex sync.Mutex

type Pool struct {
	Size         int
	Opts         Opts
	timeout      time.Duration
	stopped      bool
	ProcPool     chan *proc
	RoutinePool  chan chan *Work
	StartReady   chan chan *Work
	TaskQueue    chan *Work
	StopChannel  chan struct{}
	waitingQueue deque.Deque
	poolMutex    sync.Mutex
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

var _ Worker = (*Pool)(nil)

func assertSize(size int) {
	if size <= 0 {
		panic(fmt.Sprintf("pool size is invalid (%d)", size))
	}
}

//New Pool!
func NewGoshPool(size int) *Pool {

	pool := &Pool{
		Size:        size,
		timeout:     time.Second * idleTimeout,
		ProcPool:    make(chan *proc, size),
		RoutinePool: make(chan chan *Work, RoutinePoolQueueSize),
		StartReady:  make(chan chan *Work),
		TaskQueue:   make(chan *Work, 1),
		StopChannel: make(chan struct{}),
	}
	assertSize(pool.Size)
	go pool.send()
	return pool
}

//sends tasks
func (p *Pool) send() {
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

	go func() {

	Cycle:

		for {
			select {
			case goshworker, ok := <-PoolGlobal:
				if !ok {
					break Cycle
				}
				go func(goshworker *proc) {
					mutex.Lock()
					defer mutex.Unlock()
					goshworker.isActive = true
					p.ProcPool <- goshworker
				}(goshworker)
			default:
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
							goshworker := <-p.ProcPool
							goshworker.Start()
							go func(goshworker *proc, work *Work) {
								goshworker.Run(p.StartReady, p.RoutinePool)
								Tasks := <-p.StartReady
								Tasks <- work
								p.ProcPool <- goshworker
							}(goshworker, work)
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
		}
		//if set to wait, push all work to queue.
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
	}()
}

//insidestop
func (p *Pool) stop(wait bool) {

	p.poolMutex.Lock()
	defer p.poolMutex.Unlock()
	if p.stopped {
		return
	}
	p.stopped = true
	if wait {
		p.TaskQueue <- nil
	}

	procnum := cap(p.ProcPool)
	for i := 0; i < procnum; i++ {
		proc := <-p.ProcPool
		proc.Recycle()
	}
	close(p.TaskQueue)
	<-p.StopChannel
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

func (p *Pool) Submit(t Task) {
	if t != nil {
		w := &Work{t, make(chan error, 1)}
		p.TaskQueue <- w
	}
}

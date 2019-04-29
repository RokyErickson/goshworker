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
	Size            channels.BufferCap
	Opts            Opts
	timeout         time.Duration
	stopped         bool
	ProcGoshPool    channels.Channel
	RoutineGoshPool channels.Channel
	StartReady      channels.Channel
	TaskQueue       chan *Work
	StopChannel     channels.Channel
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

func assertSize(size channels.BufferCap) {
	if size <= 0 {
		panic(fmt.Sprintf("pool size is invalid (%d)", size))
	}
}

//New GoshPool!
func NewGoshPool(size channels.BufferCap) *GoshPool {

	pool := &GoshPool{
		Size:            size,
		timeout:         time.Second * idleTimeout,
		ProcGoshPool:    channels.NewNativeChannel(size),
		RoutineGoshPool: channels.NewNativeChannel(RoutineGoshPoolQueueSize),
		StartReady:      channels.NewNativeChannel(0),
		TaskQueue:       make(chan *Work, 1),
		StopChannel:     channels.NewNativeChannel(0),
	}
	assertSize(pool.Size)
	pool.send()
	return pool
}

//sends tasks
func (p *GoshPool) send() {

	go func() {
		mutex.Lock()
		defer p.StopChannel.Close()

		timeout := time.NewTimer(p.timeout)
		var (
			workerCount int
			wait        bool
		)

		numWorkersTotal, _ := GetNumWorkersTotal()

		if p.Size > numWorkersTotal {
			panic("Cannot obtain more workers than the number in the global worker pool")
		}

		numWorkersAvail, _ := GetNumWorkersAvail()

		var size int = int(p.Size)

		if size > numWorkersAvail {
			panic("Not enough workers available at this moment")
		}
		for i := 0; i < size; i++ {
			shellworker := <-GoshPoolGlobal.Out()
			worker := shellworker.(*goshworker)
			worker.isActive = true
			go func(worker *goshworker) { p.ProcGoshPool.In() <- worker }(worker)
		}
		mutex.Unlock()

		//incoming tasks directly to worker
	Cycle:
		for {
			if p.waitingQueue.Len() != 0 {
				select {
				case work, ok := <-p.TaskQueue:
					if !ok {
						break Cycle
					}
					if work == nil {
						wait = true
						break Cycle
					}
					p.waitingQueue.PushBack(work)
					Tasks := <-p.RoutineGoshPool.Out()
					TasksChan := Tasks.(channels.Channel)
					//worker ready, make him work!
					TasksChan.In() <- p.waitingQueue.PopFront().(*Work)
				}
				continue
			}
			//working queue empty
			timeout.Reset(p.timeout)
			select {
			case work, ok := <-p.TaskQueue:
				if !ok || work == nil {
					break Cycle
				}
				//got work to do.
				select {
				case Tasks := <-p.RoutineGoshPool.Out():
					TasksChan := Tasks.(channels.Channel)
					//give work
					TasksChan.In() <- work
				default:
					//proc not ready make new proc.
					if workerCount < size {
						workerCount++
						shellworker := <-p.ProcGoshPool.Out()
						worker := shellworker.(*goshworker)
						worker.Start()
						go func(worker *goshworker, work *Work) {
							worker.Run(p.StartReady, p.RoutineGoshPool)
							Tasks := <-p.StartReady.Out()
							TasksChan := Tasks.(channels.Channel)
							TasksChan.In() <- work
							p.ProcGoshPool.In() <- worker
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
					case Tasks := <-p.RoutineGoshPool.Out():
						TasksChan := Tasks.(channels.Channel)
						//Kill routine pool proc.
						TasksChan.Close()
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
				Tasks := <-p.RoutineGoshPool.Out()
				TasksChan := Tasks.(channels.Channel)
				TasksChan.In() <- p.waitingQueue.PopFront().(*Work)
			}
		}

		//stop all procs
		for workerCount > 0 {
			Tasks := <-p.RoutineGoshPool.Out()
			TaskChan := Tasks.(channels.Channel)
			TaskChan.Close()
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
	var num int
	workernum := p.ProcGoshPool.Cap()
	num = int(workernum)
	for i := 0; i < num; i++ {
		shellworker := <-p.ProcGoshPool.Out()
		worker := shellworker.(*goshworker)
		worker.Recycle()
	}
	close(p.TaskQueue)
	p.StopChannel.Out()
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

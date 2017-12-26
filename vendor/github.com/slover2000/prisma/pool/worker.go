package pool

import (
	"fmt"
	"sync"
	"time"
	"hash/fnv"
	"math/rand"
)

type (
	// Job is what need to be run in the pool
	Job interface {
		Key() string
		Run()
	}

	// Executor This interface provides a way of decoupling task submission from the mechanics of how each task will be run, including details of thread use, 
	Executor interface {
		Execute(Job) bool
		Stop()
	}

	workerChan struct {
		lastUseTime time.Time
		ch          chan Job
	}

	// fixedWorkerPool is a pool that reuses a fixed number of threads
	fixedWorkerPool struct {
		lock         	sync.Mutex
		workersCount	int
		ready					[]*workerChan
		stopCh 				chan struct{}
	}
)

// NewFixedWorkerPool create a worker pool with fixed number of thread
func NewFixedWorkerPool(num int, block bool) Executor {
	pool := &fixedWorkerPool{
		workersCount: num,
		ready: make([]*workerChan, 0),
		stopCh: make(chan struct{}),
	}

	// initialize pool workers
	for i := 0; i < pool.workersCount; i++ {
		var ch *workerChan
		if block {
			ch = &workerChan{lastUseTime: time.Now(), ch: make(chan Job, 0)}
		}	else {
			ch = &workerChan{lastUseTime: time.Now(), ch: make(chan Job, 1)}
		}
		pool.ready = append(pool.ready, ch)
		go func(input chan Job) {
			for job := range input {
				job.Run()
			}
			fmt.Println("go worker exit.")
		}(ch.ch)
	}

	return pool
}

// Execute fixed worker pool execute implementation
func (wp *fixedWorkerPool) Execute(job Job) bool {
	hashCode := uint32(0)
	key := job.Key()
	if len(key) > 0 {
		h := fnv.New32a()
		h.Write([]byte(key))
		hashCode = h.Sum32()
	}

	wp.lock.Lock()
	defer wp.lock.Unlock()

	idx := 0
	if hashCode != 0 {
		idx = int(hashCode % uint32(len(wp.ready)))
	} else {
		idx = rand.Intn(len(wp.ready))
	}
	
	ch := wp.ready[idx]
	if ch == nil {
		return false
	}

	ch.ch <- job
	return true
}

// Stop stop the fixed worker pool
func (wp *fixedWorkerPool) Stop() {
	if wp.stopCh == nil {
		panic("BUG: workerPool wasn't started")
	}
	close(wp.stopCh)
	wp.stopCh = nil

	// Stop all the workers waiting for jobs.
	// Do not wait for busy workers - they will stop after
	// serving the job
	wp.lock.Lock()
	defer wp.lock.Unlock()

	ready := wp.ready
	for i, ch := range ready {
		close(ch.ch)
		ready[i] = nil
	}
	wp.ready = ready[:0]
}
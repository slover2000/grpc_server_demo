package pool

import (	
	"log"
	"time"
	"runtime"
	"hash/fnv"
	"math/rand"	
	"container/heap"
	"github.com/sasha-s/go-deadlock"
)

type (
	// ScheduledExecutor that can schedule a job to run after a given delay, or to execute periodically
	ScheduledExecutor interface {
		Schedule(job Job, delay time.Duration)
		ScheduleAtFixedRate(job Job, initDelay, period time.Duration)
		Stop()
	}

	// An scheduleJob is something we manage in a schedule queue.
	scheduleJob struct {
		job        Job
		runtime    time.Time
		period     time.Duration
		// The index is needed by update and is maintained by the heap.Interface methods.
		index 		int // The index of the item in the heap.
	}

	// A scheduleQueue implements heap.Interface and holds todo jobs.
	scheduleQueue	[]*scheduleJob

	scheduleChan struct {
		ch			chan Job
	}

	// ScheduleWorkerPool is a pool implements ScheduledExecutor interface
	scheduleWorkerPool struct {
		lock			deadlock.Mutex
		ready			[]*scheduleChan
		queue			scheduleQueue
		stopCh 		    chan struct{}
	}
)

// NewScheduleWorkerPool create a scheduled worker pool
func NewScheduleWorkerPool(cores int) ScheduledExecutor {
	pool := &scheduleWorkerPool{
		ready: make([]*scheduleChan, 0),
		queue: make(scheduleQueue, 0),
		stopCh: make(chan struct{}),
	}

	stopCh := pool.stopCh
	// schedule routine
	go func() {
		for {
			select {
			case <-stopCh:
				log.Println("schedule pool exit")
				return
			default:
				todo, wait := pool.getNextRunningJob()
				if todo != nil {
					pool.submit(todo.job)					
					if todo.period > 0 {
						todo.runtime = time.Now().Add(todo.period)
						pool.lock.Lock()		
						heap.Push(&pool.queue, todo)
						pool.lock.Unlock()
					}
				} else {
					if wait > 0 {
						time.Sleep(wait)
					} else {
						time.Sleep(50 * time.Millisecond)
					}
				}
			}
		}		
	}()

	// initialize workers
	for i := 0; i < cores; i++ {
		ch := &scheduleChan{ch: make(chan Job, 1)}
		pool.ready = append(pool.ready, ch)
		go func(input chan Job) {
			for job := range input {
				job.Run()
			}
			log.Println("go schedule worker exit")
		}(ch.ch)
	}	

	return pool
}

func (sp *scheduleWorkerPool) Schedule(job Job, delay time.Duration) {
	sp.lock.Lock()
	defer sp.lock.Unlock()
	todo := &scheduleJob{job: job, runtime: time.Now().Add(delay)}
	heap.Push(&sp.queue, todo)
}

func (sp *scheduleWorkerPool) ScheduleAtFixedRate(job Job, initDelay, period time.Duration) {
	sp.lock.Lock()
	defer sp.lock.Unlock()	
	todo := &scheduleJob{job: job, runtime: time.Now().Add(initDelay), period: period}
	heap.Push(&sp.queue, todo)
}

func (sp *scheduleWorkerPool) getNextRunningJob() (todo *scheduleJob, delay time.Duration) {
	sp.lock.Lock()
	defer sp.lock.Unlock()

	if len(sp.queue) > 0 {
		now := time.Now()
		wait := sp.queue[0].runtime.Sub(now)
		if wait > 0 {
			delay = wait
		} else {
			todo = heap.Pop(&sp.queue).(*scheduleJob)
		}
	}

	return
}

func (sp *scheduleWorkerPool) submit(job Job) {
	hashCode := uint32(0)
	key := job.Key()
	if len(key) > 0 {
		h := fnv.New32a()
		h.Write([]byte(key))
		hashCode = h.Sum32()
	}

	idx := 0
	if hashCode != 0 {
		idx = int(hashCode % uint32(len(sp.ready)))
	} else {
		idx = rand.Intn(len(sp.ready))
	}
	log.Printf("select worker: %d", idx)
	ch := sp.ready[idx]
	ch.ch <- job
}

// Stop stop the schedule pool
func (sp *scheduleWorkerPool) Stop() {
	if sp.stopCh == nil {
		panic("BUG: schedulePool wasn't started")
	}
	close(sp.stopCh)
	sp.stopCh = nil

	sp.lock.Lock()
	defer sp.lock.Unlock()

	ready := sp.ready
	for i, ch := range ready {
		close(ch.ch)
		ready[i] = nil
	}
	sp.ready = ready[:0]
	sp.queue = sp.queue[:0]
}

func (sq scheduleQueue) Len() int { 
	return len(sq)
}

func (sq scheduleQueue) Less(i, j int) bool {
	return sq[i].runtime.Unix() < sq[j].runtime.Unix()
}

func (sq scheduleQueue) Swap(i, j int) {
	sq[i], sq[j] = sq[j], sq[i]
	sq[i].index = i
	sq[j].index = j
}

func (sq *scheduleQueue) Push(x interface{}) {
	n := len(*sq)
	item := x.(*scheduleJob)
	item.index = n
	*sq = append(*sq, item)
}

func (sq *scheduleQueue) Pop() interface{} {
	old := *sq
	n := len(old)
	item := old[n-1]
	item.index = -1 // for safety
	*sq = old[0 : n-1]
	return item
}
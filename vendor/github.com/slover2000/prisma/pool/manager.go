package pool

import (
	"sync"
	"runtime"
)

type (
	// Manager ...
	Manager interface {
		WorkerPool() Executor
		SchedulePool() ScheduledExecutor
		Init()
		Shutdown()
	}

	manager struct {
		fixedPool    Executor
		schedulePool ScheduledExecutor
	}
)

// NewManager create a instance of manager
func NewManager(cores int, block bool) Manager {
	manager := &manager{
		fixedPool: NewFixedWorkerPool(cores, block),
		schedulePool: NewScheduleWorkerPool(cores),
	}

	return manager
}

func (m *manager) Init() {

}

func (m *manager) WorkerPool() Executor {
	return m.fixedPool
}

func (m *manager) SchedulePool() ScheduledExecutor {
	return m.schedulePool
}

func (m *manager) Shutdown() {
	m.fixedPool.Stop()
	m.schedulePool.Stop()
}
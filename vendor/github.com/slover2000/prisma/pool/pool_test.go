package pool

import (
	"fmt"
	"time"
	"testing"	
)

type MockJob struct {
	key	string	
}

func (mock *MockJob) Key() string {
	return mock.key
}

func (mock *MockJob) Run() {	
	t := time.Now()
	fmt.Printf("%s mock job[%s].\n", mock.key, t.Format("2006-01-02 15:04:05"))
}

type MockScheduleJob struct {

}

func (mock *MockScheduleJob) Key() string {
	return ""
}

func (mock *MockScheduleJob) Run() {
	t := time.Now()
	fmt.Printf("%s schedule job run\n", t.Format("2006-01-02 15:04:05"))
}


func TestWorkerPool(t *testing.T) {
	fixedPool := NewFixedWorkerPool(5, false)
	boom := time.After(10 * time.Second)
	i := 0
	for {
		select {
		case <-boom:
			fixedPool.Stop()
			time.Sleep(100 * time.Millisecond)
			return
		default:
			mockJob := &MockJob{key: fmt.Sprintf("key:%d", i)}
			fixedPool.Execute(mockJob)
			i++
			time.Sleep(500 * time.Millisecond)
		}
	}
}

func TestSchedulePool(t *testing.T) {
	schedulePool := NewScheduleWorkerPool()
	stop := time.After(10 * time.Second)

	mockJob2 := &MockScheduleJob{}
	schedulePool.ScheduleAtFixedRate(mockJob2, 1 * time.Second, 1000 * time.Millisecond)
	i := 0
	for {
		select {
		case <-stop:
			schedulePool.Stop()
			time.Sleep(100 * time.Millisecond)
			return
		default:
			mockJob := &MockJob{key: fmt.Sprintf("key:%d", i)}
			schedulePool.Schedule(mockJob, 100 * time.Millisecond)
			i++
			time.Sleep(500 * time.Millisecond)
		}
	}	
}
package jobs

import (
	"sync"
	"sync/atomic"
)

// WorkQueue encapsulates operations on a
// workque.
type WorkQueue interface {
	// Get block waits for items from the workqueue and
	// returns an item .
	Get() interface{}
	// Stops sends a stop signal to thework queue
	Stop() bool
	// Add adds a task to this workqueue. The returned flag is for the operation success
	Add(task interface{}) bool
	// Count returns the current number of items in the queue
	Count() int
}

type workQueue struct {
	q      chan interface{}
	count  int32
	rwlock sync.RWMutex
}

// NewWorkQueue creates new WorkQueue implementation object
func NewWorkQueue(buffer int) WorkQueue {
	return &workQueue{
		q: make(chan interface{}, buffer),
	}
}

func (w *workQueue) Get() (task interface{}) {
	var ok bool
	if w.q == nil {
		return nil
	}
	select {
	case task, ok = <-w.q:
		{
			if ok {
				atomic.AddInt32(&w.count, -1)
			}
			return
		}
	}
}

func (w *workQueue) Stop() bool {
	defer w.rwlock.Unlock()
	w.rwlock.Lock()
	if w.q != nil {
		close(w.q)
		w.q = nil
	}
	return false
}

func (w *workQueue) Add(task interface{}) bool {
	if w.q == nil {
		return false
	}
	defer w.rwlock.Unlock()
	w.rwlock.Lock()
	select {
	case w.q <- task:
		{
			atomic.AddInt32(&w.count, 1)
			return true
		}
	}
}

func (w *workQueue) Count() int {
	return int(atomic.LoadInt32(&w.count))
}

package jobs

import (
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
	// Add adds a task to this workqueue
	Add(task interface{})
	// Count returns the current number of items in the queue
	Count() int
}

type workQueue struct {
	q      chan interface{}
	stopCh chan struct{}
	count  int32
}

// NewWorkQueue creates new WorkQueue implementation object
func NewWorkQueue(buffer int) WorkQueue {
	return &workQueue{
		q:      make(chan interface{}, buffer),
		stopCh: make(chan struct{}),
	}
}

func (w *workQueue) Get() (task interface{}) {
	var ok bool
	select {
	case <-w.stopCh:
		{
			return
		}
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
	if w.q != nil {
		defer func() {
			close(w.stopCh)
			close(w.q)
			w.q = nil
		}()
		// make sure there's at least one consumer for the
		// stopCh message or it will block waiting forever
		go func() {
			<-w.stopCh
		}()
		w.stopCh <- struct{}{}
		return true
	}
	return false
}

func (w *workQueue) Add(task interface{}) {
	if w.q == nil {
		panic("Trying to add to a not started workqueue")
	}
	select {
	case <-w.stopCh:
		{
			return
		}
	case w.q <- task:
		{
			atomic.AddInt32(&w.count, 1)
		}
	}
}

func (w *workQueue) Count() int {
	return int(atomic.LoadInt32(&w.count))
}

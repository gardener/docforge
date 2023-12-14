// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/hashicorp/go-multierror"
	"k8s.io/klog/v2"
)

const (
	maxWorkerSize = 100
	minWorkerSize = 1
	bufferSize    = 200
)

// JobQueue enqueues assignments for parallel processing and synchronous response
type JobQueue struct {
	// id is job identifier used in log messages
	id string
	// size is the number of workers processing a batch of tasks in parallel
	size int
	// workFunc for processing tasks
	workFunc WorkerFunc
	// failFast controls the behavior of this JobQueue upon errors. If set to true, it will quit
	// further processing upon the first error that occurs. For fault tolerant applications
	// use false.
	failFast bool
	// tracks the number tasks to wait for
	wg *sync.WaitGroup
	// tasks is the queue for tasks picked up by the workers in this JobQueue. The AddTask
	// method will feed the queue with elements, and it may be fed from other sources in parallel.
	tasks chan interface{}
	// errList collect workers errors
	errList *multierror.Error
	// initialize and stop mutex
	initMux, stopMux sync.Once
	// synchronization mutex
	mux sync.Mutex
	// if true the queue is stopped
	stopped bool
	// processed tasks count
	tc uint32
}

// QueueController can Start/Stop the queue and see its status
type QueueController interface {
	// Start initializes worker's goroutines. The provided context ctx is used by worker goroutines
	Start(ctx context.Context)
	// Stop stops the worker's goroutines, it could be triggered internally on context cancellation or failFast situation
	Stop()
	// GetErrorList returns the errors, occurred during task processing
	GetErrorList() *multierror.Error
	// GetProcessedTasksCount returns the processed tasks count
	GetProcessedTasksCount() int
	// GetWaitingTasksCount returns waiting tasks count
	GetWaitingTasksCount() int
}

// The WorkerFunc type declares workers functional interface
type WorkerFunc func(ctx context.Context, task interface{}) error

// NewJobQueue create an empty task queue
func NewJobQueue(id string, size int, workFunc WorkerFunc, failFast bool, wg *sync.WaitGroup) (*JobQueue, error) {
	if size < minWorkerSize || size > maxWorkerSize {
		return nil, fmt.Errorf("job queue %s init fails: invalid workers size '%d', valid size interval is [%d,%d]", id, size, minWorkerSize, maxWorkerSize)
	}
	if workFunc == nil {
		return nil, fmt.Errorf("job queue %s init fails: worker func is nil", id)
	}
	if wg == nil {
		return nil, fmt.Errorf("job queue %s init fails: wait group is nil", id)
	}
	jq := &JobQueue{
		id:       id,
		size:     size,
		workFunc: workFunc,
		failFast: failFast,
		wg:       wg,
		tasks:    make(chan interface{}, bufferSize),
	}
	return jq, nil
}

// Start initializes worker's goroutines
// the provided context ctx is used by worker goroutines
func (jq *JobQueue) Start(ctx context.Context) {
	jq.initMux.Do(func() {
		klog.V(6).Infof("starting %s queue\n", jq.id)
		// start workers
		for i := 0; i < jq.size; i++ {
			go jq.work(ctx)
		}
	})
}

// Stop stops the worker's goroutines, it could be triggered
// internally on context cancellation or failFast situation
func (jq *JobQueue) Stop() {
	jq.stopMux.Do(func() {
		jq.mux.Lock()
		defer jq.mux.Unlock()
		klog.V(6).Infof("stopping %s queue\n", jq.id)
		jq.stopped = true
		close(jq.tasks)
	})
}

// AddTask adds a task to the tasks queue and increase wg counter
// returns true if the task is added and false if it is skipped
// (e.g. if the JobQueue is stopped or failFast situation)
func (jq *JobQueue) AddTask(task interface{}) bool {
	defer func() {
		if recover() != nil {
			// decrease wait group counter
			jq.wg.Done()
			klog.V(6).Infof("recover adding task %v in closed %s queue\n", task, jq.id)
		}
	}()
	if jq.shouldProcess() {
		jq.wg.Add(1)
		jq.tasks <- task
		return true
	}
	klog.V(6).Infof("skipping task %v in %s queue\n", task, jq.id)
	return false
}

// GetErrorList returns the errors, occurred during task processing
func (jq *JobQueue) GetErrorList() *multierror.Error {
	return jq.errList
}

// GetProcessedTasksCount returns the processed tasks count
func (jq *JobQueue) GetProcessedTasksCount() int {
	return int(jq.tc)
}

// GetWaitingTasksCount returns waiting tasks count
func (jq *JobQueue) GetWaitingTasksCount() int {
	return len(jq.tasks)
}

// worker's goroutines call work to process tasks from the tasks queue in a loop
// if context is canceled trigger JobQueue stop
func (jq *JobQueue) work(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			{
				klog.V(6).Infof("context is done for %s queue\n", jq.id)
				jq.Stop()
			}
		case t, ok := <-jq.tasks:
			{
				if !ok {
					klog.V(6).Infof("job queue %s is stopped\n", jq.id)
					return
				}
				jq.runWorkFunc(ctx, t)
			}
		}
	}
}

// runWorkFunc runs the work func, if error occurs appends the error to the errList
// and finally decrease wg counter
func (jq *JobQueue) runWorkFunc(ctx context.Context, t interface{}) {
	defer jq.wg.Done()
	defer atomic.AddUint32(&jq.tc, 1)
	defer func() {
		if r := recover(); r != nil {
			err := fmt.Errorf("panic in %s for task %v recovered: %v", jq.id, t, r)
			klog.Warning(err.Error(), "\n", string(debug.Stack()))
			jq.appendError(err)
		}
	}()
	if jq.shouldProcess() {
		if err := jq.workFunc(ctx, t); err != nil {
			jq.appendError(err)
		}
	}
}

// appendError appends an error in the errList
// triggers JobQueue stop if failFast is true
func (jq *JobQueue) appendError(err error) {
	jq.mux.Lock()
	defer jq.mux.Unlock()

	jq.errList = multierror.Append(jq.errList, err)
	if jq.failFast {
		go jq.Stop() // trigger stop in separate goroutine
	}
}

// shouldProcess determines whether to continue processing work or not
// if queue is stopped or failFast is true and error has occurred,
// then stop further processing
func (jq *JobQueue) shouldProcess() bool {
	jq.mux.Lock()
	defer jq.mux.Unlock()

	return !jq.stopped && !(jq.failFast && jq.errList != nil)
}

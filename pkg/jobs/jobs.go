// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
	klog "k8s.io/klog/v2"
)

// Job enqueues assignments for parallel processing and synchronous response
type Job struct {
	// ID is job identifier used in log messages
	ID string
	// MaxWorkers is the maximum number of workers processing a batch of tasks in parallel
	MaxWorkers int
	// MinWorkers is the minimum number of workers processing a batch of tasks in parallel
	MinWorkers int
	// Worker for processing tasks
	Worker Worker
	// FailFast controls the behavior of this Job upon errors. If set to true, it will quit
	// further processing upon the first error that occurs. For fault tolerant applications
	// use false.
	FailFast bool
	// WorkQueue is the queue for tasks picked up by the workers in this Job. The Dispatch
	// method will feed its tasks argument elements to the queue, and it may be fed
	// from other sources in parallel, including the workers.
	Queue WorkQueue
	// IsWorkerExitsOnEmptyQueue controls whether a worker exits right after its Work function is
	// done and no more tasks are available in the queue, or will loop waiting for more tasks.
	// Note that this flag does not prevent the worker from block waiting for a task. This
	// can be interrupted only by the workqueue with a task or stop signal. However, after a task
	// is processed it will be consulted whether to continue or exit before block waiting for
	// another.
	isWorkerExitsOnEmptyQueue bool

	rwlock sync.RWMutex
}

// WorkerError wraps an underlying error struct and adds optional code
// to enrich the context of the error e.g. with HTTP status codes
type WorkerError struct {
	error
	code int
}

// NewWorkerError creates worker errors
func NewWorkerError(err error, code int) *WorkerError {
	return &WorkerError{
		err,
		code,
	}
}

// Is implements the contract for errors.Is (https://golang.org/pkg/errors/#Is)
func (we WorkerError) Is(target error) bool {
	var (
		_target WorkerError
		ok      bool
	)
	if _target, ok = target.(WorkerError); !ok {
		return false
	}
	if we.code != _target.code {
		return false
	}
	if !errors.Is(we.error, _target.error) {
		return false
	}
	return true
}

// Unwrap implements the contract for errors.Unwrap (https://golang.org/pkg/errors/#Unwrap)
func (we WorkerError) Unwrap() error {
	return we.error
}

func newerror(err error, code int) *WorkerError {
	return &WorkerError{
		err,
		code,
	}
}

// Worker declares workers functional interface
type Worker interface {
	// Work processes the task within the given context.
	Work(ctx context.Context, task interface{}, wq WorkQueue) *WorkerError
}

// The WorkerFunc type is an adapter to allow the use of
// ordinary functions as Workers. If f is a function
// with the appropriate signature, WorkerFunc(f) is a
// Worker object that calls f.
type WorkerFunc func(ctx context.Context, task interface{}, wq WorkQueue) *WorkerError

// Work calls f(ctx, Task).
func (f WorkerFunc) Work(ctx context.Context, task interface{}, wq WorkQueue) *WorkerError {
	return f(ctx, task, wq)
}

// Dispatch spawns a set of workers processing in parallel the supplied tasks.
// If the context is cancelled or has timed out (if it's a timeout context), or if
// any other error occurs during processing of tasks, a workerError error is
// returned as soon as possible, processing halts and workers are disposed.
func (j *Job) Dispatch(ctx context.Context, tasks []interface{}) *WorkerError {
	if j.MaxWorkers < j.MinWorkers {
		panic(fmt.Sprintf("Job %s maxWorkers < minWorkers: %d < %d", j.ID, j.MaxWorkers, j.MinWorkers))
	}
	if j.MaxWorkers == 0 {
		return nil
	}
	workersCount := len(tasks)
	if workersCount > j.MaxWorkers {
		workersCount = j.MaxWorkers
	}
	if workersCount < j.MinWorkers {
		workersCount = j.MinWorkers
	}

	var (
		errors              *multierror.Error
		workerError         *WorkerError
		loop                = true
		stoppedWorkersCount int
		quitCh              = make(chan struct{})
	)

	// add tasks
	for i := 0; i < len(tasks); i++ {
		j.Queue.Add(tasks[i])
	}

	// fire up parallel workers
	errCh := j.startWorkers(ctx, workersCount, quitCh)

	// wait job done or context cancelled
	for loop {
		select {
		case <-ctx.Done():
			{
				if j.Queue.Stop() {
					klog.V(1).Infof("Context canceled -> workqueue stopped\n")
				}
				break
			}
		case <-quitCh:
			{
				stoppedWorkersCount++
				if stoppedWorkersCount == 1 {
					// at least one worker exited - we are done
					// Unlock all others waiting to get a task
					if j.Queue.Stop() {
						klog.V(1).Infof("Workqueue stopped\n")
					}
				}
				if stoppedWorkersCount == workersCount {
					if j.Queue.Count() > 0 {
						klog.Warningf(
							fmt.Sprintf("%d unprocessed items in queue. ", j.Queue.Count()),
						)
					}
					loop = false
				}
			}
		case err, ok := <-errCh:
			{
				if !ok {
					break
				}
				if err != nil {
					errors = multierror.Append(errors, err)
					if j.FailFast {
						if j.Queue.Stop() {
							klog.V(1).Infof("Workqueue stopped\n")
						}
						break
					}
				}
			}
		}
	}
	if err := errors.ErrorOrNil(); err != nil {
		workerError = NewWorkerError(err, 0)
	}
	return workerError
}

// blocks waiting until the required amount of workers are started
func (j *Job) startWorkers(ctx context.Context, workersCount int, quitCh chan struct{}) <-chan *WorkerError {
	var (
		errcList = make([]<-chan *WorkerError, 0)
		wg       sync.WaitGroup
		// rwLock   sync.RWMutex
	)
	// appendToErrChClist := func(errcList []<-chan *WorkerError, errCh chan *WorkerError) {
	// 	defer rwLock.Unlock()
	// 	rwLock.Lock()
	// 	errcList = append(errcList, errCh)
	// }

	wg.Add(workersCount)
	for i := 0; i < workersCount; i++ {
		go func(ctx context.Context, workerId int, wq WorkQueue, quitCh chan struct{}) {
			errCh := make(chan *WorkerError, 1)
			defer func(errCh chan *WorkerError) {
				quitCh <- struct{}{}
				close(errCh)
				klog.V(1).Infof("%s worker %d stopped\n", j.ID, workerId)
			}(errCh)
			errcList = append(errcList, errCh)
			// appendToErrChClist(errcList, errCh)

			wg.Done()
			klog.V(1).Infof("%s worker %d started\n", j.ID, workerId)
			for {
				var task interface{}
				if !j.GetIsWorkerExitsOnEmptyQueue() && wq.Count() == 0 {
					return
				}
				klog.V(1).Infof("%s worker %d waiting for task \n", j.ID, workerId)
				if task = wq.Get(); task == nil {
					return
				}
				klog.V(1).Infof("%s worker %d acquired task\n", j.ID, workerId)
				if err := j.Worker.Work(ctx, task, wq); err != nil {
					errCh <- err
				}
				klog.V(1).Infof("%s worker %d finished task: %v \n", j.ID, workerId, task)
			}
		}(ctx, i, j.Queue, quitCh)
	}
	// Wait for all error channels to be populated
	wg.Wait()
	errCh := mergeErrors(errcList...)
	return errCh
}

// GetIsWorkerExitsOnEmptyQueue .. TODO:
func (j *Job) GetIsWorkerExitsOnEmptyQueue() bool {
	defer j.rwlock.Unlock()
	j.rwlock.Lock()
	return j.isWorkerExitsOnEmptyQueue
}

// SetIsWorkerExitsOnEmptyQueue .. TODO:
func (j *Job) SetIsWorkerExitsOnEmptyQueue(isWorkerExitsOnEmptyQueue bool) {
	defer j.rwlock.Unlock()
	j.rwlock.Lock()
	j.isWorkerExitsOnEmptyQueue = isWorkerExitsOnEmptyQueue
}

// merges asynchronously produced errors from multiple error channels into a single channel
func mergeErrors(channels ...<-chan *WorkerError) <-chan *WorkerError {
	var wg sync.WaitGroup
	// We must ensure that the output channel has the capacity to hold as many errors
	// as there are error channels. This will ensure that it never blocks, even
	// if waitForPipeline returns early.
	errCh := make(chan *WorkerError, len(channels))

	// Start an outputF goroutine for each input channel in channels.  outputF
	// copies values from ch to errCh until ch is closed, then calls wg.Done.
	outputF := func(ch <-chan *WorkerError) {
		for err := range ch {
			errCh <- err
		}
		wg.Done()
	}
	wg.Add(len(channels))
	for _, ch := range channels {
		go outputF(ch)
	}

	// Start a goroutine to close errCh once all the outputF goroutines are
	// done. This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(errCh)
	}()
	return errCh
}

// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
// This file is licensed under the Apache Software License, v.2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/hashicorp/go-multierror"
)

// Job enques assignments for parallel processing and synchronous response
type Job struct {
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

func newerror(err error, code int) *WorkerError {
	return &WorkerError{
		err,
		code,
	}
}

// Worker declares workers functional interface
type Worker interface {
	// Work processes the task within the given context.
	Work(ctx context.Context, task interface{}) *WorkerError
}

// The WorkerFunc type is an adapter to allow the use of
// ordinary functions as Workers. If f is a function
// with the appropriate signature, WorkerFunc(f) is a
// Worker object that calls f.
type WorkerFunc func(ctx context.Context, task interface{}) *WorkerError

// Work calls f(ctx, Task).
func (f WorkerFunc) Work(ctx context.Context, task interface{}) *WorkerError {
	return f(ctx, task)
}

// Allocates worker tasks and error channels and asynchronously feeds tasks to the worker tasks channel
// staying sensitive to termination signals from the provided context. Context terminal signals are registered
// as errors to the error channel.
func (j *Job) allocate(ctx context.Context, tasks []interface{}) (<-chan interface{}, <-chan *WorkerError) {
	msgCh := make(chan interface{})
	errCh := make(chan *WorkerError)
	go func() {
		defer close(msgCh)
		defer close(errCh)
		for _, task := range tasks {
			select {
			case msgCh <- task:
			case <-ctx.Done():
				{
					errCh <- newerror(ctx.Err(), 0)
					return
				}
			}
		}
	}()
	return msgCh, errCh
}

// Processes asynchronously tasks from the tasks channel until channel is closed or context signals
// termination. The processing delegates to the Worker.Work function implementation registered in this Job.
// Context terminal signals are registered as errors and sent to the error channel.
func (j *Job) process(ctx context.Context, taskCh <-chan interface{}) <-chan *WorkerError {
	errCh := make(chan *WorkerError, 1)
	go func() {
		defer close(errCh)
		for {
			select {
			case task, ok := <-taskCh:
				{
					if !ok {
						return
					}
					if err := j.Worker.Work(ctx, task); err != nil {
						errCh <- err
						return
					}
				}
			case <-ctx.Done():
				{
					errCh <- newerror(ctx.Err(), 0)
					return
				}
			}
		}
	}()
	return errCh
}

// Dispatch spawns a set of workers processing in parallel the supplied tasks.
// If the context is cancelled or has timed out (if it's a timeout context), or if
// any other error occurs during processing of tasks, a workerError error is
// returned as soon as possible, processing halts and workers are disposed.
func (j *Job) Dispatch(ctx context.Context, tasks []interface{}) *WorkerError {
	if j.MaxWorkers < j.MinWorkers {
		panic(fmt.Sprintf("Job maxWorkers < minWorkers: %d < %d", j.MaxWorkers, j.MinWorkers))
	}
	workersCount := len(tasks)
	if workersCount > j.MaxWorkers {
		workersCount = j.MaxWorkers
	}
	if workersCount < j.MinWorkers {
		workersCount = j.MinWorkers
	}

	var errcList []<-chan *WorkerError

	taskCh, errc := j.allocate(ctx, tasks)
	errcList = append(errcList, errc)
	for i := 0; i < workersCount; i++ {
		errc = j.process(ctx, taskCh)
		errcList = append(errcList, errc)
	}

	return waitForPipeline(j.FailFast, errcList...)
}

// merges asynchronously produced errors from multiple error channels into a single channel
func mergeErrors(channels ...<-chan *WorkerError) <-chan *WorkerError {
	var wg sync.WaitGroup
	// We must ensure that the output channel has the capacity to hold as many errors
	// as there are error channels. This will ensure that it never blocks, even
	// if waitForPipeline returns early.
	errCh := make(chan *WorkerError, len(channels))

	// Start an outputF goroutine for each input channel in channels.  outputF
	// copies values from ch to errCh until c is closed, then calls wg.Done.
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

// waitForPipeline waits for results from all error channels.
// It returns early on the first error if failfast is true or
// collects errors and returns an aggregated error at the end.
func waitForPipeline(failFast bool, errChs ...<-chan *WorkerError) *WorkerError {
	var (
		errors      *multierror.Error
		workerError *WorkerError
	)
	errCh := mergeErrors(errChs...)
	for err := range errCh {
		if err != nil {
			if failFast {
				return err
			}
			errors = multierror.Append(err)
		}
	}
	if err := errors.ErrorOrNil(); err != nil {
		workerError = &WorkerError{
			error: errors,
		}
	}
	return workerError
}

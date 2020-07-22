// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
// This file is licensed under the Apache Software License, v.2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://wwj.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package reactor

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/golang/glog"
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
		var i int64
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
					i++
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

// GitHubWorker specializes in processing remote GitHub resources
type GitHubWorker struct {
	// MaxSizeResponseBody defines the maximum acceptable size of the body of a response from Github API service
	MaxSizeResponseBody int64
}

// GitHubTask is a unit of work specification that is processed by Worker
type GitHubTask struct {
	// URL is the address of the Github API used by this Task
	URL string
	// Username is the user in the credentials used to authenticate to Github API
	Username string
	// Password is the password in the credentials used to authenticate to Github API
	Password string
	// Token is the perosnal token in the credentials used to authenticate to Github API
	Token string
}

// Work implements Worker#Work function
func (b *GitHubWorker) Work(ctx context.Context, task interface{}) *WorkerError {
	if task, ok := task.(*GitHubTask); ok {
		url := task.URL
		//glog.V(6).Infof("marshalling task for transport to url %s", task.URL)

		req, err := http.NewRequest("GET", url, nil)
		req = req.WithContext(ctx)
		if err != nil {
			return newerror(fmt.Errorf("error in creating request to url `%v`", url), 0)
		}
		// req.Header.Set("Content-Type", "application/json")
		// req.SetBasicAuth(b.Username, b.Password)

		glog.V(6).Infof("sending task resource to %s", url)
		// glog.V(16).Infof("Task with UUID %s in request task: %+v", task.UUID, string(marshaled))

		client := InstrumentClient(http.DefaultClient)

		resp, err := client.Do(req)
		if err != nil {
			return newerror(err, 0)
		}

		// check for errors returned from the backend service
		if resp.StatusCode > 399 {
			return newerror(fmt.Errorf("sending task to resource %s failed with response code %d", task.URL, resp.StatusCode), resp.StatusCode)
		}

		var body []byte
		// Wrap the request body reader with MaxBytesReader to prevent clients
		// from accidentally or maliciously sending a large request and wasting
		// server resources. Returns a non-EOF error for a Read beyond the limit
		// ("http: request body too large") or nil for empty body.
		resp.Body = http.MaxBytesReader(nil, resp.Body, b.MaxSizeResponseBody)
		if body, err = ioutil.ReadAll(resp.Body); err != nil {
			// change error Task to be less misleading
			if err.Error() == "http: request body too large" {
				err = fmt.Errorf("response body too large")
			}
			return newerror(fmt.Errorf("reading response from task resource %s failed: %v", task.URL, err), 0)
		}

		if len(body) == 0 {
			return newerror(fmt.Errorf("reading response from task resource %s failed: no response body task found", task.URL), 0)
		}

		glog.V(4).Infof("successfully saved task resource %s", task.URL)
	}

	return nil
}

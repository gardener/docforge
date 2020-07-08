// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
// This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
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

package reactor

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	// "net/url"
	"sync"

	"github.com/golang/glog"
)

// WorkerInput is ..
type WorkerInput struct {
	URL string
}

// Backend dispatches assignments for parallel processing of workerInput queue
// and synchronous response
type Backend struct {
	// MaxWorkers is the maximum number of workers sending a batch auditlog workerInputs in parallel to backend
	MaxWorkers int
	// MinWorkers is the minimum number of workers sending a batch auditlog workerInputs in parallel to backend
	MinWorkers int
	// Worker implements a unit of work, i.e. a backend roundtrip
	Worker Worker
}

// WorkerError wraps an underlying error struct and adds optional code
// to enrich the context of the error e.g. with HTTP status codes
type WorkerError struct {
	error
	code int
}

func newerror(err error, code int) *WorkerError {
	return &WorkerError{
		err,
		code,
	}
}

// Worker declares workers functional interface
type Worker interface {
	// Work processes the workerInput with the given context
	Work(ctx context.Context, workerInput *WorkerInput) *WorkerError
}

// The WorkerFunc type is an adapter to allow the use of
// ordinary functions as Workers. If f is a function
// with the appropriate signature, WorkerFunc(f) is a
// Worker object that calls f.
type WorkerFunc func(ctx context.Context, workerInput *WorkerInput) *WorkerError

// Work calls f(ctx, workerInput).
func (f WorkerFunc) Work(ctx context.Context, workerInput *WorkerInput) *WorkerError {
	return f(ctx, workerInput)
}

func (w *Backend) allocate(ctx context.Context, workerInputs []*WorkerInput) (<-chan *WorkerInput, <-chan *WorkerError) {
	msgCh := make(chan *WorkerInput)
	errCh := make(chan *WorkerError)
	go func() {
		defer close(msgCh)
		defer close(errCh)
		for _, workerInput := range workerInputs {
			select {
			case msgCh <- workerInput:
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

func (w *Backend) process(ctx context.Context, workerInputCh <-chan *WorkerInput) <-chan *WorkerError {
	errCh := make(chan *WorkerError, 1)
	go func() {
		defer close(errCh)
		for {
			select {
			case workerInput, ok := <-workerInputCh:
				{
					if !ok {
						return
					}
					if err := w.Worker.Work(ctx, workerInput); err != nil {
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

// Dispatch spawns a set of workers processing in parallel the supplied workerInputs.
// If the context is cancelled or has timed out (if it's a timeout context), or if
// any other error occurs during processing of workerInputs, a Proxy error is returned
// as soon as possible, processing halts and workers are disposed.
func (w *Backend) Dispatch(ctx context.Context, workerInputs []*WorkerInput) *WorkerError {
	if w.MaxWorkers < w.MinWorkers {
		panic(fmt.Sprintf("Backend maxWorkers < minWorkers: %d < %d", w.MaxWorkers, w.MinWorkers))
	}
	workersCount := len(workerInputs)
	if workersCount > w.MaxWorkers {
		workersCount = w.MaxWorkers
	}
	if workersCount < w.MinWorkers {
		workersCount = w.MinWorkers
	}

	var errcList []<-chan *WorkerError

	workerInputCh, errc := w.allocate(ctx, workerInputs)
	errcList = append(errcList, errc)

	for i := 0; i < workersCount; i++ {
		errc = w.process(ctx, workerInputCh)
		errcList = append(errcList, errc)
	}

	return waitForPipeline(errcList...)
}

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
	// done.  This must start after the wg.Add call.
	go func() {
		wg.Wait()
		close(errCh)
	}()
	return errCh
}

// waitForPipeline waits for results from all error channels.
// It returns early on the first error.
func waitForPipeline(errChs ...<-chan *WorkerError) *WorkerError {
	errCh := mergeErrors(errChs...)
	for err := range errCh {
		if err != nil {
			return err
		}
	}
	return nil
}

// BackendWorker implements a Backend Work function for POSTing
// WorkerInput resources to the Auditlog service endpoint
type BackendWorker struct {
	// URL is the address that this worker will send logs to
	URL string
	// Username is the user in the credentials used to authenticate to the backend
	Username string
	// Password is the password in the credentials used to authenticate to the backend
	Password string
	// MaxSizeResponseBody defines the maximum acceptable size of the body of a response from backend service
	MaxSizeResponseBody int64
}

// Work implements Worker#Work function
func (b *BackendWorker) Work(ctx context.Context, workerInput *WorkerInput) *WorkerError {
	url := workerInput.URL
	//glog.V(6).Infof("marshalling workerInput for transport to url %s", workerInput.URL)

	req, err := http.NewRequest("GET", url, nil)
	req = req.WithContext(ctx)
	if err != nil {
		return newerror(fmt.Errorf("error in creating request to url `%v`", url), 0)
	}
	// req.Header.Set("Content-Type", "application/json")
	// req.SetBasicAuth(b.Username, b.Password)

	glog.V(6).Infof("sending workerInput resource to %s", url)
	// glog.V(16).Infof("WorkerInput with UUID %s in request workerInput: %+v", workerInput.UUID, string(marshaled))

	client := InstrumentClient(http.DefaultClient)

	resp, err := client.Do(req)
	if err != nil {
		return newerror(err, 0)
	}

	// check for errors returned from the backend service
	if resp.StatusCode > 399 {
		return newerror(fmt.Errorf("sending workerInput to resource %s failed with response code %d", workerInput.URL, resp.StatusCode), resp.StatusCode)
	}

	var body []byte
	// Wrap the request body reader with MaxBytesReader to prevent clients
	// from accidentally or maliciously sending a large request and wasting
	// server resources. Returns a non-EOF error for a Read beyond the limit
	// ("http: request body too large") or nil for empty body.
	resp.Body = http.MaxBytesReader(nil, resp.Body, b.MaxSizeResponseBody)
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		// change error workerInput to be less misleading
		if err.Error() == "http: request body too large" {
			err = fmt.Errorf("response body too large")
		}
		return newerror(fmt.Errorf("reading response from workerInput resource %s failed: %v", workerInput.URL, err), 0)
	}

	if len(body) == 0 {
		return newerror(fmt.Errorf("reading response from workerInput resource %s failed: no response body workerInput found", workerInput.URL), 0)
	}

	glog.V(4).Infof("successfully saved workerInput resource %s", workerInput.URL)

	return nil
}

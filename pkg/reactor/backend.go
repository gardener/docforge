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

// Payload is ..
type Payload struct {
	URL string
}

// Backend dispatches assignments for parallel processing of payload queue
// and synchronous response
type Backend struct {
	// MaxWorkers is the maximum number of workers sending a batch auditlog payloads in parallel to backend
	MaxWorkers int
	// MinWorkers is the minimum number of workers sending a batch auditlog payloads in parallel to backend
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
	// Work processes the payload with the given context
	Work(ctx context.Context, payload *Payload) *WorkerError
}

// The WorkerFunc type is an adapter to allow the use of
// ordinary functions as Workers. If f is a function
// with the appropriate signature, WorkerFunc(f) is a
// Worker object that calls f.
type WorkerFunc func(ctx context.Context, payload *Payload) *WorkerError

// Work calls f(ctx, payload).
func (f WorkerFunc) Work(ctx context.Context, payload *Payload) *WorkerError {
	return f(ctx, payload)
}

func (w *Backend) allocate(ctx context.Context, payloads []*Payload) (<-chan *Payload, <-chan *WorkerError) {
	msgCh := make(chan *Payload)
	errCh := make(chan *WorkerError)
	go func() {
		defer close(msgCh)
		defer close(errCh)
		for _, payload := range payloads {
			select {
			case msgCh <- payload:
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

func (w *Backend) process(ctx context.Context, payloadCh <-chan *Payload) <-chan *WorkerError {
	errCh := make(chan *WorkerError, 1)
	go func() {
		defer close(errCh)
		for {
			select {
			case payload, ok := <-payloadCh:
				{
					if !ok {
						return
					}
					if err := w.Worker.Work(ctx, payload); err != nil {
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

// Dispatch spawns a set of workers processing in parallel the supplied payloads.
// If the context is cancelled or has timed out (if it's a timeout context), or if
// any other error occurs during processing of payloads, a Proxy error is returned
// as soon as possible, processing halts and workers are disposed.
func (w *Backend) Dispatch(ctx context.Context, payloads []*Payload) *WorkerError {
	if w.MaxWorkers < w.MinWorkers {
		panic(fmt.Sprintf("Backend maxWorkers < minWorkers: %d < %d", w.MaxWorkers, w.MinWorkers))
	}
	workersCount := len(payloads)
	if workersCount > w.MaxWorkers {
		workersCount = w.MaxWorkers
	}
	if workersCount < w.MinWorkers {
		workersCount = w.MinWorkers
	}

	var errcList []<-chan *WorkerError

	payloadCh, errc := w.allocate(ctx, payloads)
	errcList = append(errcList, errc)

	for i := 0; i < workersCount; i++ {
		errc = w.process(ctx, payloadCh)
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
// Payload resources to the Auditlog service endpoint
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
func (b *BackendWorker) Work(ctx context.Context, payload *Payload) *WorkerError {
	url := payload.URL
	glog.V(6).Infof("marshalling payload for transport to url %s", payload.URL)

	req, err := http.NewRequest("GET", url, nil)
	req = req.WithContext(ctx)
	if err != nil {
		return newerror(fmt.Errorf("error in creating request to url `%v`", url), 0)
	}
	// req.Header.Set("Content-Type", "application/json")
	// req.SetBasicAuth(b.Username, b.Password)

	glog.V(6).Infof("sending payload resource with to %s", url)
	// glog.V(16).Infof("Payload with UUID %s in request payload: %+v", payload.UUID, string(marshaled))

	client := InstrumentClient(http.DefaultClient)

	resp, err := client.Do(req)
	if err != nil {
		return newerror(err, 0)
	}

	// check for errors returned from the backend service
	if resp.StatusCode > 399 {
		return newerror(fmt.Errorf("sending payload resource for %s failed with response code %d", payload.URL, resp.StatusCode), resp.StatusCode)
	}

	var body []byte
	// Wrap the request body reader with MaxBytesReader to prevent clients
	// from accidentally or maliciously sending a large request and wasting
	// server resources. Returns a non-EOF error for a Read beyond the limit
	// ("http: request body too large") or nil for empty body.
	resp.Body = http.MaxBytesReader(nil, resp.Body, b.MaxSizeResponseBody)
	if body, err = ioutil.ReadAll(resp.Body); err != nil {
		// change error payload to be less misleading
		if err.Error() == "http: request body too large" {
			err = fmt.Errorf("response body too large")
		}
		return newerror(fmt.Errorf("reading response for payload resource %s failed: %v", payload.URL, err), 0)
	}

	if len(body) == 0 {
		return newerror(fmt.Errorf("reading response for payload resource %s failed: no response body payload found", payload.URL), 0)
	}

	glog.V(4).Infof("successfully saved payload resource %s", payload.URL)

	return nil
}

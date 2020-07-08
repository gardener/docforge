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
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gardener/docode/pkg/metrics"
	"github.com/gardener/docode/pkg/util/tests"
	"github.com/gardener/docode/pkg/util/units"
	"github.com/hashicorp/go-multierror"
	"github.com/prometheus/client_golang/prometheus"
	io_prometheus_client "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
)

func init() {
	tests.SetGlogV(6)
}

var shortSenderCallsCount int32

func newTasksList(tasksCount int, serverURL string, randomizePaths bool) []*Task {
	var Tasks []*Task

	if tasksCount > 0 {
		Tasks = make([]*Task, tasksCount)
		for i, c := 0, int('a'); i < len(Tasks); i++ {
			if randomizePaths {
				c++
				if c > 127 {
					c = int('a')
				}
			}
			Tasks[i] = &Task{
				URL: fmt.Sprintf("%s/%s", serverURL, string(c)),
			}
		}
	}

	return Tasks
}

func shortSender(ctx context.Context, task *Task) *WorkerError {
	time.Sleep(10 * time.Millisecond)
	atomic.AddInt32(&shortSenderCallsCount, 1)
	return nil
}

func TestDispatchAdaptive(t *testing.T) {
	shortSenderCallsCount = 0
	tasksCount := 20
	minWorkers := 0
	maxWorkers := 40
	timeout := 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender),
		FailFast:   true,
	}

	t0 := time.Now()
	if err := job.Dispatch(ctx, newTasksList(tasksCount, "", false)); err != nil {
		t.Errorf("%v", err)
	}
	processingDuration := time.Now().Sub(t0)
	t.Logf("\nProcess duration: %s\n", processingDuration.String())
	assert.Equal(t, tasksCount, int(shortSenderCallsCount))
}

func TestDispatchStrict(t *testing.T) {
	shortSenderCallsCount = 0
	tasksCount := 10
	minWorkers := 10
	maxWorkers := 10
	timeout := 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender),
		FailFast:   true,
	}

	t0 := time.Now()
	if err := job.Dispatch(ctx, newTasksList(tasksCount, "", false)); err != nil {
		t.Errorf("%v", err)
	}
	processingDuration := time.Now().Sub(t0)
	t.Logf("\nProcess duration: %s\n", processingDuration.String())
	assert.Equal(t, tasksCount, int(shortSenderCallsCount))
}

func TestDispatchNoWorkers(t *testing.T) {
	shortSenderCallsCount = 0
	tasksCount := 10
	minWorkers := 0
	maxWorkers := 0
	timeout := 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender),
		FailFast:   true,
	}

	err := job.Dispatch(ctx, newTasksList(tasksCount, "", false))

	assert.NotNil(t, err)
	assert.Equal(t, context.DeadlineExceeded, err.error)
	assert.Equal(t, 0, int(shortSenderCallsCount))
}

func TestDispatchWrongWorkersRange(t *testing.T) {
	shortSenderCallsCount = 0
	tasksCount := 10
	minWorkers := 10
	maxWorkers := 0
	timeout := 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender),
		FailFast:   true,
	}

	defer func(t *testing.T, shortSenderCallsCount int32) {
		assert.Equal(t, 0, int(shortSenderCallsCount))
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}(t, shortSenderCallsCount)

	job.Dispatch(ctx, newTasksList(tasksCount, "", false))
}

func TestDispatchCtxTimeout(t *testing.T) {
	shortSenderCallsCount = 0
	tasksCount := 400
	minWorkers := 0
	maxWorkers := 1
	timeout := 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender),
		FailFast:   true,
	}

	var actualError = job.Dispatch(ctx, newTasksList(tasksCount, "", false))

	assert.NotNil(t, actualError)
	assert.Equal(t, newerror(context.DeadlineExceeded, 0), actualError)
	assert.NotEqual(t, tasksCount, int(atomic.LoadInt32(&shortSenderCallsCount)))
}

func TestDispatchCtxCancel(t *testing.T) {
	shortSenderCallsCount = 0
	tasksCount := 400
	minWorkers := 0
	maxWorkers := 1
	timeout := 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender),
		FailFast:   true,
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	var actualError = job.Dispatch(ctx, newTasksList(tasksCount, "", false))

	assert.NotNil(t, actualError)
	assert.Equal(t, newerror(context.Canceled, 0), actualError)
	assert.NotEqual(t, tasksCount, int(atomic.LoadInt32(&shortSenderCallsCount)))
}

var expectedError = newerror(errors.New("test"), 123)

var faultySenderCallsCount uint32
var totalCallsCount uint32

func faultySender(ctx context.Context, task *Task) *WorkerError {
	time.Sleep(50 * time.Millisecond)
	atomic.AddUint32(&totalCallsCount, 1)
	atomic.AddUint32(&faultySenderCallsCount, 1)
	count := int(atomic.LoadUint32(&faultySenderCallsCount))
	if count == 3 || count == 5 || count == 8 {
		return expectedError
	}
	return nil
}

func TestDispatchError(t *testing.T) {
	tasksCount := 10
	minWorkers := 0
	maxWorkers := 2
	timeout := 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(faultySender),
		FailFast:   true,
	}

	actualError := job.Dispatch(ctx, newTasksList(tasksCount, "", false))

	assert.NotNil(t, actualError)
	assert.Equal(t, expectedError, actualError)
	//Did we fail fast
	actualCallCount := int(atomic.LoadUint32(&totalCallsCount))
	assert.True(t, actualCallCount < tasksCount)
	atomic.StoreUint32(&totalCallsCount, 0)
}

func TestDispatchFaultTolerantOnError(t *testing.T) {
	tasksCount := 10
	minWorkers := 0
	maxWorkers := 4
	timeout := 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(faultySender),
		FailFast:   false,
	}

	actualError := job.Dispatch(ctx, newTasksList(tasksCount, "", false))

	assert.NotNil(t, actualError)
	if actualError != nil {
		assert.NotNil(t, actualError.error)
		if merr, ok := actualError.error.(*multierror.Error); ok {
			assert.True(t, merr.Len() == 1)
			assert.Equal(t, merr.Errors[0], expectedError)
		}
	}
	assert.True(t, int(atomic.LoadUint32(&totalCallsCount)) == tasksCount)
	atomic.StoreUint32(&totalCallsCount, 0)
}

func TestClientMetering(t *testing.T) {
	tasksCount := 4
	minWorkers := 4
	maxWorkers := 4
	timeout := 60 * 60 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	backendService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		time.Sleep(50 * time.Millisecond)
		w.Write([]byte("123"))
	}))
	defer backendService.Close()
	var job = &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker: &BackendWorker{
			MaxSizeResponseBody: units.MB,
		},
	}

	reg := prometheus.NewRegistry()
	RegisterMetrics(true, reg)
	inputs := newTasksList(tasksCount, backendService.URL, true)
	if err := job.Dispatch(ctx, inputs); err != nil {
		t.Errorf("%v", err)
	}

	if mfs, err := reg.Gather(); assert.NoError(t, err) {
		metricsMap := make(map[string]interface{})
		for _, mf := range mfs {
			metricsMap[mf.GetName()] = mf.GetMetric()
		}
		for _, tc := range []struct {
			name       string
			assertions func(string, []*io_prometheus_client.Metric)
		}{
			{
				name: metrics.Namespace + "_client_api_requests_total",
				assertions: func(metricName string, samples []*io_prometheus_client.Metric) {
					assert.Lenf(t, samples, 1, "unexpected number of metric families `%s` gathered", metricName)
					assert.True(t, samples[0].Counter.GetValue() == 4)
				},
			}, {
				name:       metrics.Namespace + "_client_in_flight_requests",
				assertions: nil,
			}, {
				name: metrics.Namespace + "_request_duration_seconds",
				assertions: func(metricName string, samples []*io_prometheus_client.Metric) {
					assert.Lenf(t, samples, 1, "unexpected number of metric families `%s` gathered", metricName)
					assert.True(t, samples[0].Histogram.GetSampleCount() == 4)
				},
			},
		} {
			assert.Containsf(t, metricsMap, tc.name, "expected metric `%s` not registered", tc.name)
			if tc.assertions != nil {
				tc.assertions(tc.name, metricsMap[tc.name].([]*io_prometheus_client.Metric))
			}
		}
	}
}

// BackendWorker tests
func TestWorker(t *testing.T) {
	var (
		actual               bool
		err                  error
		backendRequestsCount int
	)
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		backendRequestsCount++
		defer r.Body.Close()
		if _, err = ioutil.ReadAll(r.Body); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		actual = true
		w.Write([]byte("123"))
	}))
	defer backend.Close()
	w := &BackendWorker{
		URL:                 backend.URL,
		MaxSizeResponseBody: units.KB,
	}
	input := &Task{
		URL: backend.URL,
	}

	workerError := w.Work(context.Background(), input)

	assert.Nil(t, err)
	assert.Nil(t, workerError)
	assert.True(t, actual)
	assert.Equal(t, 1, backendRequestsCount)
}

func TestWorkerResponseTooLarge(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("123"))
	}))
	defer backend.Close()
	w := &BackendWorker{
		URL:                 backend.URL,
		MaxSizeResponseBody: 0,
	}

	err := w.Work(context.Background(), &Task{
		URL: backend.URL,
	})

	assert.NotNil(t, err)
	assert.Equal(t, fmt.Sprintf("reading response from Task resource %s failed: response body too large", backend.URL), err.Error())
}

func TestWorkerResponseFault(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer backend.Close()
	w := &BackendWorker{
		URL:                 backend.URL,
		MaxSizeResponseBody: units.KB,
	}

	err := w.Work(context.Background(), &Task{
		URL: backend.URL,
	})

	assert.NotNil(t, err)
	assert.Equal(t, fmt.Sprintf("sending Task to resource %s failed with response code 500", backend.URL), err.Error())
}

func TestWorkerCtxTimeout(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
	}))
	defer backend.Close()
	w := &BackendWorker{
		URL:                 backend.URL,
		MaxSizeResponseBody: units.KB,
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := w.Work(ctx, &Task{
		URL: backend.URL,
	})

	assert.NotNil(t, err)
	assert.Equal(t, fmt.Sprintf("Get %q: context deadline exceeded", backend.URL), err.Error())
}

func TestWorkerCtxCancel(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(250 * time.Millisecond)
	}))
	defer backend.Close()
	w := &BackendWorker{
		URL:                 backend.URL,
		MaxSizeResponseBody: units.KB,
	}
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	err := w.Work(ctx, &Task{
		URL: backend.URL,
	})

	assert.NotNil(t, err)
	assert.Equal(t, fmt.Sprintf("Get %q: context canceled", backend.URL), err.Error())
}

package reactor

// import (
// 	"context"
// 	"io/ioutil"
// 	"net/http"
// 	"net/http/httptest"
// 	"testing"
// 	"time"

// 	"github.com/gardener/docode/pkg/metrics"
// 	"github.com/prometheus/client_golang/prometheus"
// 	io_prometheus_client "github.com/prometheus/client_model/go"
// 	"github.com/stretchr/testify/assert"
// )

// func TestClientMetering(t *testing.T) {
// 	tasksCount := 4
// 	minWorkers := 4
// 	maxWorkers := 4
// 	timeout := 60 * 60 * time.Second

// 	ctx, cancel := context.WithTimeout(context.Background(), timeout)
// 	defer cancel()
// 	backendService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		defer r.Body.Close()
// 		time.Sleep(50 * time.Millisecond)
// 		w.Write([]byte("123"))
// 	}))
// 	defer backendService.Close()
// 	var job = &Job{
// 		MinWorkers: minWorkers,
// 		MaxWorkers: maxWorkers,
// 		Worker: WorkerFunc(func(ctx context.Context, tasks interface{}) *WorkerError {
// 			return nil
// 		}),
// 	}

// 	reg := prometheus.NewRegistry()
// 	RegisterMetrics(reg)
// 	inputs := newTasksList(tasksCount, backendService.URL, true)
// 	if err := job.Dispatch(ctx, inputs); err != nil {
// 		t.Errorf("%v", err)
// 	}

// 	if mfs, err := reg.Gather(); assert.NoError(t, err) {
// 		metricsMap := make(map[string]interface{})
// 		for _, mf := range mfs {
// 			metricsMap[mf.GetName()] = mf.GetMetric()
// 		}
// 		for _, tc := range []struct {
// 			name       string
// 			assertions func(string, []*io_prometheus_client.Metric)
// 		}{
// 			{
// 				name: metrics.Namespace + "_client_api_requests_total",
// 				assertions: func(metricName string, samples []*io_prometheus_client.Metric) {
// 					assert.Lenf(t, samples, 1, "unexpected number of metric families `%s` gathered", metricName)
// 					assert.True(t, samples[0].Counter.GetValue() == 4)
// 				},
// 			}, {
// 				name:       metrics.Namespace + "_client_in_flight_requests",
// 				assertions: nil,
// 			}, {
// 				name: metrics.Namespace + "_request_duration_seconds",
// 				assertions: func(metricName string, samples []*io_prometheus_client.Metric) {
// 					assert.Lenf(t, samples, 1, "unexpected number of metric families `%s` gathered", metricName)
// 					assert.True(t, samples[0].Histogram.GetSampleCount() == 4)
// 				},
// 			},
// 		} {
// 			assert.Containsf(t, metricsMap, tc.name, "expected metric `%s` not registered", tc.name)
// 			if tc.assertions != nil {
// 				tc.assertions(tc.name, metricsMap[tc.name].([]*io_prometheus_client.Metric))
// 			}
// 		}
// 	}
// }

// func TestWorker(t *testing.T) {
// 	var (
// 		actual               bool
// 		err                  error
// 		backendRequestsCount int
// 	)
// 	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		backendRequestsCount++
// 		defer r.Body.Close()
// 		if _, err = ioutil.ReadAll(r.Body); err != nil {
// 			w.WriteHeader(http.StatusInternalServerError)
// 			return
// 		}
// 		actual = true
// 		w.Write([]byte("123"))
// 	}))
// 	defer backend.Close()
// 	w := &struct{}{}
// 	input := &struct{}{}

// 	workerError := w.Work(context.Background(), input)

// 	assert.Nil(t, err)
// 	assert.Nil(t, workerError)
// 	assert.True(t, actual)
// 	assert.Equal(t, 1, backendRequestsCount)
// }

// func TestWorkerResponseTooLarge(t *testing.T) {
// 	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.Write([]byte("123"))
// 	}))
// 	defer backend.Close()
// 	w := &GitHubWorker{}

// 	err := w.Work(context.Background(), &GitHubTask{})

// 	assert.NotNil(t, err)
// 	assert.Equal(t, fmt.Sprintf("reading response from task resource %s failed: response body too large", backend.URL), err.Error())
// }

// func TestWorkerResponseFault(t *testing.T) {
// 	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		w.WriteHeader(http.StatusInternalServerError)
// 	}))
// 	defer backend.Close()
// 	w := &GitHubWorker{}

// 	err := w.Work(context.Background(), &GitHubTask{})

// 	assert.NotNil(t, err)
// 	assert.Equal(t, fmt.Sprintf("sending task to resource %s failed with response code 500", backend.URL), err.Error())
// }

// func TestWorkerCtxTimeout(t *testing.T) {
// 	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		time.Sleep(250 * time.Millisecond)
// 	}))
// 	defer backend.Close()
// 	w := &GitHubWorker{}
// 	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
// 	defer cancel()

// 	err := w.Work(ctx, &GitHubTask{})

// 	assert.NotNil(t, err)
// 	assert.Equal(t, fmt.Sprintf("Get %q: context deadline exceeded", backend.URL), err.Error())
// }

// func TestWorkerCtxCancel(t *testing.T) {
// 	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 		time.Sleep(250 * time.Millisecond)
// 	}))
// 	defer backend.Close()
// 	w := &GitHubWorker{}
// 	ctx, cancel := context.WithCancel(context.Background())
// 	go func() {
// 		time.Sleep(100 * time.Millisecond)
// 		cancel()
// 	}()

// 	err := w.Work(ctx, &GitHubTask{})

// 	assert.NotNil(t, err)
// 	assert.Equal(t, fmt.Sprintf("Get %q: context canceled", backend.URL), err.Error())
// }

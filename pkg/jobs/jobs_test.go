// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package jobs

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gardener/docforge/pkg/util/tests"
	"github.com/hashicorp/go-multierror"
	"github.com/stretchr/testify/assert"
)

func init() {
	tests.SetKlogV(6)
}

func newTasksList(tasksCount int, serverURL string, randomizePaths bool) []interface{} {
	var tasks []interface{}

	if tasksCount > 0 {
		tasks = make([]interface{}, tasksCount)
		for i, c := 0, int('a'); i < len(tasks); i++ {
			if randomizePaths {
				c++
				if c > 127 {
					c = int('a')
				}
			}
			tasks[i] = &struct{}{}
		}
	}

	return tasks
}

type shortSender struct {
	shortSenderCallsCount int32
}

func (s *shortSender) work(ctx context.Context, task interface{}, wq WorkQueue) *WorkerError {
	time.Sleep(10 * time.Millisecond)
	atomic.AddInt32(&s.shortSenderCallsCount, 1)
	return nil
}

func TestDispatchAdaptive(t *testing.T) {
	shortSender := &shortSender{}
	tasksCount := 20
	minWorkers := 0
	maxWorkers := 40
	timeout := 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender.work),
		FailFast:   true,
		Queue:      NewWorkQueue(tasksCount),
	}

	t0 := time.Now()
	if err := job.Dispatch(ctx, newTasksList(tasksCount, "", false)); err != nil {
		t.Errorf("%v", err)
	}
	processingDuration := time.Now().Sub(t0)
	t.Logf("\nProcess duration: %s\n", processingDuration.String())
	assert.Equal(t, tasksCount, int(shortSender.shortSenderCallsCount))
}

func TestDispatchStrict(t *testing.T) {
	shortSender := &shortSender{}
	tasksCount := 10
	minWorkers := 10
	maxWorkers := 10
	timeout := 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender.work),
		FailFast:   true,
		Queue:      NewWorkQueue(tasksCount),
	}

	t0 := time.Now()
	if err := job.Dispatch(ctx, newTasksList(tasksCount, "", false)); err != nil {
		t.Errorf("%v", err)
	}
	processingDuration := time.Now().Sub(t0)
	t.Logf("\nProcess duration: %s\n", processingDuration.String())
	assert.Equal(t, tasksCount, int(shortSender.shortSenderCallsCount))
}

func TestDispatchNoWorkers(t *testing.T) {
	shortSender := &shortSender{}
	tasksCount := 10
	minWorkers := 0
	maxWorkers := 0
	timeout := 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender.work),
		FailFast:   true,
		Queue:      NewWorkQueue(tasksCount),
	}

	err := job.Dispatch(ctx, newTasksList(tasksCount, "", false))

	assert.Nil(t, err)
	// assert.Equal(t, context.DeadlineExceeded, err.error)
	assert.Equal(t, 0, int(shortSender.shortSenderCallsCount))
}

func TestDispatchWrongWorkersRange(t *testing.T) {
	shortSender := &shortSender{}
	tasksCount := 10
	minWorkers := 10
	maxWorkers := 0
	timeout := 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender.work),
		FailFast:   true,
		Queue:      NewWorkQueue(tasksCount),
	}

	defer func(t *testing.T, shortSenderCallsCount int32) {
		assert.Equal(t, 0, int(shortSenderCallsCount))
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}(t, shortSender.shortSenderCallsCount)

	job.Dispatch(ctx, newTasksList(tasksCount, "", false))
}

func TestDispatchCtxTimeout(t *testing.T) {
	shortSender := &shortSender{}
	tasksCount := 400
	minWorkers := 0
	maxWorkers := 1
	timeout := 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender.work),
		FailFast:   true,
		Queue:      NewWorkQueue(tasksCount),
	}

	var actualError = job.Dispatch(ctx, newTasksList(tasksCount, "", false))

	assert.Nil(t, actualError)
	// assert.Equal(t, newerror(context.DeadlineExceeded, 0), actualError)
	assert.NotEqual(t, tasksCount, int(atomic.LoadInt32(&shortSender.shortSenderCallsCount)))
}

func TestDispatchCtxCancel(t *testing.T) {
	shortSender := &shortSender{}
	tasksCount := 400
	minWorkers := 0
	maxWorkers := 1
	timeout := 5 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender.work),
		FailFast:   true,
		Queue:      NewWorkQueue(tasksCount),
	}
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	var actualError = job.Dispatch(ctx, newTasksList(tasksCount, "", false))

	assert.Nil(t, actualError)
	// assert.Equal(t, newerror(context.Canceled, 0), actualError)
	assert.NotEqual(t, tasksCount, int(atomic.LoadInt32(&shortSender.shortSenderCallsCount)))
}

func TestDispatchLateTasksAddition(t *testing.T) {
	shortSender := &shortSender{}
	tasksCount := 1
	minWorkers := 1
	maxWorkers := 1
	timeout := 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(shortSender.work),
		FailFast:   true,
		Queue:      NewWorkQueue(tasksCount),
	}

	var (
		actualError *WorkerError
		wg          sync.WaitGroup
	)

	wg.Add(1)
	go func() {
		actualError = job.Dispatch(ctx, make([]interface{}, 0))
		time.Sleep(50 * time.Millisecond)
		wg.Done()
	}()

	go job.Queue.Add(struct{}{})

	wg.Wait()

	assert.Nil(t, actualError)
	assert.Equal(t, tasksCount, int(atomic.LoadInt32(&shortSender.shortSenderCallsCount)))
}

func TestDispatchWaitAfterWorkDoneTrue(t *testing.T) {
	shortSender := &shortSender{}
	tasksCount := 2
	minWorkers := 1
	maxWorkers := 1
	timeout := 100 * time.Millisecond

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	job := &Job{
		MinWorkers:                minWorkers,
		MaxWorkers:                maxWorkers,
		Worker:                    WorkerFunc(shortSender.work),
		FailFast:                  true,
		Queue:                     NewWorkQueue(tasksCount),
		IsWorkerExitsOnEmptyQueue: true,
	}

	actualError := job.Dispatch(ctx, newTasksList(tasksCount, "", false))

	assert.Error(t, actualError)
	assert.Equal(t, context.DeadlineExceeded, ctx.Err())
	assert.Equal(t, tasksCount, int(atomic.LoadInt32(&shortSender.shortSenderCallsCount)))
}

var expectedError = newerror(errors.New("test"), 123)

type faultySender struct {
	faultySenderCallsCount int32
}

func (f *faultySender) Work(ctx context.Context, task interface{}, wq WorkQueue) *WorkerError {
	time.Sleep(50 * time.Millisecond)
	count := int(atomic.AddInt32(&f.faultySenderCallsCount, 1))
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

	worker := &faultySender{}
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     worker,
		FailFast:   true,
		Queue:      NewWorkQueue(tasksCount),
	}

	actualError := job.Dispatch(ctx, newTasksList(tasksCount, "", false))

	assert.Error(t, actualError)
	assert.Equal(t, expectedError, errors.Unwrap((errors.Unwrap(actualError))))
	//Did we fail early?
	actualCallCount := int(atomic.LoadInt32(&worker.faultySenderCallsCount))
	assert.True(t, actualCallCount < tasksCount)
}

func TestDispatchFaultTolerantOnError(t *testing.T) {
	tasksCount := 10
	minWorkers := 0
	maxWorkers := 5
	timeout := 1 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	worker := &faultySender{}
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     worker,
		FailFast:   false,
		Queue:      NewWorkQueue(tasksCount),
	}
	tasks := newTasksList(tasksCount, "", true)
	actualError := job.Dispatch(ctx, tasks)

	assert.Error(t, actualError)
	if actualError != nil {
		assert.NotNil(t, actualError.error)
		if merr, ok := actualError.error.(*multierror.Error); !ok {
			assert.True(t, merr.Len() == 1)
			assert.Equal(t, merr.Errors[0], expectedError)
		}
	}
	// Are we fault tolerant?
	actualCallCount := int(atomic.LoadInt32(&worker.faultySenderCallsCount))
	assert.True(t, actualCallCount == tasksCount)
}

type taskSpawiningSender struct {
	taskSpawningSenderCallsCount int32
	addedJobs                    int32
	jobsExpectedToAdd            int32
}

func (s *taskSpawiningSender) work(ctx context.Context, task interface{}, wq WorkQueue) *WorkerError {
	//time.Sleep(10 * time.Millisecond)
	atomic.AddInt32(&s.taskSpawningSenderCallsCount, 1)
	t := newTasksList(1, "", false)
	if atomic.LoadInt32(&s.addedJobs) < atomic.LoadInt32(&s.jobsExpectedToAdd) {
		wq.Add(t[0])
		atomic.AddInt32(&s.addedJobs, 1)
	}
	return nil
}

func TestDispatchContinuous(t *testing.T) {
	tasksCount := 5
	expectedTasksCount := tasksCount * 2
	minWorkers := 2
	maxWorkers := 2
	timeout := 100 * time.Second

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	sender := &taskSpawiningSender{
		jobsExpectedToAdd: int32(tasksCount),
	}
	job := &Job{
		MinWorkers: minWorkers,
		MaxWorkers: maxWorkers,
		Worker:     WorkerFunc(sender.work),
		FailFast:   true,
		Queue:      NewWorkQueue(expectedTasksCount),
	}

	t0 := time.Now()
	if err := job.Dispatch(ctx, newTasksList(tasksCount, "", false)); err != nil {
		t.Errorf("err %v != nil", err)
	}
	processingDuration := time.Now().Sub(t0)
	t.Logf("\nProcess duration: %s\n", processingDuration.String())
	assert.Equal(t, expectedTasksCount, int(sender.taskSpawningSenderCallsCount))
}

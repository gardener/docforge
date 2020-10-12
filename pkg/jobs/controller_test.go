package jobs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestController(t *testing.T) {
	testCases := []struct {
		worker              *shortSender
		tasksCount          int
		workersCount        int
		failFast            bool
		timeout             time.Duration
		expectedWorkerCalls int
		run                 func(ctx context.Context, j *Job, c Controller)
	}{
		{
			worker:              &shortSender{},
			tasksCount:          4,
			workersCount:        1,
			failFast:            true,
			timeout:             100 * time.Millisecond,
			expectedWorkerCalls: 4,
			run: func(ctx context.Context, j *Job, c Controller) {
				errCh := make(chan error)
				shutdownCh := make(chan struct{})
				defer func() {
					close(errCh)
					close(shutdownCh)
				}()
				go c.Start(ctx, errCh, nil)
				go func() {
					for _, task := range newTasksList(4, "", false) {
						j.Queue.Add(task)
					}
					c.Stop(shutdownCh)
				}()
				select {
				case <-ctx.Done():
					return
				case <-shutdownCh:
					return
				}
			},
		},
		{
			worker:              &shortSender{},
			tasksCount:          4,
			workersCount:        1,
			failFast:            true,
			timeout:             100000 * time.Millisecond,
			expectedWorkerCalls: 4,
			run: func(ctx context.Context, j *Job, c Controller) {
				errCh := make(chan error)
				shutdownCh := make(chan struct{})
				defer func() {
					close(errCh)
					close(shutdownCh)
				}()
				go c.Start(ctx, errCh, shutdownCh)
				go func() {
					for _, task := range newTasksList(4, "", false) {
						j.Queue.Add(task)
					}
					c.Stop(nil)
				}()
				select {
				case <-ctx.Done():
					return
				case <-shutdownCh:
					return
				}
			},
		},
	}
	for _, tc := range testCases {
		t.Run("", func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), tc.timeout)
			defer cancel()
			job := &Job{
				FailFast:                  tc.failFast,
				ID:                        "Test",
				MaxWorkers:                tc.workersCount,
				MinWorkers:                tc.workersCount,
				IsWorkerExitsOnEmptyQueue: true,
				Worker:                    WorkerFunc(tc.worker.work),
				Queue:                     NewWorkQueue(tc.tasksCount),
			}

			c := NewController(job)
			tc.run(ctx, job, c)

			assert.NotNil(t, c)
			assert.True(t, tc.worker.shortSenderCallsCount == int32(tc.expectedWorkerCalls))
		})
	}
}

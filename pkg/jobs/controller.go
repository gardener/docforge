package jobs

import (
	"context"
	"sync"
)

// Controller is the functional interface of worker controllers for work queue.
// It captures a controller lifecycle from its start (Start), through adding
// tasks for workers (Enqueue) to its immediate (Shutdown) or gracefull end (Stop).
type Controller interface {
	// Start starts a controller that reports errors on errCh and
	// optionally stopped status on shutdownCh if the channel is provided.
	// The controller blocks waiting on tasks in its queue until
	// interrupted by context (ctx) or a Shutdown/Stop function call.
	Start(ctx context.Context, errCh chan<- error, shutdownCh chan struct{})
	// Enqueue adds a task to this controller's queue for workers to
	// process. The function is non-blocking and can be interrupted by
	// context.
	Enqueue(ctx context.Context, task interface{})
	// Shutdown will singal a started controller to quit waiting continuously
	// for task and work on tasks until its queue is drained, then exit.
	// The shutdownCh parameter is an optional channel to notify when
	// shutdown is complete.
	Stop(shutdownCh chan struct{})
	// Stops the controller and its workers, regardless of whether there
	// are tasks in the queue waiting processing.
	Shutdown()
}

type controller struct {
	*Job
	shutdownChs []chan struct{}
}

// NewController creates new Controller instances
func NewController(job *Job) Controller {
	job.IsWorkerExitsOnEmptyQueue = true
	c := &controller{
		Job: job,
	}
	return c

}

func (c *controller) Start(ctx context.Context, errCh chan<- error, shutdownCh chan struct{}) {
	var wg sync.WaitGroup
	c.registerShutdownChannel(shutdownCh)

	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := c.Dispatch(ctx, make([]interface{}, 0)); err != nil {
			errCh <- err
		}
	}()

	wg.Wait()

	if len(c.shutdownChs) > 0 {
		for _, ch := range c.shutdownChs {
			go func(ch chan struct{}) {
				ch <- struct{}{}
			}(ch)
		}
	}
}

func (c *controller) Enqueue(ctx context.Context, task interface{}) {
	go func() {
		select {
		case <-ctx.Done():
			return
		default:
			{
				c.Queue.Add(task)
			}
		}
	}()
}

func (c *controller) Stop(shutdownCh chan struct{}) {
	defer func() {
		c.IsWorkerExitsOnEmptyQueue = false
	}()
	c.registerShutdownChannel(shutdownCh)
}

func (c *controller) Shutdown() {
	c.Queue.Stop()
}

func (c *controller) registerShutdownChannel(shutdownCh chan struct{}) {
	if shutdownCh == nil {
		return
	}
	if len(c.shutdownChs) < 1 {
		c.shutdownChs = []chan struct{}{shutdownCh}
		return
	}
	c.shutdownChs = append(c.shutdownChs, shutdownCh)
}

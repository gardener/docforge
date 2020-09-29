package jobs

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/gardener/docforge/pkg/util/tests"
)

func init() {
	tests.SetGlogV(6)
}

func Test(t *testing.T) {
	wq := NewWorkQueue(1)
	timeout := 1 * time.Second
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// start 2 parallel workers
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		fmt.Printf("Spawning worker %d\n", i)
		idx := i
		go func() {
			defer fmt.Printf("Worker %d stopped\n", idx)
			wg.Done()
			fmt.Printf("Worker %d spawned\n", idx)
			var additionalAdded bool
			for {
				var (
					task interface{}
				)
				if task = wq.Get(); task == nil {
					return
				}
				fmt.Printf("Work done by worker %d %v\n", idx, task)
				if !additionalAdded {
					wq.Add(struct{}{})
					additionalAdded = true
				}
			}
		}()
	}
	//wait until workers started
	wg.Wait()

	// parallel production of 5 tasks
	go func() {
		for i := 0; i < 5; i++ {
			wq.Add(struct{}{})
		}
	}()

	// wait job done or timeout
	ticker := time.NewTicker(10 * time.Millisecond)
	for {
		select {
		case <-ctx.Done():
			{
				fmt.Printf("ctx Done received\n")
				return
			}
		case <-ticker.C:
			{
				if wq.(*workQueue).Count() < 1 {
					if stopped := wq.Stop(); stopped {
						fmt.Printf("Stopped\n")
						return
					}
				}
			}
		}
	}
}

package jobs

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/gardener/docforge/pkg/util/tests"
	klog "k8s.io/klog/v2"
)

func init() {
	tests.SetKlogV(6)
}

func TestConcurrentStop(t *testing.T) {
	wq := NewWorkQueue(1)
	var wg sync.WaitGroup
	wg.Add(100)
	for i := 0; i < 100; i++ {
		go func() {
			time.Sleep(10 * time.Millisecond)
			wq.Stop()
			wg.Done()
		}()
	}
	wg.Wait()
}

func TestConcurrentAddGet(t *testing.T) {
	wq := NewWorkQueue(1)
	var wg sync.WaitGroup
	wg.Add(2000)
	go func() {
		for i := 0; i < 1000; i++ {
			go func() {
				time.Sleep(10 * time.Millisecond)
				wq.Add(struct{}{})
				wg.Done()
			}()
		}
	}()
	go func() {
		for i := 0; i < 1000; i++ {
			go func() {
				time.Sleep(10 * time.Millisecond)
				wq.Get()
				wg.Done()
			}()
		}
	}()
	wg.Wait()
}

func TestConcurrentAddGetStop(t *testing.T) {
	wq := NewWorkQueue(1)
	var wg sync.WaitGroup
	wg.Add(2000)
	go func() {
		for i := 0; i < 1000; i++ {
			go func() {
				time.Sleep(10 * time.Millisecond)
				wq.Add(struct{}{})
				wg.Done()
			}()
		}
	}()
	go func() {
		time.Sleep(100 * time.Millisecond)
		wq.Stop()
	}()
	go func() {
		for i := 0; i < 1000; i++ {
			go func() {
				time.Sleep(10 * time.Millisecond)
				wq.Get()
				wg.Done()
			}()
		}
	}()
	wg.Wait()
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
		klog.V(6).Infof("Spawning worker %d\n", i)
		idx := i
		go func() {
			defer klog.V(6).Infof("Worker %d stopped\n", idx)
			wg.Done()
			klog.V(6).Infof("Worker %d spawned\n", idx)
			var additionalAdded bool
			for {
				var (
					task interface{}
				)
				if task = wq.Get(); task == nil {
					return
				}
				klog.V(6).Infof("Work done by worker %d %v\n", idx, task)
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
				klog.V(6).Infoln("ctx Done received")
				return
			}
		case <-ticker.C:
			{
				if wq.(*workQueue).Count() < 1 {
					if stopped := wq.Stop(); stopped {
						klog.V(6).Infoln("Stopped")
						return
					}
				}
			}
		}
	}
}

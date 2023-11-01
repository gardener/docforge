// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package jobs_test

import (
	"context"
	"errors"
	"sync"

	"github.com/gardener/docforge/pkg/reactor/jobs"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type task struct{}

var _ = Describe("Jobs", func() {
	var (
		size     int
		failFast bool
		worker   jobs.WorkerFunc
		wg       *sync.WaitGroup
		ctx      context.Context
		queue    *jobs.JobQueue
		err      error
	)
	BeforeEach(func() {
		size = 2
		failFast = false
		worker = func(ctx context.Context, task interface{}) error {
			if task == nil {
				return errors.New("task is nil")
			}
			return nil
		}
		wg = &sync.WaitGroup{}
		ctx = context.Background()
	})
	JustBeforeEach(func() {
		queue, err = jobs.NewJobQueue("TestQueue", size, worker, failFast, wg)
	})
	When("creating new JobQueue", func() {
		It("creates a job queue", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(queue).NotTo(BeNil())
		})
		Context("workers size is invalid", func() {
			BeforeEach(func() {
				size = 101
			})
			It("should error", func() {
				Expect(queue).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("101"))
			})
		})
		Context("worker func not set", func() {
			BeforeEach(func() {
				worker = nil
			})
			It("should error", func() {
				Expect(queue).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("worker func is nil"))
			})
		})
		Context("wait group not set", func() {
			BeforeEach(func() {
				wg = nil
			})
			It("should error", func() {
				Expect(queue).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("wait group is nil"))
			})
		})
	})
	When("adding tasks to not started JobQueue", func() {
		JustBeforeEach(func() {
			Expect(queue.AddTask(struct{}{})).To(BeTrue())
			Expect(queue.AddTask(nil)).To(BeTrue())
			Expect(queue.AddTask(&task{})).To(BeTrue())
		})
		It("buffers the tasks for execution", func() {
			Expect(queue.GetWaitingTasksCount()).To(Equal(3))
			Expect(queue.GetProcessedTasksCount()).To(Equal(0))
		})
	})
	When("adding tasks to started JobQueue", func() {
		JustBeforeEach(func() {
			queue.Start(ctx)
			Expect(queue.AddTask(struct{}{})).To(BeTrue())
			Expect(queue.AddTask(nil)).To(BeTrue())
			Expect(queue.AddTask(&task{})).To(BeTrue())
			wg.Wait()
		})
		It("process the tasks for execution", func() {
			Expect(queue.GetProcessedTasksCount()).To(Equal(3))
			Expect(queue.GetWaitingTasksCount()).To(Equal(0))
		})
		It("reports errors during task processing", func() {
			Expect(queue.GetErrorList()).NotTo(BeNil())
			Expect(queue.GetErrorList().Unwrap()).To(Equal(errors.New("task is nil")))
		})
	})
	When("adding tasks to stopped JobQueue", func() {
		JustBeforeEach(func() {
			queue.Start(context.Background())
			queue.Stop()
		})
		It("skips the tasks", func() {
			Expect(queue.AddTask(struct{}{})).To(BeFalse())
			Expect(queue.AddTask(nil)).To(BeFalse())
			Expect(queue.AddTask(&task{})).To(BeFalse())
		})
	})
	When("fail fast strategy is set", func() {
		BeforeEach(func() {
			failFast = true
		})
		JustBeforeEach(func() {
			Expect(queue.AddTask(nil)).To(BeTrue())
			queue.Start(ctx)
			wg.Wait()
		})
		It("skips the tasks after first error", func() {
			Expect(queue.GetProcessedTasksCount()).To(Equal(1))
			Expect(queue.AddTask(struct{}{})).To(BeFalse())
			Expect(queue.AddTask(&task{})).To(BeFalse())
			Expect(queue.GetErrorList().Unwrap()).To(Equal(errors.New("task is nil")))
		})
	})
	When("workers context is canceled", func() {
		var done context.CancelFunc
		BeforeEach(func() {
			ctx, done = context.WithCancel(context.Background())
		})
		JustBeforeEach(func() {
			queue.Start(ctx)
			done()
		})
		It("skips the tasks after context cancellation", func() {
			Eventually(func() bool {
				return queue.AddTask(struct{}{})
			}).Should(BeFalse())
			Expect(queue.AddTask(nil)).To(BeFalse())
			Expect(queue.AddTask(&task{})).To(BeFalse())
		})
	})
	When("worker func panics", func() {
		BeforeEach(func() {
			worker = func(ctx context.Context, task interface{}) error {
				if task == nil {
					panic("task is nil")
				}
				return nil
			}
		})
		JustBeforeEach(func() {
			queue.Start(ctx)
			Expect(queue.AddTask(struct{}{})).To(BeTrue())
			Expect(queue.AddTask(nil)).To(BeTrue())
			wg.Wait()
		})
		It("recovers the panic and reports an error", func() {
			Expect(queue.GetProcessedTasksCount()).To(Equal(2))
			Expect(queue.GetErrorList()).NotTo(BeNil())
			Expect(queue.GetErrorList().Error()).To(ContainSubstring("task is nil"))
		})
	})
})

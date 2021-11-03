// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package reactor_test

import (
	"context"
	"errors"
	"github.com/gardener/docforge/pkg/jobs"
	"github.com/gardener/docforge/pkg/reactor"
	"github.com/gardener/docforge/pkg/reactor/reactorfakes"
	"github.com/gardener/docforge/pkg/resourcehandlers"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"sync"
)

var _ = Describe("Downloader", func() {
	var (
		err    error
		reader *reactorfakes.FakeReader
		writer *writersfakes.FakeWriter
		work   jobs.WorkerFunc
	)
	BeforeEach(func() {
		reader = &reactorfakes.FakeReader{}
		writer = &writersfakes.FakeWriter{}
	})
	JustBeforeEach(func() {
		work, err = reactor.DownloadWorkFunc(reader, writer)
	})
	When("creating worker function", func() {
		It("creates the work func successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(work).NotTo(BeNil())
		})
		Context("reader is not set", func() {
			BeforeEach(func() {
				reader = nil
			})
			It("should fails", func() {
				Expect(work).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("reader is nil"))
			})
		})
		Context("writer is not set", func() {
			BeforeEach(func() {
				writer = nil
			})
			It("should fails", func() {
				Expect(work).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("writer is nil"))
			})
		})
		When("invokes work func", func() {
			var (
				ctx  context.Context
				task interface{}
			)
			BeforeEach(func() {
				reader.ReadReturns([]byte("content"), nil)
				writer.WriteReturns(nil)
				ctx = context.Background()
				task = &reactor.DownloadTask{
					Source:    "fake_source",
					Target:    "fake_target",
					Referer:   "fake_referer",
					Reference: "fake_reference",
				}
			})
			JustBeforeEach(func() {
				Expect(work).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				err = work(ctx, task)
			})
			It("succeeded", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(reader.ReadCallCount()).To(Equal(1))
				c, source := reader.ReadArgsForCall(0)
				Expect(c).To(Equal(ctx))
				Expect(source).To(Equal("fake_source"))
				Expect(writer.WriteCallCount()).To(Equal(1))
				name, path, content, node := writer.WriteArgsForCall(0)
				Expect(node).To(BeNil())
				Expect(path).To(Equal(""))
				Expect(name).To(Equal("fake_target"))
				Expect(string(content)).To(Equal("content"))
			})
			Context("task is invalid", func() {
				BeforeEach(func() {
					task = struct{}{}
				})
				It("fails", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("incorrect download task"))
				})
			})
			Context("task is nil", func() {
				BeforeEach(func() {
					task = nil
				})
				It("fails", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("incorrect download task"))
				})
			})
			Context("read fails", func() {
				BeforeEach(func() {
					reader.ReadReturns(nil, errors.New("fake_read_err"))
				})
				It("fails", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake_read_err"))
				})
			})
			Context("read fails with resource not found", func() {
				BeforeEach(func() {
					reader.ReadReturns(nil, resourcehandlers.ErrResourceNotFound("fake_target"))
				})
				It("succeeded", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(reader.ReadCallCount()).To(Equal(1))
					Expect(writer.WriteCallCount()).To(Equal(0))
				})
			})
			Context("write fails", func() {
				BeforeEach(func() {
					writer.WriteReturns(errors.New("fake_write_err"))
				})
				It("fails", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("fake_write_err"))
				})
			})
			When("source is already downloaded", func() {
				JustBeforeEach(func() {
					Expect(err).NotTo(HaveOccurred())
					err = work(ctx, task)
				})
				It("skips duplicate downloads", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(reader.ReadCallCount()).To(Equal(1))
					Expect(writer.WriteCallCount()).To(Equal(1))
				})
			})
		})
		When("create new Download Scheduler", func() {
			var (
				wg            *sync.WaitGroup
				ctx           context.Context
				downloadTasks *jobs.JobQueue
				scheduler     reactor.DownloadScheduler
			)
			BeforeEach(func() {
				wg = &sync.WaitGroup{}
				ctx = context.Background()
			})
			JustBeforeEach(func() {
				Expect(work).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				downloadTasks, err = jobs.NewJobQueue("Download", 2, work, false, wg)
				scheduler = reactor.NewDownloadScheduler(downloadTasks)
			})
			It("succeeded", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(downloadTasks).NotTo(BeNil())
				Expect(scheduler).NotTo(BeNil())
			})
			When("scheduling download tasks", func() {
				JustBeforeEach(func() {
					downloadTasks.Start(ctx)
					Expect(scheduler.Schedule(&reactor.DownloadTask{Source: "source1"})).To(Succeed())
					Expect(scheduler.Schedule(&reactor.DownloadTask{Source: "source2"})).To(Succeed())
					Expect(scheduler.Schedule(&reactor.DownloadTask{Source: "source3"})).To(Succeed())
				})
				It("executes the tasks successfully", func() {
					wg.Wait()
					Expect(downloadTasks.GetProcessedTasksCount()).To(Equal(3))
					Expect(downloadTasks.GetErrorList()).To(BeNil())
					Expect(reader.ReadCallCount()).To(Equal(3))
					Expect(writer.WriteCallCount()).To(Equal(3))
				})
				Context("scheduling download task not possible", func() {
					JustBeforeEach(func() {
						wg.Wait()
						downloadTasks.Stop()
						err = scheduler.Schedule(&reactor.DownloadTask{Reference: "fake_reference"})
					})
					It("should error", func() {
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("fake_reference"))
					})
				})
			})
		})
	})
})

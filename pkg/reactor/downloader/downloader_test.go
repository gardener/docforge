// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package downloader_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/gardener/docforge/pkg/document"
	"github.com/gardener/docforge/pkg/reactor/downloader"
	"github.com/gardener/docforge/pkg/reactor/jobs"
	"github.com/gardener/docforge/pkg/reactor/reactorfakes"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Downloader Suite")
}

var _ = Describe("Downloader", func() {
	var (
		err    error
		reader *reactorfakes.FakeReader
		writer *writersfakes.FakeWriter
		worker *downloader.DownloadWorker
	)
	BeforeEach(func() {
		reader = &reactorfakes.FakeReader{}
		writer = &writersfakes.FakeWriter{}
	})
	JustBeforeEach(func() {
		worker, err = downloader.NewDownloader(reader, writer)
	})
	When("creating worker function", func() {
		It("creates the work func successfully", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(worker).NotTo(BeNil())
		})
		Context("reader is not set", func() {
			BeforeEach(func() {
				reader = nil
			})
			It("should fails", func() {
				Expect(worker).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("reader is nil"))
			})
		})
		Context("writer is not set", func() {
			BeforeEach(func() {
				writer = nil
			})
			It("should fails", func() {
				Expect(worker).To(BeNil())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("writer is nil"))
			})
		})
		When("invokes work func", func() {
			var (
				ctx       context.Context
				Source    string
				Target    string
				Referer   string
				Reference string
			)
			BeforeEach(func() {
				reader.ReadReturns([]byte("content"), nil)
				writer.WriteReturns(nil)
				ctx = context.Background()
				Source = "fake_source"
				Target = "fake_target"
				Referer = "fake_referer"
				Reference = "fake_reference"

			})
			JustBeforeEach(func() {
				Expect(worker).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				err = worker.Download(ctx, Source, Target, Referer, Reference)
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
					reader.ReadReturns(nil, repositoryhosts.ErrResourceNotFound("fake_target"))
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
					err = worker.Download(ctx, Source, Target, Referer, Reference)
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
				downloadTasks jobs.QueueController
				scheduler     document.DownloadScheduler
			)
			BeforeEach(func() {
				wg = &sync.WaitGroup{}
				ctx = context.Background()
			})
			JustBeforeEach(func() {
				Expect(worker).NotTo(BeNil())
				Expect(err).NotTo(HaveOccurred())
				scheduler, downloadTasks, err = downloader.New(2, false, wg, reader, writer)
			})
			It("succeeded", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(downloadTasks).NotTo(BeNil())
				Expect(scheduler).NotTo(BeNil())
			})
			When("scheduling download tasks", func() {
				JustBeforeEach(func() {
					downloadTasks.Start(ctx)
					Expect(scheduler.Schedule("source1", "", "", "")).To(Succeed())
					Expect(scheduler.Schedule("source2", "", "", "")).To(Succeed())
					Expect(scheduler.Schedule("source3", "", "", "")).To(Succeed())
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
						err = scheduler.Schedule("", "", "", "fake_reference")
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

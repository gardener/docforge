// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package downloader_test

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts/repositoryhostsfakes"
	"github.com/gardener/docforge/pkg/workers/downloader"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Downloader Suite")
}

var _ = Describe("Executing Download", func() {
	var (
		err      error
		registry *repositoryhostsfakes.FakeRegistry
		repoHost *repositoryhostsfakes.FakeRepositoryHost
		writer   *writersfakes.FakeWriter
		worker   *downloader.DownloadWorker

		ctx      context.Context
		source   string
		target   string
		document string
	)
	BeforeEach(func() {
		writer = &writersfakes.FakeWriter{}
		registry = &repositoryhostsfakes.FakeRegistry{}
		repoHost = &repositoryhostsfakes.FakeRepositoryHost{}

		registry.GetCalls(func(s string) (repositoryhosts.RepositoryHost, error) {
			if strings.HasPrefix(s, "repoHost:") {
				return repoHost, nil
			}
			return nil, fmt.Errorf("no sutiable repository host for %s", s)
		})
		repoHost.ReadReturns([]byte("content"), nil)
		writer.WriteReturns(nil)
		ctx = context.Background()
		source = "repoHost://fake_source"
		target = "fake_target"
		document = "fake_document"
	})
	JustBeforeEach(func() {
		worker, err = downloader.NewDownloader(registry, writer)
		Expect(worker).NotTo(BeNil())
		Expect(err).NotTo(HaveOccurred())

		err = worker.Download(ctx, source, target, document)
	})
	Context("source is already downloaded", func() {
		JustBeforeEach(func() {
			Expect(err).NotTo(HaveOccurred())
			err = worker.Download(ctx, source, target, document)
		})
		It("skips duplicate downloads", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(writer.WriteCallCount()).To(Equal(1))
		})
	})
	Context("no repo host for source repoHost2://fake_source", func() {
		BeforeEach(func() {
			source = "repoHost2://fake_source"
		})
		It("fails", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no sutiable repository host for repoHost2://fake_source"))
		})
	})
	Context("read fails", func() {
		BeforeEach(func() {
			repoHost.ReadReturns(nil, errors.New("fake_read_err"))
		})
		It("fails", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("fake_read_err"))
		})
	})
	Context("read fails with resource not found", func() {
		BeforeEach(func() {
			repoHost.ReadReturns(nil, repositoryhosts.ErrResourceNotFound("fake_target"))
		})
		It("succeeded", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(repoHost.ReadCallCount()).To(Equal(1))
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
	It("succeeded", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(writer.WriteCallCount()).To(Equal(1))
		name, path, content, node := writer.WriteArgsForCall(0)
		Expect(node).To(BeNil())
		Expect(path).To(Equal(""))
		Expect(name).To(Equal("fake_target"))
		Expect(string(content)).To(Equal("content"))
	})
})

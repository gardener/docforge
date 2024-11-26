// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package resourcedownloader_test

import (
	"context"
	"embed"
	_ "embed"
	"errors"
	"testing"

	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/workers/resourcedownloader"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Downloader Suite")
}

//go:embed test/*
var repo embed.FS

var _ = Describe("Executing Download", func() {
	var (
		err    error
		r      registry.Interface
		writer *writersfakes.FakeWriter
		worker *resourcedownloader.ResourceDownloadWorker

		ctx      context.Context
		source   string
		target   string
		document string
	)

	BeforeEach(func() {
		writer = &writersfakes.FakeWriter{}
		r = registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "test", []string{".md"}))
		writer.WriteReturns(nil)
		ctx = context.TODO()
		source = "https://github.com/gardener/docforge/blob/master/README.md"
		target = "fake_target"
		document = "fake_document"
	})

	JustBeforeEach(func() {
		worker, err = resourcedownloader.NewDownloader(r, writer)
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

	Context("read does not fail when resource not found", func() {
		BeforeEach(func() {
			source = "https://github.com/gardener/docforge/blob/master/Makefile"
		})
		It("succeeded", func() {
			Expect(err).To(Not(HaveOccurred()))
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
		name, path, content, node, _ := writer.WriteArgsForCall(0)
		Expect(node).To(BeNil())
		Expect(path).To(Equal(""))
		Expect(name).To(Equal("fake_target"))
		Expect(string(content)).To(Equal("readme content"))
	})
})

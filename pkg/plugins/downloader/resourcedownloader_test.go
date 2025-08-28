// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package downloader_test

import (
	"context"
	"embed"
	_ "embed"
	"errors"
	"path/filepath"
	"testing"

	"github.com/gardener/docforge/pkg/core/registry"
	"github.com/gardener/docforge/pkg/core/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/osfakes/osshim/osshimfakes"
	"github.com/gardener/docforge/pkg/plugins/downloader"
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
		err       error
		r         registry.Interface
		fakeFs    *osshimfakes.FakeOs
		fileStore map[string][]byte
		tempDir   string
		worker    *downloader.Plugin

		ctx    context.Context
		source string
		target string
	)

	BeforeEach(func() {
		// Setup fake filesystem with in-memory storage
		fakeFs = &osshimfakes.FakeOs{}
		fileStore = make(map[string][]byte)

		// Stub filesystem operations to use in-memory storage
		fakeFs.WriteFileCalls(func(path string, data []byte, perm int) error {
			fileStore[path] = data
			return nil
		})

		tempDir = "/mock/temp/dir"
		r = registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "test"))
		ctx = context.TODO()
		source = "https://github.com/gardener/docforge/blob/master/README.md"
		target = "fake_target"
	})

	JustBeforeEach(func() {
		worker = downloader.New(r, fakeFs, tempDir)
		Expect(worker).NotTo(BeNil())

		err = worker.Download(ctx, source, target)
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

	Context("write fails", func() {
		BeforeEach(func() {
			// Simulate write failure using fake filesystem
			fakeFs.WriteFileReturns(errors.New("permission denied"))
		})
		It("fails", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("permission denied"))
		})
	})

	It("succeeded", func() {
		Expect(err).NotTo(HaveOccurred())

		// Verify filesystem operations
		expectedFile := filepath.Join(tempDir, "fake_target")
		Expect(fakeFs.WriteFileCallCount()).To(Equal(1))
		writtenPath, writtenData, writtenPerm := fakeFs.WriteFileArgsForCall(0)
		Expect(writtenPath).To(Equal(expectedFile))
		Expect(string(writtenData)).To(Equal("readme content"))
		Expect(writtenPerm).To(Equal(0644))
	})
})

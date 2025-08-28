// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package downloader_test

import (
	"context"
	_ "embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/gardener/docforge/pkg/core/registry"
	"github.com/gardener/docforge/pkg/core/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/osshim/filesystem"
	"github.com/gardener/docforge/pkg/plugins/downloader"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Downloader Suite")
}

var _ = Describe("Executing Download", func() {
	var (
		err     error
		r       registry.Interface
		tempDir string
		worker  *downloader.Plugin

		ctx    context.Context
		source string
		target string
	)

	BeforeEach(func() {
		tempDir = "/tmp/docforge_test_downloader"
		r = registry.NewRegistry(repositoryhost.NewLocal("https://github.com/gardener/docforge", "test"))
		ctx = context.TODO()
		source = "https://github.com/gardener/docforge/blob/master/README.md"
		target = "fake_target"
	})

	JustBeforeEach(func() {
		worker = downloader.New(r, &filesystem.Local{}, tempDir)
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

	It("succeeded", func() {
		Expect(err).NotTo(HaveOccurred())

		// Verify the file was written correctly
		expectedFile := filepath.Join(tempDir, "fake_target")
		writtenData, err := os.ReadFile(expectedFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(writtenData)).To(Equal("readme content"))

		// Clean up test files
		os.RemoveAll(tempDir)
	})
})

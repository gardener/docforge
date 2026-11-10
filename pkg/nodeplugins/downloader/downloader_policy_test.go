// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package downloader_test

import (
	"context"

	"github.com/gardener/docforge/pkg/nodeplugins/downloader"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// dlURL returns a test registry URL for the given filename in the test/ directory.
func dlURL(filename string) string {
	return "https://github.com/gardener/docforge/blob/master/" + filename
}

var _ = Describe("Downloader — content policy", func() {
	var (
		ctx    context.Context
		r      registry.Interface
		writer *writersfakes.FakeWriter
	)

	BeforeEach(func() {
		ctx = context.TODO()
		r = registry.NewRegistry(repositoryhost.NewLocalTest(repo, "https://github.com/gardener/docforge", "test"))
		writer = &writersfakes.FakeWriter{}
	})

	// ------------------------------------------------------------------
	// Untrusted sources
	// ------------------------------------------------------------------

	Describe("Untrusted source", func() {
		It("untrusted .js → not downloaded, writer never called, no error", func() {
			d := downloader.NewDownloaderWithFlag(r, writer, false)
			Expect(d.ExecuteTask(ctx, dlURL("script.js"), "dest/script.js", false)).To(Succeed())
			Expect(writer.WriteCallCount()).To(Equal(0))
		})

		It("untrusted .JS (uppercase extension) → not downloaded", func() {
			d := downloader.NewDownloaderWithFlag(r, writer, false)
			Expect(d.ExecuteTask(ctx, dlURL("script.JS"), "dest/script.JS", false)).To(Succeed())
			Expect(writer.WriteCallCount()).To(Equal(0))
		})

		It("untrusted .vue → not downloaded", func() {
			d := downloader.NewDownloaderWithFlag(r, writer, false)
			Expect(d.ExecuteTask(ctx, dlURL("component.vue"), "dest/component.vue", false)).To(Succeed())
			Expect(writer.WriteCallCount()).To(Equal(0))
		})

		It("untrusted .html → downloaded and written unchanged (WarnNonMarkdown)", func() {
			d := downloader.NewDownloaderWithFlag(r, writer, false)
			Expect(d.ExecuteTask(ctx, dlURL("style.html"), "dest/style.html", false)).To(Succeed())
			Expect(writer.WriteCallCount()).To(Equal(1))
		})
	})

	// ------------------------------------------------------------------
	// Trusted sources
	// ------------------------------------------------------------------

	Describe("Trusted source", func() {
		It("trusted .js → downloaded and written", func() {
			d := downloader.NewDownloaderWithFlag(r, writer, false)
			Expect(d.ExecuteTask(ctx, dlURL("script.js"), "dest/script.js", true)).To(Succeed())
			Expect(writer.WriteCallCount()).To(Equal(1))
		})
	})

	// ------------------------------------------------------------------
	// --fail-on-stripped-content
	// ------------------------------------------------------------------

	Describe("--fail-on-stripped-content flag", func() {
		It("Refuse + flag → returns error, writer not called", func() {
			d := downloader.NewDownloaderWithFlag(r, writer, true)
			err := d.ExecuteTask(ctx, dlURL("script.js"), "dest/script.js", false)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("content policy"))
			Expect(writer.WriteCallCount()).To(Equal(0))
		})

		It("Refuse without flag → returns nil, writer not called", func() {
			d := downloader.NewDownloaderWithFlag(r, writer, false)
			Expect(d.ExecuteTask(ctx, dlURL("script.js"), "dest/script.js", false)).To(Succeed())
			Expect(writer.WriteCallCount()).To(Equal(0))
		})

		It("WarnNonMarkdown + flag → downloaded (no error)", func() {
			d := downloader.NewDownloaderWithFlag(r, writer, true)
			Expect(d.ExecuteTask(ctx, dlURL("style.html"), "dest/style.html", false)).To(Succeed())
			Expect(writer.WriteCallCount()).To(Equal(1))
		})
	})
})

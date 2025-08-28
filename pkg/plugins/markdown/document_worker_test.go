// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown_test

import (
	"context"
	"embed"
	"path/filepath"
	"testing"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/core/registry"
	"github.com/gardener/docforge/pkg/core/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/osfakes/osshim/osshimfakes"
	document "github.com/gardener/docforge/pkg/plugins/markdown"
	"github.com/gardener/docforge/pkg/plugins/markdown/linkresolver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Frontmatter Suite")
}

//go:embed tests/*
var manifests embed.FS

var _ = Describe("Document resolving", func() {
	var (
		dw        *document.Worker
		fakeFs    *osshimfakes.FakeOs
		fileStore map[string][]byte
		tempDir   string
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
		registry := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
		hugo := hugo.Hugo{
			Enabled:        true,
			BaseURL:        "baseURL",
			IndexFileNames: []string{"readme.md", "readme", "read.me", "index.md", "index"},
		}
		nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/docs/manifest.yaml", registry)
		Expect(err).NotTo(HaveOccurred())

		lr := linkresolver.New(nodes, registry, hugo)

		dw = document.NewDocumentWorker(lr, registry, hugo, fakeFs, tempDir, false)
	})

	Context("#ProcessNode", func() {
		It("returns correct multisource content from md and html files", func() {
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:        "renamed-document.md",
					MultiSource: []string{"https://github.com/gardener/docforge/blob/master/docs/target.md", "https://github.com/gardener/docforge/blob/master/docs/target2.md", "https://github.com/gardener/docforge/blob/master/docs/target3.html"},
				},
				Type: "file",
				Path: "one",
			}
			_, err := dw.ProcessNode(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())

			target, err := manifests.ReadFile("tests/docs/expected_target.md")
			Expect(err).NotTo(HaveOccurred())
			target2, err := manifests.ReadFile("tests/docs/expected_target2.md")
			Expect(err).NotTo(HaveOccurred())
			target3, err := manifests.ReadFile("tests/docs/expected_target3.html")
			Expect(err).NotTo(HaveOccurred())

			// Verify content and frontmatter
			expectedPath := filepath.Join(tempDir, "one", "renamed-document.md")
			expectedContent := string(target) + string(target2) + string(target3)
			Expect(node.Frontmatter["title"]).To(Equal("Renamed Document"))
			Expect(string(fileStore[expectedPath])).To(Equal(expectedContent))
		})

		It("returns correct single source content", func() {
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:   "renamed-document.md",
					Source: "https://github.com/gardener/docforge/blob/master/docs/target.md",
				},
				Type: "file",
				Path: "one",
			}
			_, err := dw.ProcessNode(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
			target, err := manifests.ReadFile("tests/docs/expected_target.md")
			Expect(err).NotTo(HaveOccurred())

			// Verify content and frontmatter
			expectedPath := filepath.Join(tempDir, "one", "renamed-document.md")
			expectedContent := string(target)
			Expect(node.Frontmatter["title"]).To(Equal("Renamed Document"))
			Expect(string(fileStore[expectedPath])).To(Equal(expectedContent))
		})

	})
})

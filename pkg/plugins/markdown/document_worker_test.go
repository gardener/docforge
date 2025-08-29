// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package markdown_test

import (
	"context"
	"embed"
	"os"
	"path/filepath"
	"testing"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/core/manifest"
	"github.com/gardener/docforge/pkg/core/registry"
	"github.com/gardener/docforge/pkg/core/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/osshim/filesystem"
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
		dw      *document.Worker
		tempDir string
	)
	BeforeEach(func() {
		tempDir = "/tmp/docforge_test_markdown"
		registry := registry.NewRegistry(repositoryhost.NewLocal("https://github.com/gardener/docforge", "tests"))
		hugo := hugo.Hugo{
			Enabled:        true,
			BaseURL:        "baseURL",
			IndexFileNames: []string{"readme.md", "readme", "read.me", "index.md", "index"},
		}
		nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/docs/manifest.yaml", registry)
		Expect(err).NotTo(HaveOccurred())

		lr := linkresolver.New(nodes, registry, hugo)

		dw = document.NewDocumentWorker(lr, registry, hugo, &filesystem.Local{}, tempDir, false)
	})

	Context("#ProcessNode", func() {
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

			// Read the actual file that was written
			writtenContent, err := os.ReadFile(expectedPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(writtenContent)).To(Equal(expectedContent))

			// Clean up test files
			os.RemoveAll(tempDir)
		})

	})
})

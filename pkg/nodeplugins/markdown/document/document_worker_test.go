// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package document_test

import (
	"context"
	"embed"
	"fmt"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkresolver"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/writers/writersfakes"
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
		dw *document.Worker

		w *writersfakes.FakeWriter
	)
	BeforeEach(func() {
		registry := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
		hugo := hugo.Hugo{
			Enabled:        true,
			BaseURL:        "baseURL",
			IndexFileNames: []string{"readme.md", "readme", "read.me", "index.md", "index"},
		}
		nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/docs/manifest.yaml", registry)
		Expect(err).NotTo(HaveOccurred())

		lr := linkresolver.New(nodes, registry, hugo)

		w = &writersfakes.FakeWriter{}
		dw = document.NewDocumentWorker(lr, registry, hugo, w)
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
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
			name, path, cnt, nodegot, _ := w.WriteArgsForCall(0)
			Expect(name).To(Equal("renamed-document.md"))
			Expect(path).To(Equal("one"))
			target, err := manifests.ReadFile("tests/docs/expected_target.md")
			Expect(err).NotTo(HaveOccurred())
			target2, err := manifests.ReadFile("tests/docs/expected_target2.md")
			fmt.Println(string(cnt))
			Expect(err).NotTo(HaveOccurred())
			target3, err := manifests.ReadFile("tests/docs/expected_target3.html")
			Expect(err).NotTo(HaveOccurred())
			Expect(node.Frontmatter["title"]).To(Equal("Renamed Document"))
			Expect(string(cnt)).To(Equal(string(target) + string(target2) + string(target3)))
			Expect(node).To(Equal(nodegot))
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
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
			name, path, cnt, nodegot, _ := w.WriteArgsForCall(0)
			Expect(name).To(Equal("renamed-document.md"))
			Expect(path).To(Equal("one"))
			target, err := manifests.ReadFile("tests/docs/expected_target.md")
			Expect(err).NotTo(HaveOccurred())
			Expect(node.Frontmatter["title"]).To(Equal("Renamed Document"))
			Expect(string(cnt)).To(Equal(string(target)))
			Expect(node).To(Equal(nodegot))
		})

	})

	Context("Fix A: root-absolute embed link on re-aggregation", func() {
		It("re-anchors a root-absolute image link to its structural-dir source and resolves it", func() {
			registry := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
			h := hugo.Hugo{
				Enabled:            true,
				BaseURL:            "baseURL",
				HugoStructuralDirs: []string{"content", "static"},
				IndexFileNames:     []string{"readme.md", "readme", "read.me", "index.md", "index"},
			}
			nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/ra/manifest.yaml", registry)
			Expect(err).NotTo(HaveOccurred())
			lr := linkresolver.New(nodes, registry, h)
			w := &writersfakes.FakeWriter{}
			dw := document.NewDocumentWorker(lr, registry, h, w)

			node := &manifest.Node{
				FileType: manifest.FileType{
					File:   "page.md",
					Source: "https://github.com/gardener/docforge/blob/master/ra/content/docs/page.md",
				},
				Type: "file",
				Path: "docs",
			}
			err = dw.ProcessNode(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
			_, _, cnt, _, _ := w.WriteArgsForCall(0)
			// Without Fix A the root-absolute link resolves against the repo root
			// (ra/images/logo.png), misses, and hard-fails. Re-anchored to the
			// content/docs source it resolves to the real blob instead.
			Expect(string(cnt)).To(ContainSubstring("ra/content/docs/images/logo.png"))
		})
	})
})

// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
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
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkvalidator/linkvalidatorfakes"
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
		vf := &linkvalidatorfakes.FakeInterface{}
		nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/docs/manifest.yaml", registry)
		Expect(err).NotTo(HaveOccurred())

		lr := linkresolver.New(nodes, registry, hugo)

		w = &writersfakes.FakeWriter{}
		dw = document.NewDocumentWorker(vf, lr, registry, hugo, w, false)
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
})

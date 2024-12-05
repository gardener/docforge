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
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/workers/document"
	"github.com/gardener/docforge/pkg/workers/linkresolver/linkresolverfakes"
	"github.com/gardener/docforge/pkg/workers/linkvalidator/linkvalidatorfakes"
	"github.com/gardener/docforge/pkg/workers/resourcedownloader/downloaderfakes"
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
		df := &downloaderfakes.FakeInterface{}
		vf := &linkvalidatorfakes.FakeInterface{}
		lrf := &linkresolverfakes.FakeInterface{}
		lrf.ResolveResourceLinkCalls(func(s1 string, n *manifest.Node, s2 string) (string, error) {
			return s1, nil
		})
		w = &writersfakes.FakeWriter{}
		dw = document.NewDocumentWorker("__resources", df, vf, lrf, registry, hugo, w, false)
	})

	Context("#ProcessNode", func() {
		It("returns correct multisource content from md and html files", func() {
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:        "node",
					MultiSource: []string{"https://github.com/gardener/docforge/blob/master/target.md", "https://github.com/gardener/docforge/blob/master/target2.md", "https://github.com/gardener/docforge/blob/master/target3.html"},
				},
				Type: "file",
				Path: "one",
			}
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
			name, path, cnt, nodegot, _ := w.WriteArgsForCall(0)
			Expect(name).To(Equal("node"))
			Expect(path).To(Equal("one"))
			target, err := manifests.ReadFile("tests/expected_target.md")
			Expect(err).NotTo(HaveOccurred())
			target2, err := manifests.ReadFile("tests/expected_target2.md")
			fmt.Println(string(cnt))
			Expect(err).NotTo(HaveOccurred())
			target3, err := manifests.ReadFile("tests/expected_target3.html")
			Expect(err).NotTo(HaveOccurred())
			Expect(string(cnt)).To(Equal(string(target) + string(target2) + string(target3)))
			Expect(node).To(Equal(nodegot))
		})

		It("returns correct single source content", func() {
			node := &manifest.Node{
				FileType: manifest.FileType{
					File:   "node",
					Source: "https://github.com/gardener/docforge/blob/master/target.md",
				},
				Type: "file",
				Path: "one",
			}
			err := dw.ProcessNode(context.TODO(), node)
			Expect(err).ToNot(HaveOccurred())
			name, path, cnt, nodegot, _ := w.WriteArgsForCall(0)
			Expect(name).To(Equal("node"))
			Expect(path).To(Equal("one"))
			target, err := manifests.ReadFile("tests/expected_target.md")
			Expect(err).NotTo(HaveOccurred())

			Expect(string(cnt)).To(Equal(string(target)))
			Expect(node).To(Equal(nodegot))
		})

	})
})

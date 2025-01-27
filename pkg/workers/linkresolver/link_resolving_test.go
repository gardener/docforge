// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package linkresolver_test

import (
	"embed"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	"github.com/gardener/docforge/pkg/workers/linkresolver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Frontmatter Suite")
}

//go:embed all:tests/*
var manifests embed.FS

var _ = Describe("Document link resolving", func() {
	Context("#ResolveResourceLink", func() {
		var (
			linkResolver linkresolver.LinkResolver
			node         *manifest.Node
			source       string
		)

		BeforeEach(func() {
			linkResolver = linkresolver.LinkResolver{}
			registry := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
			linkResolver.Repositoryhosts = registry
			linkResolver.Hugo = hugo.Hugo{
				Enabled: true,
				BaseURL: "baseURL",
			}
			linkResolver.SourceToNode = make(map[string][]*manifest.Node)
			contentFileFormats := []string{".md"}
			nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/baseline.yaml", linkResolver.Repositoryhosts, contentFileFormats)
			Expect(err).NotTo(HaveOccurred())
			for _, node := range nodes {
				if node.Source != "" {
					linkResolver.SourceToNode[node.Source] = append(linkResolver.SourceToNode[node.Source], node)
				} else if len(node.MultiSource) > 0 {
					for _, s := range node.MultiSource {
						linkResolver.SourceToNode[s] = append(linkResolver.SourceToNode[s], node)
					}
				}
			}
			source = "https://github.com/gardener/docforge/blob/master/target.md"
			node = linkResolver.SourceToNode[source][0]
		})

		It("Broken links should not return error", func() {
			newLink, err := linkResolver.ResolveResourceLink("invalidfoo/bar.md", node, source)
			Expect(err).To(Not(HaveOccurred()))
			Expect(newLink).To(Equal("https://github.com/gardener/docforge/blob/master/invalidfoo/bar.md"))
		})

		It("Resolves linking closest source correctly", func() {
			newLink, err := linkResolver.ResolveResourceLink("clickhere.md?a=b#c", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("/baseURL/one/internal/linked/?a=b#c"))
		})

		It("Resolves anchor to closes source correctly", func() {
			newLink, err := linkResolver.ResolveResourceLink("clickhere.md#anchor", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("/baseURL/one/internal/linked/#anchor"))
		})

		It("Resolves internal anchor correctly", func() {
			newLink, err := linkResolver.ResolveResourceLink("#anchor", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("/baseURL/one/node/#anchor"))
		})

		It("Resolves _index.md correctly", func() {
			newLink, err := linkResolver.ResolveResourceLink("https://github.com/gardener/docforge/blob/master/docs/_index.md", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("/baseURL/two/internal/"))
		})

		It("Resolves non-page resource links correctly", func() {
			newLink, err := linkResolver.ResolveResourceLink("./non-page.md", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("https://github.com/gardener/docforge/blob/master/non-page.md"))
		})

		It("Resolving url with no suitable repository host", func() {
			_, err := linkResolver.ResolveResourceLink("https://gitlab.com/gardener/docforge/blob/master/README.md", node, source)
			Expect(err.Error()).To(ContainSubstring("no sutiable repository host"))
		})

		Context("Resolving URL from linkResolution", func() {
			It("Resolves it correctly", func() {
				By("Node having no linkResolution should map to closest node")
				lr := node.LinkResolution
				node.LinkResolution = map[string]string{}
				newLink, err := linkResolver.ResolveResourceLink("https://github.com/gardener/docforge/blob/master/linkresolution.md", node, source)
				Expect(err).ToNot(HaveOccurred())
				Expect(newLink).To(Equal("/baseURL/one/linkresolution/"))

				By("Node having linkResolution should map to the desired node")
				node.LinkResolution = lr
				newLink, err = linkResolver.ResolveResourceLink("https://github.com/gardener/docforge/blob/master/linkresolution.md", node, source)
				Expect(err).ToNot(HaveOccurred())
				Expect(newLink).To(Equal("/baseURL/two/internal/far_linkresolution/"))
			})

			It("Resolves linkResolution correctly", func() {
				_, err := linkResolver.ResolveResourceLink("https://github.com/gardener/docforge/blob/master/linkresolution2.md", node, source)
				Expect(err.Error()).To(ContainSubstring("node with path one/node.md's LinkResolution of https://github.com/gardener/docforge/blob/master/linkresolution2.md field maps to 0 nodes"))
			})

			It("Does not change URL if there is no node with that source", func() {
				newLink, err := linkResolver.ResolveResourceLink("https://github.com/gardener/docforge/blob/master/linkresolution3.md", node, source)
				Expect(err).ToNot(HaveOccurred())
				Expect(newLink).To(Equal("https://github.com/gardener/docforge/blob/master/linkresolution3.md"))
			})
		})
	})
})

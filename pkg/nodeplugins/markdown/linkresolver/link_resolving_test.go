// SPDX-FileCopyrightText: Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package linkresolver_test

import (
	"embed"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/linkresolver"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
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
			nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/baseline.yaml", linkResolver.Repositoryhosts)
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

		It("Relative links to files not in repository pass through as absolute links", func() {
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
			Expect(newLink).To(Equal("#anchor"))
		})

		It("Resolves _index.md correctly", func() {
			newLink, err := linkResolver.ResolveResourceLink("https://github.com/gardener/docforge/blob/master/docs/_index.md", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("/baseURL/two/internal/"))
		})

		It("Resolves non-page resource links correctly", func() {
			// non-page.md exists in the repo but is not in the manifest →
			// resolved to its absolute blob URL and passed through unchanged.
			newLink, err := linkResolver.ResolveResourceLink("./non-page.md", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("https://github.com/gardener/docforge/blob/master/non-page.md"))
		})

		It("Resolving url with no suitable repository host", func() {
			_, err := linkResolver.ResolveResourceLink("https://gitlab.com/gardener/docforge/blob/master/README.md", node, source)
			Expect(err.Error()).To(ContainSubstring("no sutiable repository host"))
		})

		It("Resolves resource links containing hugo structural directory correctly", func() {
			linkResolver.Hugo.HugoStructuralDirs = []string{"content"}
			newLink, err := linkResolver.ResolveResourceLink("https://github.com/gardener/docforge/blob/master/file.md", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("/baseURL/docs/file/"))
		})

		Context("#ResolveResourceLink relative-path mode (Hugo.Enabled=false)", func() {
			// Shadow linkResolver with Hugo disabled (no BaseURL) to exercise the
			// relative-output branch.  Source node is the same one/node.md.
			var relLR linkresolver.LinkResolver

			BeforeEach(func() {
				relLR = linkresolver.LinkResolver{}
				relRegistry := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
				relLR.Repositoryhosts = relRegistry
				relLR.Hugo = hugo.Hugo{Enabled: false}
				relLR.SourceToNode = make(map[string][]*manifest.Node)
				nodes, err := manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/baseline.yaml", relLR.Repositoryhosts)
				Expect(err).NotTo(HaveOccurred())
				for _, n := range nodes {
					if n.Source != "" {
						relLR.SourceToNode[n.Source] = append(relLR.SourceToNode[n.Source], n)
					} else if len(n.MultiSource) > 0 {
						for _, s := range n.MultiSource {
							relLR.SourceToNode[s] = append(relLR.SourceToNode[s], n)
						}
					}
				}
			})

			DescribeTable("returns relative paths",
				func(inputLink, expected string) {
					got, err := relLR.ResolveResourceLink(inputLink, node, source)
					Expect(err).ToNot(HaveOccurred())
					Expect(got).To(Equal(expected))
				},
				// source node is one/node.md -> sourceDir = "one"
				// clickhere.md maps to one/internal/linked.md -> NodePath = one/internal/linked.md
				// link.Build appends "/" before suffix, so websiteLink = "one/internal/linked.md/?a=b#c"
				// Rel("one", "one/internal/linked.md/?a=b#c") = "internal/linked.md/?a=b#c"
				Entry("query and anchor preserved",
					"clickhere.md?a=b#c",
					"internal/linked.md?a=b#c",
				),
				Entry("anchor-only suffix preserved",
					"clickhere.md#anchor",
					"internal/linked.md#anchor",
				),
				Entry("anchor with double hyphens preserved exactly",
					"clickhere.md#vpn-vpn-seed-server--vpn-shoot-client",
					"internal/linked.md#vpn-vpn-seed-server--vpn-shoot-client",
				),
				Entry("link without suffix produces no trailing fragment",
					"clickhere.md",
					"internal/linked.md",
				),
				// docs/_index.md URL -> node at two/internal/_index.md (filename = last URL segment)
				// Hugo disabled: NodePath = "two/internal/_index.md"
				// Rel("one", "two/internal/_index.md") = "../two/internal/_index.md"
				Entry("cross-section doc link",
					"https://github.com/gardener/docforge/blob/master/docs/_index.md",
					"../two/internal/_index.md",
				),
			)

			It("linkResolution closest node without override", func() {
				// Clear the linkResolution override so the closest-node algorithm runs.
				// linkresolution.md has two nodes: one/linkresolution.md and two/internal/far_linkresolution.md.
				// Closest to one/node.md is one/linkresolution.md.
				// Rel("one", "one/linkresolution.md") = "linkresolution.md"
				lr := node.LinkResolution
				node.LinkResolution = map[string]string{}
				defer func() { node.LinkResolution = lr }()
				got, err := relLR.ResolveResourceLink("https://github.com/gardener/docforge/blob/master/linkresolution.md", node, source)
				Expect(err).ToNot(HaveOccurred())
				Expect(got).To(Equal("linkresolution.md"))
			})

			It("hugo structural dir stripped", func() {
				relLR.Hugo.HugoStructuralDirs = []string{"content"}
				// content/docs/file.md; TrimPrefix("content/") -> docs/file.md; Rel("one","docs/file.md") = ../docs/file.md
				got, err := relLR.ResolveResourceLink("https://github.com/gardener/docforge/blob/master/file.md", node, source)
				Expect(err).ToNot(HaveOccurred())
				Expect(got).To(Equal("../docs/file.md"))
			})

			It("Hugo.Enabled=true BaseURL empty produces absolute without prefix", func() {
				relLR.Hugo.Enabled = true
				relLR.Hugo.BaseURL = ""
				// HugoPrettyPath of one/internal/linked.md = "one/internal/linked/"
				// link.Build("/", "", "one/internal/linked/") = "/one/internal/linked/"
				got, err := relLR.ResolveResourceLink("clickhere.md", node, source)
				Expect(err).ToNot(HaveOccurred())
				Expect(got).To(Equal("/one/internal/linked/"))
			})

			It("Section index with Hugo.Enabled=true no BaseURL produces absolute without prefix", func() {
				relLR.Hugo.Enabled = true
				relLR.Hugo.BaseURL = ""
				// HugoPrettyPath of two/internal/_index.md = "two/internal/"
				// link.Build("/", "", "two/internal/") = "/two/internal/"
				got, err := relLR.ResolveResourceLink("https://github.com/gardener/docforge/blob/master/docs/_index.md", node, source)
				Expect(err).ToNot(HaveOccurred())
				Expect(got).To(Equal("/two/internal/"))
			})
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

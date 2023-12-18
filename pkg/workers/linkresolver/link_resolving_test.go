// SPDX-FileCopyrightText: 2023 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package linkresolver_test

import (
	"embed"
	"fmt"
	"net/url"
	"strings"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/cmd/hugo"
	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts"
	"github.com/gardener/docforge/pkg/readers/repositoryhosts/repositoryhostsfakes"
	"github.com/gardener/docforge/pkg/workers/linkresolver"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Frontmatter Suite")
}

//go:embed tests/*
var manifests embed.FS

var _ = Describe("Document link resolving", func() {
	Context("#ResolveLink", func() {
		var (
			linkResolver linkresolver.LinkResolver
			node         *manifest.Node
			source       string
		)

		BeforeEach(func() {
			linkResolver = linkresolver.LinkResolver{}
			localHost := repositoryhostsfakes.FakeRepositoryHost{}
			localHost.ManifestFromURLCalls(func(url string) (string, error) {
				content, err := manifests.ReadFile(url)
				return string(content), err
			})
			localHost.ToAbsLinkCalls(func(URL, link string) (string, error) {
				if URL == "tests/baseline.yaml" {
					return link, nil
				}
				if link == "invalidfoo/bar.md" {
					return "", fmt.Errorf("err")
				}
				if link == "nonexistentfoo/bar.md" {
					return "https://github.com/fake_owner/fake_repo/blob/master/nonexistentfoo/bar.md", repositoryhosts.ErrResourceNotFound("err")
				}
				u, _ := url.Parse(URL)
				ulink, _ := url.Parse(link)
				return u.ResolveReference(ulink).String(), nil
			})
			registry := &repositoryhostsfakes.FakeRegistry{}
			registry.GetCalls(func(s string) (repositoryhosts.RepositoryHost, error) {
				if strings.HasPrefix(s, "https://github.com") || s == "tests/baseline.yaml" {
					return &localHost, nil
				}
				return nil, fmt.Errorf("no sutiable repository host for %s", s)
			})
			linkResolver.Repositoryhosts = registry
			linkResolver.Hugo = hugo.Hugo{
				Enabled: true,
				BaseURL: "baseURL",
			}
			linkResolver.SourceToNode = make(map[string][]*manifest.Node)
			nodes, err := manifest.ResolveManifest("tests/baseline.yaml", linkResolver.Repositoryhosts)
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
			source = "https://github.com/fake_owner/fake_repo/blob/master/target"
			node = linkResolver.SourceToNode[source][0]

		})

		It("Resolves outside link correctly", func() {
			newLink, validate, err := linkResolver.ResolveLink("https://outside_link.com", node, source)
			Expect(err).NotTo(HaveOccurred())
			Expect(newLink).To(Equal("https://outside_link.com"))
			Expect(validate).To(Equal(true))
		})

		It("Fails when can't create absolute link", func() {
			newLink, validate, err := linkResolver.ResolveLink("invalidfoo/bar.md", node, source)
			Expect(err).To(HaveOccurred())
			Expect(newLink).To(Equal(""))
			Expect(validate).To(Equal(false))
		})

		It("Resolves non existent resource correctly and won't request validation", func() {
			newLink, validate, err := linkResolver.ResolveLink("nonexistentfoo/bar.md", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("https://github.com/fake_owner/fake_repo/blob/master/nonexistentfoo/bar.md"))
			Expect(validate).To(Equal(false))
		})

		It("Resolves linking to manifest source correctly", func() {
			newLink, validate, err := linkResolver.ResolveLink("clickhere?a=b#c", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("/baseURL/one/internal/linked/?a=b#c"))
			Expect(validate).To(Equal(true))
		})

		It("Resolves anchor correctly", func() {
			newLink, validate, err := linkResolver.ResolveLink("#anchor", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("/baseURL/one/node/#anchor"))
			Expect(validate).To(Equal(true))
		})

		It("Resolves _index.md correctly", func() {
			newLink, validate, err := linkResolver.ResolveLink("https://github.com/fake_owner/fake_repo/blob/master/docs/_index.md", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("/baseURL/two/internal/"))
			Expect(validate).To(Equal(true))
		})

		It("Escapes /:v:/ correctly", func() {
			newLink, validate, err := linkResolver.ResolveLink("https://outside_link.com/:v:/one/two", node, source)
			Expect(err).ToNot(HaveOccurred())
			Expect(newLink).To(Equal("https://outside_link.com/%3Av%3A/one/two"))
			Expect(validate).To(Equal(true))
		})
	})

})

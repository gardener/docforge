// SPDX-FileCopyrightText: 2025 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package sourceorigin_test

import (
	"testing"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/manifestplugins/sourceorigin"
	"github.com/gardener/docforge/pkg/registry/registryfakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestSourceOriginPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "SourceOrigin Suite")
}

func fileNode(source string) *manifest.Node {
	return &manifest.Node{
		FileType: manifest.FileType{File: "doc.md", Source: source},
		Type:     "file",
	}
}

var _ = Describe("SourceOrigin plugin", func() {
	const src = "https://github.com/gardener/docforge/blob/master/contents/doc.md"

	runTransform := func(plugin *sourceorigin.SourceOrigin, node *manifest.Node, isRemote bool) error {
		fakeRegistry := &registryfakes.FakeInterface{}
		fakeRegistry.IsRemoteReturns(isRemote, nil)
		transforms := plugin.PluginNodeTransformations()
		Expect(transforms).To(HaveLen(1))
		_, err := transforms[0](node, nil, fakeRegistry)
		return err
	}

	Describe("default mapping (origin field)", func() {
		plugin := &sourceorigin.SourceOrigin{}

		It("writes origin: remote for a remote source", func() {
			node := fileNode(src)
			Expect(runTransform(plugin, node, true)).To(Succeed())
			Expect(node.Frontmatter["origin"]).To(Equal("remote"))
		})

		It("writes origin: local for a locally-mapped source", func() {
			node := fileNode(src)
			Expect(runTransform(plugin, node, false)).To(Succeed())
			Expect(node.Frontmatter["origin"]).To(Equal("local"))
		})

		It("skips dir nodes", func() {
			node := &manifest.Node{Type: "dir"}
			Expect(runTransform(plugin, node, true)).To(Succeed())
			Expect(node.Frontmatter).To(BeNil())
		})

		It("skips file nodes with no source", func() {
			node := &manifest.Node{
				FileType: manifest.FileType{File: "_index.md"},
				Type:     "file",
			}
			Expect(runTransform(plugin, node, true)).To(Succeed())
			Expect(node.Frontmatter).To(BeNil())
		})
	})

	Describe("Gardener mapping (managed/local fields)", func() {
		plugin := &sourceorigin.SourceOrigin{GardenerMapping: true}

		It("writes managed: true for a remote source", func() {
			node := fileNode(src)
			Expect(runTransform(plugin, node, true)).To(Succeed())
			Expect(node.Frontmatter["managed"]).To(Equal(true))
			Expect(node.Frontmatter).NotTo(HaveKey("local"))
		})

		It("writes local: true for a locally-mapped source", func() {
			node := fileNode(src)
			Expect(runTransform(plugin, node, false)).To(Succeed())
			Expect(node.Frontmatter["local"]).To(Equal(true))
			Expect(node.Frontmatter).NotTo(HaveKey("managed"))
		})
	})

	It("preserves existing frontmatter", func() {
		plugin := &sourceorigin.SourceOrigin{}
		node := fileNode(src)
		node.Frontmatter = map[string]interface{}{"title": "My Doc"}
		Expect(runTransform(plugin, node, true)).To(Succeed())
		Expect(node.Frontmatter["title"]).To(Equal("My Doc"))
		Expect(node.Frontmatter["origin"]).To(Equal("remote"))
	})

	It("uses first MultiSource entry when Source is empty", func() {
		plugin := &sourceorigin.SourceOrigin{}
		node := &manifest.Node{
			FileType: manifest.FileType{
				MultiSource: []string{src, "https://github.com/gardener/docforge/blob/master/other.md"},
			},
			Type: "file",
		}
		Expect(runTransform(plugin, node, false)).To(Succeed())
		Expect(node.Frontmatter["origin"]).To(Equal("local"))
	})
})

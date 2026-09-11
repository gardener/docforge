// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package frontmatter_test

import (
	"embed"
	"reflect"
	"testing"

	_ "embed"

	"github.com/gardener/docforge/pkg/manifest"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document/frontmatter"
	"github.com/gardener/docforge/pkg/nodeplugins/markdown/document/frontmatter/frontmatterfakes"
	"github.com/gardener/docforge/pkg/registry"
	"github.com/gardener/docforge/pkg/registry/repositoryhost"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestJobs(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Frontmatter Suite")
}

//go:embed tests/*
var manifests embed.FS

var _ = Describe("Document frontmatter", func() {

	Context("#MoveMultiSourceFrontmatterToTopDocument", func() {
		It("merges even nil and empty NodeMetas successfully", func() {
			node1 := &frontmatterfakes.FakeNodeMeta{}
			node2 := &frontmatterfakes.FakeNodeMeta{}
			node3 := &frontmatterfakes.FakeNodeMeta{}
			node4 := &frontmatterfakes.FakeNodeMeta{}
			node5 := &frontmatterfakes.FakeNodeMeta{}
			node1.MetaReturns(map[string]interface{}{
				"foo": "foo node 1",
			})
			node2.MetaReturns(nil)
			node3.MetaReturns(map[string]interface{}{
				"foo": "foo node 3",
				"bar": "bar node 3",
			})
			node4.MetaReturns(map[string]interface{}{})
			node5.MetaReturns(map[string]interface{}{
				"foo": "foo node 4",
				"bar": "bar node 4",
				"baz": "bar node 4",
			})
			nodeAst := []frontmatter.NodeMeta{}
			nodeAst = append(nodeAst, node1, node2, node3, node4, node5)

			frontmatter.MoveMultiSourceFrontmatterToTopDocument(nodeAst)
			Expect(node1.SetMetaArgsForCall(1)).To(Equal(map[string]interface{}{
				"foo": "foo node 1",
				"bar": "bar node 3",
				"baz": "bar node 4",
			}))
			var nilmap map[string]interface{}
			Expect(node2.SetMetaArgsForCall(0)).To(Equal(nilmap))
			Expect(node3.SetMetaArgsForCall(0)).To(Equal(nilmap))
			Expect(node4.SetMetaArgsForCall(0)).To(Equal(nilmap))

		})
	})
	Context("#MergeDocumentAndNodeFrontmatter", func() {
		var (
			nodeAst *frontmatterfakes.FakeNodeMeta
			nodes   []*manifest.Node
			node    *manifest.Node
			err     error
		)
		BeforeEach(func() {
			r := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
			nodes, err = manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/frontmatter.yaml", r)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(nodes)).To(Equal(3))
			Expect(nodes[1].Name()).To(Equal("foo.md"))
			Expect(nodes[2].Name()).To(Equal("bar.md"))

			nodeAst = &frontmatterfakes.FakeNodeMeta{}
			nodeAst.MetaReturns(map[string]interface{}{
				"foo": "file_fooVal",
				"aliases": []interface{}{
					"file_alias1",
					"file_alias2",
				},
				"bar": "file_barVal",
				"barArray": []interface{}{
					"file_bar1",
					"file_bar2",
					"file_bar3",
				},
			})

		})
		It("doesn't change anything if node is nil", func() {
			node = nil

			frontmatter.MergeDocumentAndNodeFrontmatter(nodeAst, node)
			Expect(nodeAst.SetMetaCallCount()).To(Equal(0))

		})
		It("doesn't change anything if node has no frontmatter", func() {
			node = nodes[1]

			frontmatter.MergeDocumentAndNodeFrontmatter(nodeAst, node)

			setMeta := nodeAst.SetMetaArgsForCall(0)
			Expect(reflect.DeepEqual(setMeta, map[string]interface{}{
				"foo": "file_fooVal",
				"aliases": []interface{}{
					"file_alias1",
					"file_alias2",
				},
				"bar": "file_barVal",
				"barArray": []interface{}{
					"file_bar1",
					"file_bar2",
					"file_bar3",
				},
			})).To(Equal(true))
		})
		It("aliases get merged and node overrides all other", func() {
			node = nodes[2]

			frontmatter.MergeDocumentAndNodeFrontmatter(nodeAst, node)

			setMeta := nodeAst.SetMetaArgsForCall(0)
			Expect(reflect.DeepEqual(setMeta, map[string]interface{}{
				"foo": "file_fooVal",
				"aliases": []interface{}{
					"node_alias1",
					"node_alias2",
					"file_alias1",
					"file_alias2",
				},
				"bar": "node_barVal",
				"barArray": []interface{}{
					"node_bar1",
					"node_bar2",
				},
				"baz": "node_bazVal",
			})).To(BeTrue())
			Expect(reflect.DeepEqual(setMeta, node.Frontmatter)).To(BeTrue())
		})
	})

	Context("#MergeDocumentAndNodeFrontmatter protects round-trip fields", func() {
		var nodeAst *frontmatterfakes.FakeNodeMeta

		newNode := func(fm map[string]interface{}) *manifest.Node {
			n := &manifest.Node{}
			n.Frontmatter = fm
			return n
		}

		BeforeEach(func() {
			nodeAst = &frontmatterfakes.FakeNodeMeta{}
		})

		It("B1: keeps the original github_repo when the file already has it", func() {
			nodeAst.MetaReturns(map[string]interface{}{
				"github_repo": "https://github.com/gardener/documentation",
			})
			node := newNode(map[string]interface{}{
				"github_repo": "https://github.com/gardener/new-source",
			})

			frontmatter.MergeDocumentAndNodeFrontmatter(nodeAst, node)

			setMeta := nodeAst.SetMetaArgsForCall(0)
			Expect(setMeta["github_repo"]).To(Equal("https://github.com/gardener/documentation"))
			Expect(node.Frontmatter["github_repo"]).To(Equal("https://github.com/gardener/documentation"))
		})

		It("B2: uses the node value when the file lacks github_repo", func() {
			nodeAst.MetaReturns(map[string]interface{}{})
			node := newNode(map[string]interface{}{
				"github_repo": "https://github.com/gardener/new-source",
			})

			frontmatter.MergeDocumentAndNodeFrontmatter(nodeAst, node)

			setMeta := nodeAst.SetMetaArgsForCall(0)
			Expect(setMeta["github_repo"]).To(Equal("https://github.com/gardener/new-source"))
		})

		It("B3: keeps the original path_base_for_github_subdir map", func() {
			original := map[interface{}]interface{}{
				"from": "content/docs/getting-started/page.md",
				"to":   "page.md",
			}
			nodeAst.MetaReturns(map[string]interface{}{
				"path_base_for_github_subdir": original,
			})
			node := newNode(map[string]interface{}{
				"path_base_for_github_subdir": map[interface{}]interface{}{
					"from": "new/from.md",
					"to":   "new-to.md",
				},
			})

			frontmatter.MergeDocumentAndNodeFrontmatter(nodeAst, node)

			setMeta := nodeAst.SetMetaArgsForCall(0)
			Expect(setMeta["path_base_for_github_subdir"]).To(Equal(original))
		})

		It("B4: keeps original params.github_branch but merges other params keys", func() {
			nodeAst.MetaReturns(map[string]interface{}{
				"params": map[interface{}]interface{}{
					"github_branch": "master",
					"foo":           "bar",
				},
			})
			node := newNode(map[string]interface{}{
				"params": map[interface{}]interface{}{
					"github_branch": "newbranch",
					"extra":         "added",
				},
			})

			frontmatter.MergeDocumentAndNodeFrontmatter(nodeAst, node)

			setMeta := nodeAst.SetMetaArgsForCall(0)
			params, ok := setMeta["params"].(map[interface{}]interface{})
			Expect(ok).To(BeTrue())
			Expect(params["github_branch"]).To(Equal("master"))
			Expect(params["foo"]).To(Equal("bar"))
			Expect(params["extra"]).To(Equal("added"))
		})

		It("B5: an empty-string original value still wins (present = present)", func() {
			nodeAst.MetaReturns(map[string]interface{}{
				"github_repo": "",
			})
			node := newNode(map[string]interface{}{
				"github_repo": "https://github.com/gardener/new-source",
			})

			frontmatter.MergeDocumentAndNodeFrontmatter(nodeAst, node)

			setMeta := nodeAst.SetMetaArgsForCall(0)
			Expect(setMeta["github_repo"]).To(Equal(""))
		})
	})

	Context("#ComputeNodeTitle", func() {
		var (
			nodeAst        *frontmatterfakes.FakeNodeMeta
			nodes          []*manifest.Node
			node           *manifest.Node
			indexFileNames []string
			hugoEnabled    bool
			err            error
		)
		BeforeEach(func() {
			r := registry.NewRegistry(repositoryhost.NewLocalTest(manifests, "https://github.com/gardener/docforge", "tests"))
			nodes, err = manifest.ResolveManifest("https://github.com/gardener/docforge/blob/master/titles.yaml", r)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(nodes)).To(Equal(6))
			Expect(nodes[1].Name()).To(Equal("file_node-1.md"))
			Expect(nodes[2].Name()).To(Equal("_index.md"))

			indexFileNames = []string{"README.md"}
			hugoEnabled = true
			nodeAst = &frontmatterfakes.FakeNodeMeta{}
		})
		Context("top level node", func() {
			It("removes _,- and .md in the general case", func() {
				node = nodes[1]
				frontmatter.ComputeNodeTitle(nodeAst, node, indexFileNames, hugoEnabled)
				setMeta := nodeAst.SetMetaArgsForCall(0)
				Expect(setMeta).To(Equal(map[string]interface{}{
					"title": "File Node 1",
				}))
			})
			It("has title Root if file is index", func() {
				node = nodes[2]
				frontmatter.ComputeNodeTitle(nodeAst, node, indexFileNames, hugoEnabled)
				setMeta := nodeAst.SetMetaArgsForCall(0)
				Expect(setMeta).To(Equal(map[string]interface{}{
					"title": "Root",
				}))
			})
			Context("node with parent", func() {
				It("removes _,- and .md in the general case", func() {
					node = nodes[4]
					frontmatter.ComputeNodeTitle(nodeAst, node, indexFileNames, hugoEnabled)
					setMeta := nodeAst.SetMetaArgsForCall(0)
					Expect(setMeta).To(Equal(map[string]interface{}{
						"title": "File Node 2",
					}))
				})
				It("uses parents name if file is index", func() {
					node = nodes[5]
					frontmatter.ComputeNodeTitle(nodeAst, node, indexFileNames, hugoEnabled)
					setMeta := nodeAst.SetMetaArgsForCall(0)
					Expect(setMeta).To(Equal(map[string]interface{}{
						"title": "Parent Dir",
					}))
				})
			})

		})
	})

})

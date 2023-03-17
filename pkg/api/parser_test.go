// SPDX-FileCopyrightText: 2021 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0
package api_test

import (
	"errors"
	"fmt"
	"io/ioutil"
	"path/filepath"

	"github.com/gardener/docforge/pkg/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("Parser", func() {
	Describe("Parsing manifest", func() {
		DescribeTable("parsing tests", func(manifest []byte, expDoc *api.Documentation, expErr error) {
			doc, err := api.Parse(manifest, api.ParsingOptions{Hugo: true, ExtractedFilesFormats: []string{".md"}})
			if expErr == nil {
				Expect(err).To(BeNil())
			} else {
				Expect(err.Error()).To(ContainSubstring(expErr.Error()))
			}
			if expDoc != nil {
				for _, n := range expDoc.Structure {
					n.SetParentsDownwards()
				}
			}
			Expect(doc).To(Equal(expDoc))
		},
			Entry("no valid documentation entry", []byte("source: https://github.com/gardener/docforge/blob/master/README.md"), nil,
				fmt.Errorf("the document structure must contains at least one of these properties: structure, nodesSelector")),
			Entry("documentation nodeSelector path is mandatory", []byte(`
                nodesSelector:
                  name: test`), nil,
				fmt.Errorf("nodesSelector under / must contains a path property")),
			Entry("node name or/and source should be set", []byte(`
                structure:
                - name: test
                  nodes:
                  - name: sub
                    properties: {index: true}`), nil,
				fmt.Errorf("node test/sub must contains at least one of these properties: source, nodesSelector, multiSource, nodes")),
			Entry("node document mandatory properties", []byte(`
                structure:
                - name: test
                  nodes:
                  - multiSource: [source1, source2]`), nil,
				fmt.Errorf("node test/ must contains at least one of these properties: source, name")),
			Entry("node is either document or container", []byte(`
                structure:
                - name: test
                  source: https://github.com/gardener/docforge/blob/master/README.md
                  nodes:
                  - name: test2
                    source: https://github.com/gardener/docforge/blob/master/README.md`), nil,
				fmt.Errorf("node /test must be categorized as a document or a container")),
			Entry("structure nodeSelector path is mandatory", []byte(`
                structure:
                - nodesSelector:
                    frontMatter:`), nil,
				fmt.Errorf("nodesSelector under / must contains a path property")),
			Entry("structure nodeSelector path is mandatory", []byte(`
                structure:
                - name: gardener-extension-shoot-dns-service
                  properties:
                    frontmatter:
                      title: DNS services
                      description: Gardener extension controller for DNS services for shoot clusters
                  nodes:
                  - nodesSelector:
                      path: https://github.com/gardener/gardener-extension-shoot-dns-service/blob/master/.docforge/manifest.yaml
                    name: selector
                - name: gardener-extension-shoot-cert-service
                  properties:
                    frontmatter:
                      title: Certificate services
                      description: Gardener extension controller for certificate services for shoot clusters
                  nodes:
                  - nodesSelector:
                      path: https://github.com/gardener/gardener-extension-shoot-cert-service/blob/master/.docforge/manifest.yaml
                    name: selector`),
				&api.Documentation{
					Structure: []*api.Node{
						{
							Name: "gardener-extension-shoot-dns-service",
							Nodes: []*api.Node{
								{
									Name: "selector",
									NodeSelector: &api.NodeSelector{
										Path: "https://github.com/gardener/gardener-extension-shoot-dns-service/blob/master/.docforge/manifest.yaml",
									},
								},
							},
							Properties: map[string]interface{}{
								"frontmatter": map[string]interface{}{
									"description": "Gardener extension controller for DNS services for shoot clusters",
									"title":       "DNS services",
								},
							},
						},
						{
							Name: "gardener-extension-shoot-cert-service",
							Nodes: []*api.Node{
								{
									Name: "selector",
									NodeSelector: &api.NodeSelector{
										Path: "https://github.com/gardener/gardener-extension-shoot-cert-service/blob/master/.docforge/manifest.yaml",
									},
								},
							},
							Properties: map[string]interface{}{
								"frontmatter": map[string]interface{}{
									"description": "Gardener extension controller for certificate services for shoot clusters",
									"title":       "Certificate services",
								},
							},
						},
					},
				}, nil),
		)
	})
	Describe("Serialize", func() {
		var (
			doc *api.Documentation
			got string
			err error
			exp string
		)
		JustBeforeEach(func() {
			got, err = api.Serialize(doc)
		})
		Context("given valid documentation", func() {
			BeforeEach(func() {
				doc = &api.Documentation{
					Structure: []*api.Node{
						{
							Name: "A Title",
							Nodes: []*api.Node{
								{
									Name:   "node 1",
									Source: "https://test/source1.md",
								},
								{
									Name:        "node 2",
									MultiSource: []string{"https://multitest/a.md", "https://multitest/b.md"},
									Properties: map[string]interface{}{
										"custom_key": "custom_value",
									},
								},
								{
									Name: "path 1",
									Nodes: []*api.Node{
										{
											Name:   "subnode",
											Source: "https://test/subnode.md",
										},
									},
								},
							},
						},
					},
				}
				exp = `structure:
    - name: A Title
      nodes:
        - name: node 1
          source: https://test/source1.md
        - name: node 2
          multiSource:
            - https://multitest/a.md
            - https://multitest/b.md
          properties:
            custom_key: custom_value
        - name: path 1
          nodes:
            - name: subnode
              source: https://test/subnode.md
`
			})
			It("serialize as expected", func() {
				Expect(err).To(BeNil())
				Expect(got).To(Equal(exp))
			})
		})
	})
	Describe("Parse file", func() {
		var (
			manifest []byte
			got      *api.Documentation
			err      error
			exp      *api.Documentation
		)
		JustBeforeEach(func() {
			got, err = api.Parse(manifest, api.ParsingOptions{Hugo: true, ExtractedFilesFormats: []string{".md"}})
		})
		Context("given manifest file", func() {
			BeforeEach(func() {
				manifest, err = ioutil.ReadFile(filepath.Join("testdata", "parse_test_00.yaml"))
				Expect(err).ToNot(HaveOccurred())
				exp = &api.Documentation{
					Structure: []*api.Node{
						{
							Name: "00",
							Nodes: []*api.Node{
								{
									Name:   "01",
									Source: "https://github.com/gardener/gardener/blob/master/docs/concepts/gardenlet.md",
								},
								{
									Name: "02",
									MultiSource: []string{
										"https://github.com/gardener/gardener/blob/master/docs/deployment/deploy_gardenlet.md",
									},
								},
							},
						},
					},
				}
				for _, n := range exp.Structure {
					n.SetParentsDownwards()
				}
			})
			It("parse as expected", func() {
				Expect(err).To(BeNil())
				Expect(got).To(Equal(exp))
			})
		})
	})
	Describe("Getting node parrent path", func() {
		type argsStruct struct {
			node    *api.Node
			parents []*api.Node
		}
		var (
			args argsStruct
			res  string
			exp  string
		)
		JustBeforeEach(func() {
			setParents(args.node, args.parents)
			res = api.GetNodeParentPath(args.node)
		})
		Context("Pass nil parent node", func() {
			BeforeEach(func() {
				args = argsStruct{node: nil}
				exp = "root"
			})
			It("should process it correctly", func() {
				Expect(res).To(Equal(exp))
			})
		})
		Context("Pass node without parent", func() {
			BeforeEach(func() {
				args = argsStruct{node: &api.Node{Name: "top"}}
				exp = "top"
			})
			It("should process it correctly", func() {
				Expect(res).To(Equal(exp))
			})
		})
		Context("Pass node with one ancestor", func() {
			BeforeEach(func() {
				args = argsStruct{
					node: &api.Node{Name: "father"},
					parents: []*api.Node{
						{Name: "grandfather"},
					},
				}
				exp = "grandfather.father"
			})
			It("should process it correctly", func() {
				Expect(res).To(Equal(exp))
			})
		})
		Context("Pass node with two ancestor", func() {
			BeforeEach(func() {
				args = argsStruct{
					node: &api.Node{Name: "son"},
					parents: []*api.Node{
						{Name: "grandfather"},
						{Name: "father"},
					},
				}
				exp = "grandfather.father.son"
			})
			It("should process it correctly", func() {
				Expect(res).To(Equal(exp))
			})
		})
	})
	Describe("Building node collision list", func() {
		type argsStruct struct {
			nodes           []*api.Node
			parent          *api.Node
			collisionsNames []string
		}
		var (
			args argsStruct
			res  *api.Collision

			exp api.Collision
			err error
		)
		JustBeforeEach(func() {
			res, err = api.BuildNodeCollision(args.nodes, args.parent, args.collisionsNames, api.ParsingOptions{Hugo: true, ExtractedFilesFormats: []string{".md"}})
		})
		Context("Pass nodes with one collision of two nodes", func() {
			BeforeEach(func() {
				args = argsStruct{
					nodes: []*api.Node{
						{Name: "foo", Source: "foo/bar"},
						{Name: "foo", Source: "baz/bar/foo"},
					},
					parent:          &api.Node{Name: "parent"},
					collisionsNames: []string{"foo.md"},
				}
				exp = api.Collision{
					NodeParentPath: "parent",
					CollidedNodes: map[string][]string{
						"foo.md": {"foo/bar", "baz/bar/foo"},
					},
				}
			})
			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(*res).To(Equal(exp))
			})
		})
		Context("Pass nodes with one collision of three nodes", func() {
			BeforeEach(func() {
				args = argsStruct{
					nodes: []*api.Node{
						{Name: "foo", Source: "foo/bar"},
						{Name: "foo", Source: "baz/bar/foo"},
						{Name: "foo", Source: "baz/bar/foo/fuz"},
					},
					parent:          &api.Node{Name: "parent"},
					collisionsNames: []string{"foo.md"},
				}
				exp = api.Collision{
					NodeParentPath: "parent",
					CollidedNodes: map[string][]string{
						"foo.md": {"foo/bar", "baz/bar/foo", "baz/bar/foo/fuz"},
					},
				}
			})
			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(*res).To(Equal(exp))
			})
		})
		Context("Pass nodes with two collision", func() {
			BeforeEach(func() {
				args = argsStruct{
					nodes: []*api.Node{
						{Name: "foo", Source: "foo/bar"},
						{Name: "foo", Source: "baz/bar/foo"},
						{Name: "moo", Source: "moo/bar"},
						{Name: "moo", Source: "baz/bar/moo"},
						{Name: "normal", Source: "baz/bar/moo"},
					},
					parent:          &api.Node{Name: "parent"},
					collisionsNames: []string{"foo.md", "moo.md"},
				}
				exp = api.Collision{
					NodeParentPath: "parent",
					CollidedNodes: map[string][]string{
						"foo.md": {"foo/bar", "baz/bar/foo"},
						"moo.md": {"moo/bar", "baz/bar/moo"},
					},
				}
			})
			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(*res).To(Equal(exp))
			})
		})
	})
	Describe("Checking nodes for collision", func() {
		type argsStruct struct {
			nodes      []*api.Node
			parent     *api.Node
			collisions []api.Collision
		}
		var (
			args argsStruct
			res  []api.Collision

			exp []api.Collision
			err error
		)
		JustBeforeEach(func() {
			res, err = api.CheckNodesForCollision(args.nodes, args.parent, args.collisions, api.ParsingOptions{Hugo: true, ExtractedFilesFormats: []string{".md"}})
		})
		Context("Nodes with one collision", func() {
			BeforeEach(func() {
				args = argsStruct{
					nodes: []*api.Node{
						{Name: "foo", Source: "bar/baz"},
						{Name: "foo", Source: "baz/foo"},
						{Name: "normal", Source: "baz/foo"},
					},
					parent: &api.Node{
						Name: "parent",
					},
				}
				exp = []api.Collision{
					{
						NodeParentPath: "parent",
						CollidedNodes: map[string][]string{
							"foo.md": {"bar/baz", "baz/foo"},
						},
					},
				}
			})
			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(Equal(exp))
			})
		})
		Context("Nodes with two collision", func() {
			BeforeEach(func() {
				args = argsStruct{
					nodes: []*api.Node{
						{Name: "foo", Source: "bar/baz"},
						{Name: "foo", Source: "baz/foo"},
						{Name: "normal", Source: "baz/foo"},
						{Name: "moo", Source: "bar/baz"},
						{Name: "moo", Source: "baz/moo"},
					},
					parent: &api.Node{
						Name: "parent",
					},
				}
				exp = []api.Collision{
					{
						NodeParentPath: "parent",
						CollidedNodes: map[string][]string{
							"foo.md": {"bar/baz", "baz/foo"},
							"moo.md": {"bar/baz", "baz/moo"},
						},
					},
				}
			})
			It("should process it correctly", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(res).To(Equal(exp))
			})
		})
	})
	Describe("Checking for collisions", func() {
		type argsStruct struct {
			nodes []*api.Node
		}
		var (
			args argsStruct
			err  error
		)
		JustBeforeEach(func() {
			recursiveSetParents(args.nodes, nil)
			err = api.CheckForCollisions(args.nodes, api.ParsingOptions{Hugo: true, ExtractedFilesFormats: []string{".md"}})
		})
		Context("Test with one collision", func() {
			BeforeEach(func() {
				args = argsStruct{
					nodes: []*api.Node{
						{
							Name: "grandfather",
							Nodes: []*api.Node{
								{
									Name: "parent",
									Nodes: []*api.Node{
										{Name: "son", Source: "https://foo/bar/son"},
										{Name: "son", Source: "https://foo/bar/bor"},
									},
								},
							},
						},
					},
				}
			})
			It("should return error", func() {
				Expect(err).To(Equal(errors.New("Node collisions detected.\nIn grandfather.parent container node. Node with name son.md appears 2 times for sources: https://foo/bar/son, https://foo/bar/bor.")))
			})
		})
		Context("Test with many collisionsn", func() {
			BeforeEach(func() {
				args = argsStruct{
					nodes: []*api.Node{
						{
							Name: "grandfather",
							Nodes: []*api.Node{
								{
									Name: "father",
									Nodes: []*api.Node{
										{Name: "son", Source: "https://foo/bar/son"},
										{Name: "son", Source: "https://foo/bar/bor"},
									},
								},
								{
									Name: "mother",
									Nodes: []*api.Node{
										{Name: "daughter", Source: "https://foo/bar/daughter"},
										{Name: "daughter", Source: "https://foo/daughter/bor"},
									},
								},
							},
						},
						{
							Name: "grandmother",
							Nodes: []*api.Node{
								{
									Name: "father",
									Nodes: []*api.Node{
										{Name: "son", Source: "https://foo/bar/son"},
										{Name: "son", Source: "https://foo/bar/bor"},
									},
								},
								{
									Name: "mother",
									Nodes: []*api.Node{
										{Name: "daughter", Source: "https://foo/bar/daughter"},
										{Name: "daughter", Source: "https://foo/daughter/bor"},
									},
								},
							},
						},
						{
							Name:   "grandmother",
							Source: "https://some/url/to/source",
						},
					},
				}
			})
			It("should return error", func() {
				Expect(err).To(Equal(errors.New("Node collisions detected.\nIn grandfather.father container node. Node with name son.md appears 2 times for sources: https://foo/bar/son, https://foo/bar/bor.\nIn grandfather.mother container node. Node with name daughter.md appears 2 times for sources: https://foo/bar/daughter, https://foo/daughter/bor.\nIn grandmother.father container node. Node with name son.md appears 2 times for sources: https://foo/bar/son, https://foo/bar/bor.\nIn grandmother.mother container node. Node with name daughter.md appears 2 times for sources: https://foo/bar/daughter, https://foo/daughter/bor.")))
			})
		})
		Context("Test with nodes after collision", func() {
			BeforeEach(func() {
				args = argsStruct{
					nodes: []*api.Node{
						{
							Name: "l1",
							Nodes: []*api.Node{
								{
									Name: "l11",
									Nodes: []*api.Node{
										{Name: "l111", Source: "https://foo/bar/l111"},
										{Name: "l111", Source: "https://foo/bar/l111"},
										{Name: "l112", Source: "https://foo/bar/l112"},
									},
								},
							},
						},
						{
							Name: "l2",
							Nodes: []*api.Node{
								{
									Name: "l21",
								},
							},
						},
					},
				}
			})
			It("should return error", func() {
				Expect(err).To(Equal(errors.New("Node collisions detected.\nIn l1.l11 container node. Node with name l111.md appears 2 times for sources: https://foo/bar/l111, https://foo/bar/l111.")))
			})
		})
		Context("Test without collision", func() {
			BeforeEach(func() {
				args = argsStruct{
					nodes: []*api.Node{
						{
							Name: "l1",
							Nodes: []*api.Node{
								{
									Name: "l11",
									Nodes: []*api.Node{
										{Name: "l111", Source: "https://foo/bar/l111"},
										{Name: "l112", Source: "https://foo/bar/l112"},
									},
								},
							},
						},
					},
				}
			})
			It("shouldn't return error", func() {
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})

func setParents(node *api.Node, parents []*api.Node) {
	currentNode := node
	for i := len(parents) - 1; i >= 0; i-- {
		parent := parents[i]
		currentNode.SetParent(parent)
		currentNode = parent
	}
}

func recursiveSetParents(nodes []*api.Node, parent *api.Node) {
	for _, node := range nodes {
		node.SetParent(parent)
		recursiveSetParents(node.Nodes, node)
	}
}

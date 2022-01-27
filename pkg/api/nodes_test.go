// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api_test

import (
	"github.com/gardener/docforge/pkg/api"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/ginkgo/extensions/table"
	. "github.com/onsi/gomega"
)

var _ = Describe("API Nodes", func() {
	Context("Parents & Helpers", func() {
		var (
			child, parent, ancestor *api.Node
			res                     string
		)
		BeforeEach(func() {
			child = &api.Node{Name: "child"}
			parent = &api.Node{Name: "parent", Nodes: []*api.Node{child}}
			ancestor = &api.Node{Name: "ancestor", Nodes: []*api.Node{parent}}
		})
		When("set parent", func() {
			JustBeforeEach(func() {
				child.SetParent(parent)
			})
			It("sets the parent", func() {
				Expect(child.Parent()).To(Equal(parent))
			})
		})
		When("set parents downwards", func() {
			JustBeforeEach(func() {
				ancestor.SetParentsDownwards()
			})
			It("sets the parents downwards", func() {
				Expect(child.Parents()).To(Equal([]*api.Node{ancestor, parent}))
			})
		})
		When("get node path", func() {
			BeforeEach(func() {
				ancestor.SetParentsDownwards()
			})
			JustBeforeEach(func() {
				res = child.Path("/")
			})
			It("returns path to the node", func() {
				Expect(res).To(Equal("ancestor/parent"))
			})
		})
		When("get node full name", func() {
			BeforeEach(func() {
				ancestor.SetParentsDownwards()
			})
			JustBeforeEach(func() {
				res = child.FullName("/")
			})
			It("returns path to the node", func() {
				Expect(res).To(Equal("ancestor/parent/child"))
			})
		})
		Context("to string", func() {
			BeforeEach(func() {
				child.Source = "https://test/child.md"
				ancestor.SetParentsDownwards()
			})
			JustBeforeEach(func() {
				res = ancestor.String()
			})
			It("represents node as a yaml string", func() {
				Expect(res).To(Equal("name: ancestor\nnodes:\n    - name: parent\n      nodes:\n        - name: child\n          source: https://test/child.md\n"))
			})
		})
		Context("node sources", func() {
			JustBeforeEach(func() {
				res = child.Sources()
			})
			When("get single source", func() {
				BeforeEach(func() {
					child.Source = "https://test/child.md"
				})
				It("returns single source location", func() {
					Expect(res).To(Equal("https://test/child.md"))
				})
			})
			When("get multi source", func() {
				BeforeEach(func() {
					child.MultiSource = []string{"https://test/part1.md", "https://test/part2.md"}
				})
				It("returns multi source locations", func() {
					Expect(res).To(Equal("https://test/part1.md,https://test/part2.md"))
				})
			})
		})
	})
	Context("Relative path", func() {
		//       A                    A1
		//    /	    \               /
		//   B	     C             B1
		//  / \	    / \           /
		// D   E   F   G         C1
		//      \
		//       I
		// 	      \
		// 	       J
		var nodeMap map[string]*api.Node
		BeforeEach(func() {
			nodeMap = make(map[string]*api.Node)
			nodeMap["J"] = &api.Node{Name: "J"}
			nodeMap["I"] = &api.Node{Name: "I", Nodes: []*api.Node{nodeMap["J"]}}
			nodeMap["E"] = &api.Node{Name: "E", Nodes: []*api.Node{nodeMap["I"]}}
			nodeMap["D"] = &api.Node{Name: "D"}
			nodeMap["B"] = &api.Node{Name: "B", Nodes: []*api.Node{nodeMap["D"], nodeMap["E"]}}
			nodeMap["F"] = &api.Node{Name: "F"}
			nodeMap["G"] = &api.Node{Name: "G"}
			nodeMap["C"] = &api.Node{Name: "C", Nodes: []*api.Node{nodeMap["F"], nodeMap["G"]}}
			nodeMap["A"] = &api.Node{Name: "A", Nodes: []*api.Node{nodeMap["B"], nodeMap["C"]}}
			nodeMap["C1"] = &api.Node{Name: "C1"}
			nodeMap["B1"] = &api.Node{Name: "B1", Nodes: []*api.Node{nodeMap["C1"]}}
			nodeMap["A1"] = &api.Node{Name: "A1", Nodes: []*api.Node{nodeMap["B1"]}}
			nodeMap["A"].SetParentsDownwards()
			nodeMap["A1"].SetParentsDownwards()
		})
		DescribeTable("path from -> to", func(from, to, relPath string) {
			Expect(nodeMap[from].RelativePath(nodeMap[to])).To(Equal(relPath))
		},
			Entry("path to self", "I", "I", "I"),
			Entry("path to parent node", "J", "I", "../I"),
			Entry("path to sibling node", "D", "E", "./E"),
			Entry("path to descendent node", "E", "J", "./I/J"),
			Entry("path to ancestor node", "J", "E", "../../E"),
			Entry("path to node on another branch", "I", "G", "../../C/G"),
			Entry("path to root", "I", "A", "../../../A"),
			Entry("path from I to A1", "I", "A1", "../../../A1"),
			Entry("path from I to C1", "I", "C1", "../../../A1/B1/C1"),
			Entry("path from C1 to I", "C1", "I", "../../A/B/E/I"),
			Entry("path from A1 to A", "A1", "A", "./A"),
			Entry("path from A to A1", "A", "A1", "./A1"),
		)
	})
	Context("Union", func() {
		var (
			node  *api.Node
			nodes []*api.Node
			err   error
			exp   *api.Node
		)
		JustBeforeEach(func() {
			node.SetParentsDownwards()
			exp.SetParentsDownwards()
			for _, n := range nodes {
				n.SetParentsDownwards()
			}
			err = node.Union(nodes)
		})
		When("no collisions exists", func() {
			BeforeEach(func() {
				node = &api.Node{Name: "docs", Nodes: []*api.Node{{Name: "sub", Nodes: []*api.Node{{Name: "a.md", Source: "https://test/a.md"}}}}}
				nodes = []*api.Node{{Name: "r.md", Source: "https://test/r.md"}, {Name: "sub", Nodes: []*api.Node{{Name: "b.md", Source: "https://test/b.md"}}}}
				exp = &api.Node{Name: "docs", Nodes: []*api.Node{{Name: "sub", Nodes: []*api.Node{{Name: "a.md", Source: "https://test/a.md"}, {Name: "b.md", Source: "https://test/b.md"}}}, {Name: "r.md", Source: "https://test/r.md"}}}
			})
			It("merge nodes as expected", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(node).To(Equal(exp))
			})
		})
		When("collision with duplicate node exists", func() {
			BeforeEach(func() {
				node = &api.Node{Name: "docs", Nodes: []*api.Node{{Name: "sub", Nodes: []*api.Node{{Name: "a.md", Source: "https://test/a.md"}}}}}
				nodes = []*api.Node{{Name: "r.md", Source: "https://test/r.md"}, {Name: "sub", Nodes: []*api.Node{{Name: "a.md", Source: "https://test/a.md"}}}}
				exp = &api.Node{Name: "docs", Nodes: []*api.Node{{Name: "sub", Nodes: []*api.Node{{Name: "a.md", Source: "https://test/a.md"}}}, {Name: "r.md", Source: "https://test/r.md"}}}
			})
			It("skips the duplicate", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(node).To(Equal(exp))
			})
		})
		When("collision with different document nodes exists", func() {
			BeforeEach(func() {
				node = &api.Node{Name: "docs", Nodes: []*api.Node{{Name: "sub", Nodes: []*api.Node{{Name: "a.md", Source: "https://test/a.md"}}}}}
				nodes = []*api.Node{{Name: "r.md", Source: "https://test/r.md"}, {Name: "sub", Nodes: []*api.Node{{Name: "a.md", Source: "https://test/another.md"}}}}
				exp = &api.Node{Name: "docs", Nodes: []*api.Node{{Name: "sub", Nodes: []*api.Node{{Name: "a.md", Source: "https://test/a.md"}}}, {Name: "r.md", Source: "https://test/r.md"}}}
			})
			It("keeps the explicitly defined one", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(node).To(Equal(exp))
			})
		})
		When("collision with different document and container nodes exists", func() {
			BeforeEach(func() {
				node = &api.Node{Name: "docs", Nodes: []*api.Node{{Name: "sub", Nodes: []*api.Node{{Name: "a", Source: "https://test/a.md"}}}}}
				nodes = []*api.Node{{Name: "r.md", Source: "https://test/r.md"}, {Name: "sub", Nodes: []*api.Node{{Name: "a", Nodes: []*api.Node{{Name: "c", Source: "https://test/another.md"}}}}}}
				exp = &api.Node{Name: "docs", Nodes: []*api.Node{{Name: "sub", Nodes: []*api.Node{{Name: "a", Source: "https://test/a.md"}}}, {Name: "r.md", Source: "https://test/r.md"}}}
			})
			It("keeps the explicitly defined document", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(node).To(Equal(exp))
			})
		})
		When("collision with different container and document nodes exists", func() {
			BeforeEach(func() {
				node = &api.Node{Name: "docs", Nodes: []*api.Node{{Name: "sub", Nodes: []*api.Node{{Name: "a.md", Source: "https://test/a.md"}}}}}
				nodes = []*api.Node{{Name: "r.md", Source: "https://test/r.md"}, {Name: "sub", Source: "https://test/sub.md"}}
				exp = &api.Node{Name: "docs", Nodes: []*api.Node{{Name: "sub", Nodes: []*api.Node{{Name: "a.md", Source: "https://test/a.md"}}}, {Name: "r.md", Source: "https://test/r.md"}}}
			})
			It("keeps the explicitly defined container", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(node).To(Equal(exp))
			})
		})
		When("union on document node", func() {
			BeforeEach(func() {
				node = &api.Node{Name: "doc.md", Source: "https://test/doc.md"}
				nodes = []*api.Node{{Name: "node1.md", Source: "https://test/node1.md"}, {Name: "node2.md", Source: "https://test/node2.md"}}
			})
			It("should error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("doc.md"))
			})
		})
	})
})

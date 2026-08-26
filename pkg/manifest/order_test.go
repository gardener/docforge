// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func fileNames(nodes []*Node) []string {
	out := make([]string, len(nodes))
	for i, n := range nodes {
		out[i] = n.File
	}
	return out
}

func makeWeightedNode(name string, weight interface{}) *Node {
	n := &Node{
		FileType: FileType{File: name},
		Type:     "file",
	}
	if weight != nil {
		n.Frontmatter = map[string]interface{}{"weight": weight}
	}
	return n
}

var _ = Describe("weightOf", func() {
	It("returns 0, false for nil Frontmatter", func() {
		n := &Node{}
		w, ok := weightOf(n)
		Expect(ok).To(BeFalse())
		Expect(w).To(Equal(0))
	})

	It("returns 0, false when weight key is absent", func() {
		n := &Node{Frontmatter: map[string]interface{}{"title": "foo"}}
		w, ok := weightOf(n)
		Expect(ok).To(BeFalse())
		Expect(w).To(Equal(0))
	})

	It("returns int weight", func() {
		n := &Node{Frontmatter: map[string]interface{}{"weight": 5}}
		w, ok := weightOf(n)
		Expect(ok).To(BeTrue())
		Expect(w).To(Equal(5))
	})

	It("returns float64 weight converted to int", func() {
		n := &Node{Frontmatter: map[string]interface{}{"weight": float64(7)}}
		w, ok := weightOf(n)
		Expect(ok).To(BeTrue())
		Expect(w).To(Equal(7))
	})

	It("returns 0, false for string weight", func() {
		n := &Node{Frontmatter: map[string]interface{}{"weight": "heavy"}}
		w, ok := weightOf(n)
		Expect(ok).To(BeFalse())
		Expect(w).To(Equal(0))
	})

	It("returns 0, false for bool weight", func() {
		n := &Node{Frontmatter: map[string]interface{}{"weight": true}}
		w, ok := weightOf(n)
		Expect(ok).To(BeFalse())
		Expect(w).To(Equal(0))
	})
})

var _ = Describe("resolveOrder", func() {
	It("preserves manifest order when no children have weight", func() {
		parent := &Node{DirType: DirType{Structure: []*Node{
			makeWeightedNode("a", nil),
			makeWeightedNode("b", nil),
			makeWeightedNode("c", nil),
		}}}
		_, err := resolveOrder(parent, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(fileNames(parent.Structure)).To(Equal([]string{"a", "b", "c"}))
	})

	It("sorts all-weighted children ascending by weight", func() {
		parent := &Node{DirType: DirType{Structure: []*Node{
			makeWeightedNode("c", 30),
			makeWeightedNode("a", 10),
			makeWeightedNode("b", 20),
		}}}
		_, err := resolveOrder(parent, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(fileNames(parent.Structure)).To(Equal([]string{"a", "b", "c"}))
	})

	It("places weighted children before unweighted, preserving unweighted manifest order", func() {
		parent := &Node{DirType: DirType{Structure: []*Node{
			makeWeightedNode("u1", nil),
			makeWeightedNode("w2", 20),
			makeWeightedNode("u2", nil),
			makeWeightedNode("w1", 10),
		}}}
		_, err := resolveOrder(parent, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(fileNames(parent.Structure)).To(Equal([]string{"w1", "w2", "u1", "u2"}))
	})

	It("is stable for equal weights, falling back to manifest order", func() {
		parent := &Node{DirType: DirType{Structure: []*Node{
			makeWeightedNode("first", 10),
			makeWeightedNode("second", 10),
			makeWeightedNode("third", 10),
		}}}
		_, err := resolveOrder(parent, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(fileNames(parent.Structure)).To(Equal([]string{"first", "second", "third"}))
	})

	It("accepts float64 weight as YAML often produces", func() {
		parent := &Node{DirType: DirType{Structure: []*Node{
			makeWeightedNode("b", float64(20)),
			makeWeightedNode("a", float64(10)),
		}}}
		_, err := resolveOrder(parent, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(fileNames(parent.Structure)).To(Equal([]string{"a", "b"}))
	})

	It("treats string or bool weight as absent without panicking", func() {
		parent := &Node{DirType: DirType{Structure: []*Node{
			makeWeightedNode("a", "heavy"),
			makeWeightedNode("b", true),
			makeWeightedNode("c", nil),
		}}}
		Expect(func() {
			_, _ = resolveOrder(parent, nil, nil)
		}).ToNot(Panic())
		Expect(fileNames(parent.Structure)).To(Equal([]string{"a", "b", "c"}))
	})

	It("does not panic with nil Frontmatter on children", func() {
		parent := &Node{DirType: DirType{Structure: []*Node{
			{FileType: FileType{File: "x"}, Type: "file"},
			{FileType: FileType{File: "y"}, Type: "file"},
		}}}
		Expect(func() {
			_, _ = resolveOrder(parent, nil, nil)
		}).ToNot(Panic())
	})

	It("exits early and returns false when fewer than 2 children", func() {
		single := &Node{DirType: DirType{Structure: []*Node{makeWeightedNode("only", 5)}}}
		changed, err := resolveOrder(single, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeFalse())

		empty := &Node{}
		changed, err = resolveOrder(empty, nil, nil)
		Expect(err).ToNot(HaveOccurred())
		Expect(changed).To(BeFalse())
	})
})

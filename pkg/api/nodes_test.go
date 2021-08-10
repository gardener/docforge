// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"reflect"
	"testing"
)

//       A                    A1
//    /	    \               /
//   B	     C             B1
//  / \	    / \           /
// D   E   F   G         C1
//      \
//       I
// 	      \
// 	       J
func initTestStructure() (*Node, map[string]*Node) {
	idx := make(map[string]*Node)
	jNode := &Node{
		Name: "J",
	}
	idx["J"] = jNode
	iNode := &Node{
		Name: "I",
		Nodes: []*Node{
			jNode,
		},
	}
	idx["I"] = iNode
	eNode := &Node{
		Name: "E",
		Nodes: []*Node{
			iNode,
		},
	}
	idx["E"] = eNode
	dNode := &Node{
		Name: "D",
	}
	idx["D"] = dNode
	bNode := &Node{
		Name: "B",
		Nodes: []*Node{
			dNode,
			eNode,
		},
	}
	idx["B"] = bNode
	gNode := &Node{
		Name: "G",
	}
	idx["G"] = gNode
	fNode := &Node{
		Name: "F",
	}
	idx["F"] = fNode
	cNode := &Node{
		Name: "C",
		Nodes: []*Node{
			fNode,
			gNode,
		},
	}
	idx["C"] = cNode
	aNode := &Node{
		Name: "A",
		Nodes: []*Node{
			bNode,
			cNode,
		},
	}
	aNode.SetParentsDownwards()
	idx["A"] = aNode
	// new tree
	cNode1 := &Node{
		Name: "C1",
	}
	idx["C1"] = cNode1
	bNode1 := &Node{
		Name: "B1",
		Nodes: []*Node{
			cNode1,
		},
	}
	idx["B1"] = bNode1
	aNode1 := &Node{
		Name: "A1",
		Nodes: []*Node{
			bNode1,
		},
	}
	aNode1.SetParentsDownwards()
	idx["A1"] = aNode1
	return aNode, idx
}

func hashOfNodes(names ...string) map[string]*Node {
	h := make(map[string]*Node)
	for _, name := range names {
		h[name] = &Node{Name: name}
	}
	return h
}

func TestParents(t *testing.T) {
	_, idx := initTestStructure()
	cases := []struct {
		description string
		inNode      *Node
		want        []*Node
	}{
		{
			"get parents of node",
			idx["J"],
			[]*Node{idx["A"], idx["B"], idx["E"], idx["I"]},
		},
		{
			"get parents of root",
			idx["A"],
			nil,
		},
	}
	for _, c := range cases {
		t.Run(c.description, func(t *testing.T) {
			got := c.inNode.Parents()
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("parents(%v) == %v, want %v", c.inNode.Name, got, c.want)
			}
		})
	}
}

func TestPath(t *testing.T) {
	_, idx := initTestStructure()
	tests := []struct {
		name     string
		from     *Node
		to       *Node
		expected string
	}{
		{
			"path to self",
			idx["I"],
			idx["I"],
			"I",
		},
		{
			"path to parent node",
			idx["J"],
			idx["I"],
			"../I",
		},
		{
			"path to sibling node",
			idx["D"],
			idx["E"],
			"./E",
		},
		{
			"path to descendent node",
			idx["E"],
			idx["J"],
			"./I/J",
		},
		{
			"path to ancestor node",
			idx["J"],
			idx["E"],
			"../../E",
		},
		{
			"path to node on another branch",
			idx["I"],
			idx["G"],
			"../../C/G",
		},
		{
			"path to root",
			idx["I"],
			idx["A"],
			"../../../A",
		},
		{
			"path from I to A1",
			idx["I"],
			idx["A1"],
			"../../../A1",
		},
		{
			"path from I to C1",
			idx["I"],
			idx["C1"],
			"../../../A1/B1/C1",
		},
		{
			"path from C1 to I",
			idx["C1"],
			idx["I"],
			"../../A/B/E/I",
		},
		{
			"path from A1 to A",
			idx["A1"],
			idx["A"],
			"./A",
		},
		{
			"path from A to A1",
			idx["A"],
			idx["A1"],
			"./A1",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := relativePath(tc.from, tc.to)
			if !reflect.DeepEqual(s, tc.expected) {
				t.Errorf("expected %v !=  %v", tc.expected, s)
			}
		})
	}
}

func TestIntersect(t *testing.T) {
	nodes := hashOfNodes("A", "B", "C", "D", "E", "F")
	tests := []struct {
		name     string
		a        []*Node
		b        []*Node
		expected []*Node
	}{
		{
			"it should have intersection of several elements",
			[]*Node{nodes["A"], nodes["B"], nodes["C"]},
			[]*Node{nodes["D"], nodes["B"], nodes["C"]},
			[]*Node{nodes["B"], nodes["C"]},
		},
		{
			"it should have intersection of one element",
			[]*Node{nodes["A"], nodes["B"], nodes["C"]},
			[]*Node{nodes["D"], nodes["E"], nodes["C"]},
			[]*Node{nodes["C"]},
		},
		{
			"it should have no intersection",
			[]*Node{nodes["A"], nodes["B"], nodes["C"]},
			[]*Node{nodes["D"], nodes["E"], nodes["F"]},
			[]*Node{},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := intersect(tc.a, tc.b)
			if !reflect.DeepEqual(got, tc.expected) {
				t.Errorf("expected %v !=  %v", tc.expected, got)
			}
		})
	}
}

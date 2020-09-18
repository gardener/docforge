// Copyright (c) 2018 SAP SE or an SAP affiliate company. All rights reserved.
// This file is licensed under the Apache Software License, v.2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package api

import (
	"reflect"
	"testing"
)

//       A
//    /	    \
//   B	     C
//  / \	    / \
// D   E   F   G
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
	return aNode, idx
}

func arrayOfNodes(names ...string) []*Node {
	n := make([]*Node, len(names))
	for _, name := range names {
		n = append(n, &Node{Name: name})
	}
	return n
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
			[]*Node{},
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
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := relativePath(tc.from, tc.to)
			// fmt.Println(s)
			if !reflect.DeepEqual(s, tc.expected) {
				t.Errorf("expected %v !=  %v", tc.expected, s)
			}
		})
	}
}

func TestIntersect(t *testing.T) {
	tests := []struct {
		name     string
		a        []*Node
		b        []*Node
		expected []*Node
	}{
		{
			"it should have intersection of several elements",
			arrayOfNodes("A", "B", "C"),
			arrayOfNodes("D", "B", "C"),
			arrayOfNodes("B", "C"),
		},
		{
			"it should have intersection of one element",
			arrayOfNodes("A", "B", "C"),
			arrayOfNodes("D", "E", "C"),
			arrayOfNodes("C"),
		},
		{
			"it should have no intersection",
			arrayOfNodes("A", "B", "C"),
			arrayOfNodes("D", "E", "F"),
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

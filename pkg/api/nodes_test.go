// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestNode_Union(t *testing.T) {
	type fields struct {
		Name             string
		Source           string
		ContentSelectors []ContentSelector
		Template         *Template
		Nodes            []*Node
		NodeSelector     *NodeSelector
		Properties       map[string]interface{}
		Links            *Links
		parent           *Node
		stats            []*Stat
	}
	type args struct {
		nodes           []*Node
		generateNewName func(node *Node) string
	}
	tests := []struct {
		name          string
		fields        fields
		args          args
		expectedNodes []*Node
	}{
		{
			name:   "node_without_nodes_appends_new_successsfully",
			fields: fields{},
			args: args{
				nodes: []*Node{
					{
						Name:   "newNode.md",
						Source: "github.com/ga/ku/blob/main/node.md",
					},
				},
				generateNewName: GenerateNewName,
			},
			expectedNodes: []*Node{
				{
					Name:   "newNode.md",
					Source: "github.com/ga/ku/blob/main/node.md",
				},
			},
		},
		{
			name: "union_skips_duplicates",
			fields: fields{
				Nodes: []*Node{
					{
						Name:   "newNode.md",
						Source: "github.com/ga/ku/blob/main/node.md",
					},
				},
			},
			args: args{
				nodes: []*Node{
					{
						Name:   "newNode.md",
						Source: "github.com/ga/ku/blob/main/node.md",
					},
				},
				generateNewName: GenerateNewName,
			},
			expectedNodes: []*Node{
				{
					Name:   "newNode.md",
					Source: "github.com/ga/ku/blob/main/node.md",
				},
			},
		},
		{
			name: "successfully_merges_nodes_from_both_lists_skips_duplicates",
			fields: fields{
				Nodes: []*Node{
					{
						Name:   "newNode.md",
						Source: "github.com/ga/ku/blob/main/node.md",
					},
					{
						Name:   "node_01.md",
						Source: "github.com/ga/ku/blob/main/node_01.md",
					},
				},
			},
			args: args{
				nodes: []*Node{
					{
						Name:   "newNode.md",
						Source: "github.com/ga/ku/blob/main/node.md",
					},
					{
						Name:   "node_02.md",
						Source: "github.com/ga/ku/blob/main/node_02.md",
					},
				},
				generateNewName: GenerateNewName,
			},
			expectedNodes: []*Node{
				{
					Name:   "newNode.md",
					Source: "github.com/ga/ku/blob/main/node.md",
				},
				{
					Name:   "node_01.md",
					Source: "github.com/ga/ku/blob/main/node_01.md",
				},
				{
					Name:   "node_02.md",
					Source: "github.com/ga/ku/blob/main/node_02.md",
				},
			},
		},
		{
			name: "successfully_generates_name_for_duplicate_node",
			fields: fields{
				Nodes: []*Node{
					{
						Name:   "newNode.md",
						Source: "github.com/ga/ku/blob/main/node.md",
					},
					{
						Name:   "node_01.md",
						Source: "github.com/ga/ku/blob/main/node_01.md",
					},
				},
			},
			args: args{
				nodes: []*Node{
					{
						Name:   "newNode.md",
						Source: "github.com/ga/ku/blob/main/not_equal_node.md",
					},
					{
						Name:   "node_02.md",
						Source: "github.com/ga/ku/blob/main/node_02.md",
					},
				},
				generateNewName: func(node *Node) string {
					return "mocknode.md"
				},
			},
			expectedNodes: []*Node{
				{
					Name:   "mocknode.md",
					Source: "github.com/ga/ku/blob/main/not_equal_node.md",
				},
				{
					Name:   "newNode.md",
					Source: "github.com/ga/ku/blob/main/node.md",
				},
				{
					Name:   "node_01.md",
					Source: "github.com/ga/ku/blob/main/node_01.md",
				},
				{
					Name:   "node_02.md",
					Source: "github.com/ga/ku/blob/main/node_02.md",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &Node{
				Name:             tt.fields.Name,
				Source:           tt.fields.Source,
				ContentSelectors: tt.fields.ContentSelectors,
				Template:         tt.fields.Template,
				Nodes:            tt.fields.Nodes,
				NodeSelector:     tt.fields.NodeSelector,
				Properties:       tt.fields.Properties,
				Links:            tt.fields.Links,
				parent:           tt.fields.parent,
				stats:            tt.fields.stats,
			}

			n.Union(tt.args.nodes, tt.args.generateNewName)

			assert.Equal(t, len(tt.expectedNodes), len(n.Nodes))
			for _, node := range tt.expectedNodes {
				assert.Contains(t, n.Nodes, node)
			}
		})
	}
}

func TestGenerateNewName(t *testing.T) {
	type args struct {
		node *Node
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GenerateNewName(tt.args.node); got != tt.want {
				t.Errorf("GenerateNewName() = %v, want %v", got, tt.want)
			}
		})
	}
}

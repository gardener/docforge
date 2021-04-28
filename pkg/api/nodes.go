// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

import (
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parent returns the parent node (if any) of this node n
func (n *Node) Parent() *Node {
	return n.parent
}

// SetParent assigns a parent node reference to node n to form upstream hierarchy
func (n *Node) SetParent(node *Node) {
	n.parent = node
}

// Parents returns the path of nodes from this nodes parent to the root of the
// hierarchy
func (n *Node) Parents() []*Node {
	var parent *Node
	if parent = n.parent; parent == nil {
		return nil
	}
	return append(parent.Parents(), parent)
}

// SetParentsDownwards walks recursively the hierarchy under this node to set the
// parent property.
func (n *Node) SetParentsDownwards() {
	if len(n.Nodes) > 0 {
		for _, child := range n.Nodes {
			child.parent = n
			child.SetParentsDownwards()
		}
	}
}

// RelativePath returns the relative path between two nodes on the same tree,
// formatted with `..` for ancestors path if any and `.` for current node in relative
// path to descendant. The function can also calculate path to a node on another
// branch
func (n *Node) RelativePath(to *Node) string {
	return relativePath(n, to)
}

func relativePath(from, to *Node) string {
	if from == to {
		return from.Name
	}
	fromPathToRoot := append(from.Parents(), from)
	toPathToRoot := append(to.Parents(), to)
	if intersection := intersect(fromPathToRoot, toPathToRoot); len(intersection) > 0 {
		// to is descendant
		if intersection[len(intersection)-1] == from {
			toPathToRoot = toPathToRoot[(len(intersection) - 1):]
			s := []string{}
			for _, n := range toPathToRoot {
				s = append(s, n.Name)
			}
			s[0] = "."
			return strings.Join(s, "/")
		}
		// to is ancestor
		if intersection[len(intersection)-1] == to {
			fromPathToRoot = fromPathToRoot[(len(intersection)):]
			s := []string{}
			for range fromPathToRoot {
				s = append(s, "..")
			}
			s = append(s, to.Name)
			return strings.Join(s, "/")
		}
		// to is on another branch
		fromPathToRoot = fromPathToRoot[len(intersection):]
		s := []string{}
		if len(fromPathToRoot) > 1 {
			for range fromPathToRoot[1:] {
				s = append(s, "..")
			}
		} else {
			// sibling
			s = append(s, ".")
		}
		toPathToRoot = toPathToRoot[len(intersection):]
		for _, n := range toPathToRoot {
			s = append(s, n.Name)
		}
		return strings.Join(s, "/")
	}
	return ""
}

func intersect(a, b []*Node) []*Node {
	intersection := make([]*Node, 0)
	hash := make(map[*Node]struct{})
	for _, v := range a {
		hash[v] = struct{}{}
	}
	for _, v := range b {
		if _, found := hash[v]; found {
			intersection = append(intersection, v)
		}
	}
	return intersection
}

// GetRootNode returns the root node in the parents path
// for a node object n
func (n *Node) GetRootNode() *Node {
	parentNodes := n.Parents()
	if len(parentNodes) > 0 {
		return parentNodes[0]
	}
	return nil
}

// Peers returns the peer nodes of the node
func (n *Node) Peers() []*Node {
	var parent *Node
	if parent = n.Parent(); parent == nil {
		return nil
	}
	peers := []*Node{}
	for _, node := range parent.Nodes {
		if node != n {
			peers = append(peers, node)
		}
	}
	return peers
}

// GetStats returns statistics for this node
func (n *Node) GetStats() []*Stat {
	return n.stats
}

// AddStats appends Stats
func (n *Node) AddStats(s ...*Stat) {
	for _, stat := range s {
		n.stats = append(n.stats, stat)
	}
}

// FindNodeBySource traverses up and then all around the
// tree paths in the node's documentation structure, looking for
// a node that has the source string either in source, contentSelector
// or template
func FindNodeBySource(source string, node *Node) *Node {
	if node == nil {
		return nil
	}
	if n := matchAnySource(source, node); n != nil {
		return n
	}
	root := node.GetRootNode()
	if root == nil {
		root = node
	}
	return withMatchinContentSelectorSource(source, root)
}

func matchAnySource(source string, node *Node) *Node {
	if node.Source == source {
		return node
	}
	for _, contentSelector := range node.ContentSelectors {
		if contentSelector.Source == source {
			return node
		}
	}
	if t := node.Template; t != nil {
		for _, contentSelector := range t.Sources {
			if contentSelector.Source == source {
				return node
			}
		}
	}
	return nil
}

func withMatchinContentSelectorSource(source string, node *Node) *Node {
	if node == nil {
		return nil
	}
	if n := matchAnySource(source, node); n != nil {
		return n
	}

	for i := range node.Nodes {
		foundNode := withMatchinContentSelectorSource(source, node.Nodes[i])
		if foundNode != nil {
			return foundNode
		}
	}

	return nil
}

// SortNodesByName recursively sorts all child nodes in the
// node hierarchy by node Name
func SortNodesByName(node *Node) {
	if nodes := node.Nodes; nodes != nil {
		sort.Slice(nodes, func(i, j int) bool {
			return nodes[i].Name > nodes[j].Name
		})
		for _, n := range nodes {
			SortNodesByName(n)
		}
	}
}

func (n *Node) String() string {
	node, err := yaml.Marshal(n)
	if err != nil {
		return ""
	}
	return string(node)
}

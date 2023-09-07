// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manifestadapter

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	// CachedNodeContent - key used to store Node content into properties
	CachedNodeContent = "\x00cachedNodeContent"
	// ContainerNodeSourceLocation - key used to store container Node source location into properties
	ContainerNodeSourceLocation = "\x00containerNodeSourceLocation"
	// NodeResourceSHA - key used to store Source resource SHA for later use in https://developer.github.com/v3/git/blobs/#get-a-blob
	NodeResourceSHA = "\x00nodeResourceSHA"
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

// Path serializes the node parents path to root
// as string of segments that are the parents names
// and delimited by separator
func (n *Node) Path(separator string) string {
	var pathSegments []string
	for _, parent := range n.Parents() {
		pathSegments = append(pathSegments, parent.Name)
	}
	return strings.Join(pathSegments, separator)
}

// FullName returns fully qualified name of this node
// i.e. Node.Path + Node.Name
func (n *Node) FullName(separator string) string {
	return fmt.Sprintf("%s%s%s", n.Path(separator), separator, n.Name)
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

// IsDocument returns true if the node is a document node
func (n *Node) IsDocument() bool {
	return len(n.MultiSource) > 0 || len(n.Source) > 0
}

// RelativePath returns the relative path between two nodes on the same tree or the forest under a Documentation.Structure,
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
			var s []string
			for _, n := range toPathToRoot {
				s = append(s, n.Name)
			}
			s[0] = "."
			return strings.Join(s, "/")
		}
		// to is ancestor
		if intersection[len(intersection)-1] == to {
			fromPathToRoot = fromPathToRoot[(len(intersection)):]
			var s []string
			for range fromPathToRoot {
				s = append(s, "..")
			}
			s = append(s, to.Name)
			return strings.Join(s, "/")
		}
		// to is on another branch
		fromPathToRoot = fromPathToRoot[len(intersection):]
		var s []string
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
	// the nodes are in different trees
	// (e.g. the roots of the nodes are different elements in the api#Documentation.Structure array)
	var s []string
	if len(fromPathToRoot) > 1 {
		for range fromPathToRoot[1:] {
			s = append(s, "..")
		}
	} else {
		s = append(s, ".")
	}
	for _, n := range toPathToRoot {
		s = append(s, n.Name)
	}
	return strings.Join(s, "/")
}

func intersect(a, b []*Node) []*Node {
	var intersection []*Node
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

func (n *Node) String() string {
	node, err := yaml.Marshal(n)
	if err != nil {
		return ""
	}
	return string(node)
}

// Serialize marshals the given documentation and transforms it to string
func Serialize(docs *Documentation) (string, error) {
	var (
		err error
		b   []byte
	)
	if b, err = yaml.Marshal(docs); err != nil {
		return "", err
	}
	return string(b), nil
}

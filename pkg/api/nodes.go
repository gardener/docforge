package api

import (
	"strings"
)

// Parent returns the parent node (if any) of this node n
func (n *Node) Parent() *Node {
	return n.parent
}

// SetParent returns the parent node (if any) of this node n
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

// Returns the relative path between two nodes on the same tree, formatted
// with `..` for ancestors path if any and `.` for current node in relative
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

func (n *Node) GetRootNode() *Node {
	parentNodes := n.Parents()
	if len(parentNodes) > 0 {
		return parentNodes[0]
	}
	return nil
}

func FindNodeByContentSource(nodeContentSource string, node *Node) *Node {
	if node == nil {
		return nil
	}

	for _, contentSelector := range node.ContentSelectors {
		if contentSelector.Source == nodeContentSource {
			return node
		}
	}

	return withMatchinContentSelectorSource(nodeContentSource, node.GetRootNode())
}

func withMatchinContentSelectorSource(nodeContentSource string, node *Node) *Node {
	if node == nil {
		return nil
	}
	for _, contentSelector := range node.ContentSelectors {
		if contentSelector.Source == nodeContentSource {
			return node
		}
	}

	for i := range node.Nodes {
		foundNode := withMatchinContentSelectorSource(nodeContentSource, node.Nodes[i])
		if foundNode != nil {
			return foundNode
		}
	}

	return nil
}

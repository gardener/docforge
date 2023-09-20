// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Parent returns the parent node (if any) of this node n
func (n *Node) Parent() *Node {
	return n.parent
}

// FullName returns fully qualified name of this node
// i.e. Node.Path + Node.Name
func (n *Node) FullName() string {
	return n.Path + "/" + n.Name()
}

// Name is the name of the node
func (n *Node) Name() string {
	switch n.Type {
	case "file":
		return n.File
	case "dir":
		return n.Dir
	default:
		return ""
	}
}

// IsDocument returns true if the node is a document node
func (n *Node) IsDocument() bool {
	return len(n.MultiSource) > 0 || len(n.Source) > 0
}

// RelativePath returns the relative path betwee two nodes
func (n *Node) RelativePath(to *Node) string {
	p, _ := filepath.Rel(n.Path, to.Path)
	return p
}

func (n *Node) String() string {
	node, err := yaml.Marshal(n)
	if err != nil {
		return ""
	}
	return string(node)
}

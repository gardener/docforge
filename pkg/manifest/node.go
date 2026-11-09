// SPDX-FileCopyrightText: Copyright Contributors to the Gardener project
//
// SPDX-License-Identifier: Apache-2.0

package manifest

import (
	"github.com/gardener/docforge/pkg/internal/link"
	"github.com/gardener/docforge/pkg/internal/must"
	"gopkg.in/yaml.v3"
)

// Node represents a generic mnifest node
type Node struct {
	ManifType `yaml:",inline"`

	FileType `yaml:",inline"`

	DirType `yaml:",inline"`

	FilesTreeType `yaml:",inline"`

	// Proccessor determines which node processor to use
	Processor string `yaml:"processor,omitempty"`
	// Frontmatter of the node
	Frontmatter map[string]interface{} `yaml:"frontmatter,omitempty"`
	// Type of node
	Type string `yaml:"type,omitempty"`
	// Path of node
	Path string `yaml:"path,omitempty"`
	// LinkResolution describes how links should be resolved when processing the given node
	LinkResolution map[string]string `yaml:"linkResolution,omitempty"`
	// AllowScriptBlocks opts this node's content into the trusted content policy (exempt from script/style stripping)
	AllowScriptBlocks bool `yaml:"allowScriptBlocks,omitempty"`
	// Parent of node
	parent *Node
	// trusted is computed by makeComputeTrust during manifest resolution; cannot be set via YAML
	trusted bool
	// trustManifest is the URL of the root manifest that granted trust to this node
	trustManifest string
	// trustDeclarant points to the node where allowScriptBlocks was directly declared.
	// Set during Phase A; the pointer remains valid after Phase B removes fileTree nodes.
	trustDeclarant *Node
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

// NodePath returns fully qualified name of this node
// i.e. Node.Path + Node.Name
func (n *Node) NodePath() string {
	return must.Succeed(link.Build(n.Path, n.Name()))
}

// HasContent returns true if the node is a document node
func (n *Node) HasContent() bool {
	return len(n.MultiSource) > 0 || len(n.Source) > 0
}

// Parent is the node parent
func (n *Node) Parent() *Node {
	return n.parent
}

// AllowsScriptBlocks reports whether this node is trusted (exempt from the content policy).
// Trust is computed during manifest resolution and cannot be set via YAML.
func (n *Node) AllowsScriptBlocks() bool {
	return n.trusted
}

// RemoveParent removes parent of node
func (n *Node) RemoveParent() {
	n.parent = nil
}

func (n *Node) String() string {
	node, err := yaml.Marshal(n)
	if err != nil {
		return ""
	}
	return string(node)
}

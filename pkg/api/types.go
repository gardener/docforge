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

// Documentation is a documentation structure that can be serialized and deserialized
// and parsed into a model supporting the tasks around building a concrete documentaiton
// bundle.
type Documentation struct {
	// Root is the root node of this documentation structure
	Root *Node `yaml:"root"`
	// Variables are a set of key-value entries, where the key is the variable name
	// and the value is a node template. Nodes defined as variables can be resused
	// by reference throughout the documentation structure to minimise duplicate
	// node definitions. A reference to a variable is in the format `$variable-name`,
	// where `varaible-name` is a key in this Variables map structure.
	//
	// Note: WiP - proposed, not implemented yet.
	Variables map[string]*Node `yaml:"variables,omitempty"`
}

// Node is a recursive, tree data structure representing documentation model.
type Node struct {
	// Title is the title for a node displayed to human users
	Title string `yaml:"title,omitempty"`
	// Source is a sequence of path specifications to locate the resources
	// that represent this document node. There must be at minimum one. When
	// they are multiple, the resulting document is an aggregation of the
	// material located at each path.
	//
	// A source path specification entries are in the following format:
	// `path[#{semantic-block-selector}]`, where:
	// - `path` is a valid resource locator for a document.
	// - `semantic-block-selector`is an expression that selects semantic block
	//   elements from the document similiar to CSS selectors (Note: WiP - proposed,
	//	 not implemented yet.).
	//
	// Examples:
	// - A single file
	//   `source: ["path/a/b/c/file.md"]`
	//
	// - Two files in order to construct a new document
	//   `source: ["path1/a/b/c/file1.md",
	//             "path2/e/f/g/file2.md"]`
	//
	// - A file and the section under the first heading level 1 from another file
	//   in that order to construct a new document.
	//   Note: WiP - proposed, not implemented yet.
	//   `source: ["path1/a/b/c/file1.md",
	//             "path2/e/f/g/file2.md#{h1:first-of-type}"]`
	Source []string `yaml:"source,omitempty"`
	// Nodes is an array of nodes that are subnodes (children) of this node
	//
	// Note: For a non-strict alternative for specifying child nodes, refer to
	//       `NodesSelector`
	Nodes []*Node `yaml:"nodes,omitempty"`
	// NodesSelector is a structure modeling an existing structure of documents at a
	// location that can be further filtered by their metadata propertis and set as
	// child nodes to this node. This is an alternative to explicitly setting child
	// nodes structure resource paths with `Nodes`.
	// Note: WiP - proposed, not implemented yet.
	NodeSelector *NodeSelector `yaml:"nodesSelector,omitempty"`
	// Properties are a map of arbitary, key-value pairs to model custom,
	// untyped node properties. They could be used to instruct specific ResourceHandlers
	// and the serialization of the Node. For example the properyies member could be
	// used to set the front-matter to markdowns for front-matter aware builders such
	// as Hugo.
	Properties map[string]interface{} `yaml:"properties,omitempty"`
	// Name is the name of this node. If omited, the name is the resource name from
	// Source as reported by an eligible ResourceHandler's Name() method.
	// Node with multiple Source entries require name.
	Name string `yaml:"name,omitempty"`
	// A reference to the parent of this node, unless it is the root. Unexported and
	// assigned internally when the node structure is resolved. Not marshalled.
	parent *Node
}

// NodeSelector is an specification for selecting subnodes (children) for a node.
// The order in which the documents are selected is not guaranteed. The interpreters
// of NodeSelectors can make use of the resource metadata or other sources to construct
// and populate child Nodes dynamically.
//
// Example:
// - Select all documents located at path/a/b/c that have front-matter property
//   `type` with value `faq`:
//   ```
//  nodesSelector: {
//	  path: "path/a/b/c",
//	  annotation: "type:faq"
//	}
//  ```
//  will select markdown documents located at path/a/b/c with front-matter:
//  ---
//  type: faq
//  ---
//
// Note: WiP - proposed, not implemented yet.
type NodeSelector struct {
	// Path is a resource locator to a set of files, i.e. to a resource container.
	Path string `yaml:"path"`
	// Depth a maximum depth of the recursion. If omitted or less than 0, the
	// constraint is not considered
	Depth int64 `yaml:"depth,omitempty"`
	// Annotation is an optional expression, filtering documents located at `Path`
	// by their metadata properties. Markdown metadata is commonly provisioned as
	// `front-matter` block at the head of the document delimited by comment
	// tags (`---`).
	Annotation string `yaml:"annotation,omitempty"`
}

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
func (n *Node) SetParentsDownwards(node *Node) {
	if len(node.Nodes) > 0 {
		for _, n := range node.Nodes {
			n.parent = node
			n.SetParentsDownwards(n)
		}
	}
}

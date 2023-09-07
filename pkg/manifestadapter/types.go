// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package manifestadapter

// The Documentation element represents a manifest document and the top-level element
// in the model. A Documentation must contain at least one of:
// - Structure
// - NodeSelector
type Documentation struct {
	// The root of this Documentation hierarchy that contains the top-level nodes of
	// the structure. Physically, it translates to the path provided with the
	// `--destination` flag to docforge.
	//
	// Optional, if NodeSelector is set
	Structure []*Node `yaml:"structure,omitempty"`
}

// A Node is a node in a documentation structure.
// A Node with a `Nodes` property is a *container node*. Container nodes are
// recursive. They can contain other nodes in their `Nodes` property, which in turn
// can contain other nodes and form a tree hierarchy in this way. On a file system
// it is serialized as directory.
// A Node that defines either of the content assignment properties -  `Source` or
// `MultiSource` is a *document node*. The twо properties are alternatives.
// Only one can be used in a node. On a file system a document node
// is serialized as file.
type Node struct {
	// Name is an identifying string for this node that will be used also for its
	// serialization.
	// If this is a document node that defines a Source property, and Name is not
	// explicitly defined, the Name is inferred to be the resource name in the Source
	// location.
	// The Name value of a node that defines Source property can be an expression
	// constructed from several variables:
	//
	// - `$name`: the original name of the resource provided by Source
	// - `$ext`: the extension of the resource provided by Source. May be empty string
	//   if the resource has no extension.
	// - `$uuid`: a UUID identifier generated and at disposal for each node.
	//
	// Example: `name: $name-$uuid$ext`
	//
	// Optional if Source is specified, Mandatory otherwise
	Name string `yaml:"name,omitempty"`
	// Source declares a content assignment to this node from a single location.
	//
	// Mandatory if this is a document node and MultiSource is not specified.
	// Applicable to document nodes only.
	Source string `yaml:"source,omitempty"`
	// MultiSource is a sequence of contents for this document node from different locations.
	//
	// The content provided by the list of MultiSource is aggregated into a single
	// document in the order in which they are declared.
	// Mandatory if this is a document node and Source is not specified.
	// Applicable to document nodes only.
	// Alternative to Source.
	MultiSource []string `yaml:"multiSource,omitempty"`
	// Nodes is a list of nodes that are descendants of this Node in the
	// documentation structure.
	// Applicable to container nodes only.
	Nodes []*Node `yaml:"nodes,omitempty"`
	// Properties are a map of arbitrary, key-value pairs to model custom, untyped
	// node properties. The requirements and constraints on the properties depends
	// on the feature that makes use of them.
	// For example, specifying a "frontmatter"
	// property on a node will result in applying the value as front matter in the
	// resulting document content. When Hugo processors are applied this can be
	// applied not only on document, but also on container nodes.
	//
	// Optional
	Properties map[string]interface{} `yaml:"properties,omitempty"`

	// private fields
	parent *Node
}

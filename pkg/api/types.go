// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package api

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
	// Declares rules for dynamic resolution of nodes into a Structure. Can be defined
	// together with Structure to combine implicit and explicit definition of structure.
	//
	// Optional, if Structure is set
	NodeSelector *NodeSelector `yaml:"nodesSelector,omitempty"`
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
	// NodesSelector specifies a NodeSelector to be used for dynamic
	// resolution of nodes into descendants of this container node. The modelled
	// structure is merged into this node's *Nodes* field, mashing it up with
	// potentially explicitly defined descendants there. The merge strategy
	// identifies identical nodes by their name and when there is a match, it performs
	// a deep merge of their properties. When there are merger conflicts, the
	// explicitly defined node wins.
	// Depending on the goal, a NodeSelector can coexist, or be an alternative to an
	// explicitly defined structure.
	//
	// Applicable to container nodes only.
	NodeSelector *NodeSelector `yaml:"nodesSelector,omitempty"`
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

// NodeSelector is a specification for selecting nodes from a location that is
// resolved at runtime dynamically.
type NodeSelector struct {
	// Path specifies the source for generating a node hierarchy. This can be another
	// Documentation manifest or path that can be resolved to file/folder list,
	// and potentially recursively into hierarchy.

	// When Path references a Documentation manifest, it is resolved recursively and
	// reconciled with the including Documentation manifest with regard to links
	// configuration.

	// When Path references a supported resource container (GitHub or Git repository at
	// the moment), the structure of the resource inside will be used to generate a
	// node structure. For GitHub path that is a folder in a GitHub repo, the
	// generated node hierarchy corresponds ot the file/folder structure at that
	// path.

	// Without any further criteria, all nodes within path are included, but
	// optionally nodes can be excluded e.g. by defining constraints on accepted paths
	// or the depth of the hierarchy.
	//
	// Mandatory
	Path string `yaml:"path"`
	// ExcludePath is a set of exclusion rules applied to node candidates based on the
	// node path. Each rule is a regular expression tested to match on each node's
	// path, relative to the Path property.
	//
	// Optional
	ExcludePaths []string `yaml:"excludePaths,omitempty"`
	// Depth is a maximum depth of the recursion for selecting nodes from hierarchy.
	// If omitted or less than 0, the constraint is not considered.
	//
	// Optional
	Depth int32 `yaml:"depth,omitempty"`
	// ExcludeFrontMatter is a set of rules for a `nodesSelector` to **exclude** nodes
	// with compliant front-matter. The compliance is positive if a node matches one or
	// more rules. Applies to document nodes only.

	// If a node is evaluated to be compliant with both `FrontMatter` and
	// `ExcludeFrontMatter` rules, it will be excluded.

	// Markdown metadata is commonly provisioned as `front-matter` block at the head
	// of the document delimited by comment tags (`---`). The supported format of the
	// metadata is YAML.

	// The `ExcludeFrontMatter` rules are mappings between path patterns identifying an
	// element in the front-matter and a value. If the path matches an actual path
	// to an element in the front-matter and the value of this element matches the
	// rule value, there is a positive match.

	// The path patterns are a very simplified form of JSONPath notation.
	// An object in path is modeled as dot (`.`). Paths start with the root object,
	// i.e. the most minimal path is `.`.
	// An object element value is referenced by its name (key) in the object map:
	// `.a.b.c` is path to element `c` in map `b` in map `a` in root object map.
	// Element values can be scalar, object maps or arrays.
	// An element in an array is referenced by its index: `.a.b[1]` references `b`
	//   array element with index 1.
	// Paths can include up to one wildcard `**` symbol that models *any* path node.
	// A `.a.**.c` models any path starting with	`.a.` and ending with `.c`.
	//
	// Optional
	ExcludeFrontMatter map[string]interface{} `yaml:"excludeFrontMatter,omitempty"`
	// FrontMatter is a set of rules for a `nodesSelector` to **include** nodes with
	// compliant front-matter. The compliance is positive if a node matches one or
	// more rules. Applies to document nodes only.

	// If a node is evaluated to be compliant with both `FrontMatter` and
	// `ExcludeFrontMatter` rules, it will be excluded.

	// Markdown metadata is commonly provisioned as `front-matter` block at the head
	// of the document delimited by comment tags (`---`). The supported format of the
	// metadata is YAML.

	// The `FrontMatter` rules are mappings between path patterns identifying an
	// element in the front-matter and a value. If the path matches an actual path
	// to an element in the front-matter and the value of this element matches the
	// rule value, there is a positive match.

	// The path patterns are a very simplified form of JSONPath notation.
	// An object in path is modeled as dot (`.`). Paths start with the root object,
	// i.e. the most minimal path is `.`.
	// An object element value is referenced by its name (key) in the object map:
	// `.a.b.c` is path to element `c` in map `b` in map `a` in root object map.
	// Element values can be scalar, object maps or arrays.
	// An element in an array is referenced by its index: `.a.b[1]` references `b`
	// array element with index 1.
	// Paths can include up to one wildcard `**` symbol that models *any* path node.
	// A `.a.**.c` models any path starting with	`.a.` and ending with `.c`.
	//
	// Optional
	FrontMatter map[string]interface{} `yaml:"frontMatter,omitempty"`
}

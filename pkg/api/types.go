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
	// LocalityDomain defines the scope of the downloadable resources
	// for this structure
	LocalityDomain LocalityDomain `yaml:"localityDomain,omitempty"`
}

// Node is a recursive, tree data structure representing documentation model.
type Node struct {
	parent *Node
	// Name is the name of this node. If omited, the name is the resource name from
	// Source as reported by an eligible ResourceHandler's Name() method.
	// Node with multiple Source entries require name.
	Name string `yaml:"name,omitempty"`
	// A reference to the parent of this node, unless it is the root. Unexported and
	// assigned internally when the node structure is resolved. Not marshalled.
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
	ContentSelectors []ContentSelector `yaml:"contentSelectors,omitempty"`
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
	Properties     map[string]interface{} `yaml:"properties,omitempty"`
	LocalityDomain `yaml:"localityDomain,omitempty"`
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

// ContentSelector specifies a document node content target
type ContentSelector struct {
	// URI of a document
	Source string `yaml:"source,omitempty"`
	// Optional filtering expression that selects content from the document content
	// Omiting this file will select the whole document content.
	Selector *string `yaml:"selector,omitempty"`
}

// LinksMatchers defines links exclusion/inclusion patterns
type LinksMatchers struct {
	// Include is a list of regular expressions that will be matched to every
	// link that is candidate for download to determine whether it is
	// eligible. The links to match are absolute.
	// Include can be used in conjunction with Exclude when it is easier/
	// preferable to deny all resources and allow selectively.
	// Include can be used in conjunction with localityDomain to add
	// additional resources not in the domain.
	Include []string `yaml:"include,omitempty"`
	// Exclude is a list of regular expression that will be matched to every
	// link that is candidate for download to determine whether it is
	// not eligible. The links to match are absolute.
	// Use Exclude to further constrain the set of downloaded resources
	// that are in a locality domain.
	Exclude []string `yaml:"exclude,omitempty"`
}

// LocalityDomain contains the entries defining a
// locality domain scope. Each entry is a mapping
// between a domain, such as github.com/gardener/gardener,
// and a path in it that defines "local" resources.
// Documents referenced by documentation node structure
// are always part of the locality domain. Other
// resources referenced by those documents are checked
// against the path hierarchy of locality domain
// entries to determine how they will be processed.
type LocalityDomain map[string]*LocalityDomainValue

// LocalityDomainValue encapsulates the memebers of a
// LocalityDomain entry value
type LocalityDomainValue struct {
	// Version sets the version of the resources that will
	// be referenced in this domain. Download targets and
	// absolute links in documents referenced by the structure
	// will be rewritten to match this version
	Version string `yaml:"version"`
	// Path is the relative path inside a domain that contains
	// resources considered 'local' that will be downloaded.
	Path          string `yaml:"path"`
	LinksMatchers `yaml:",inline"`
	// LinkSubstitutes is an optional map of links and their
	// substitutions. Use it to override the default handling of those
	// links in documents in this locality domain:
	// - An empty substitution string ("") removes a link markdown
	//   turning. It leaves only its text component in the document
	//   for links and nothing for images.
	//   This applies only to markdown for links and images.
	// - A fixed string that will replace the whole original link
	//   destination.
	LinkSubstitutes Substitutes
	// DownloadSubstitutes is an optional map of resource names in this
	// locality domain and their substitutions. Use it to override the
	// default downloads naming:
	// - An exact download name mapped to a download resource will be used
	//   to name that resources when downloaded.
	// - An expression with substitution variables can be used
	//   to change the default pattern for generating donwloaded resouce
	//   names, which is $uuid.
	//   The supported variables are:
	//   - $name: the original name of the resouce
	//   - $path: the original path of the resource in this domain (may be empty)
	//   - $uuid: the identifier generated for the downloaded resource
	//   Example expression: $name-$uuid
	DownloadSubstitutes Substitutes
}

// Substitutes is map of ...
type Substitutes map[string]string

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

// Documentation models a manifest for building a documentation structure from various
// sources into a coherent bundle.
type Documentation struct {
	// Structure defines a documentation structure hierarchy.
	//
	// Optional, alternative to NodeSelector
	Structure []*Node `yaml:"structure,omitempty"`
	// NodesSelector is a specification for building a documentation structure hierarchy.
	// The root of the hierarchy is a node generated for the resource at the nodeSelector's
	// path. It is attached as single, direct descendant of this Documentation.
	// NodesSelector on this level is useful in a scenario where the intended structure needs
	// not to be modelled but complies with an existing hierarchy that can be resolved to
	// a documentation structure.
	// Note: WiP - proposed, not implemented yet.
	//
	// Optional, alternative to Structure
	NodeSelector *NodeSelector `yaml:"nodesSelector,omitempty"`
	// Links defines global rules for processing document links
	//
	// Optional
	Links *Links `yaml:"links,omitempty"`
	// Variables are a set of key-value entries that allow manifests to be parameterized.
	// When the manifest is resolved, variables values are interpolated throughout the text.
	//
	// Note: WiP - proposed, not implemented yet.
	// Optional
	Variables map[string]interface{} `yaml:"variables,omitempty"`
}

// Node is a recursive, tree data structure representing documentation structure.
// A Node's descendents are its `nodes` array elements.
// A node without any of the options for content assignment - Source, ContentSlectors
// or Template is a container node, and is serialized as folder. If it has a content
// assignment property, it is a document node and is serialized as file.
// Document nodes have a nil Nodes property.
type Node struct {
	// Name is an identifying name for this node that will be used also for its serialization.
	// Name cannot be omitted if this is a container node.
	// Name can be omitted for document nodes only if the Source property is specified. In this
	// case, the Name value is the resource name from the location specified in Source.
	// A document node without Source requires Name.
	// The Name value of a node with Source can be also an expression constructed from several
	// variables:
	// - $name: the  original name of the resource provided by Source
	// - $ext: the extension of the resource provided by Source. May be empty string if the
	//   resource has no extension.
	// - $uuid: a UUID identifier generated  and at disposal for each node.
	//
	// Mandatory if this is a container Node or Source is not specified, optional otherwise
	Name string `yaml:"name,omitempty"`
	// ContentSelectors is a sequence of specifications for selecting cotent for this node.
	// The content provided by the list of ContentSelectors is aggregated into a single document.
	//
	// Mandatory when there is no Name property. Alternative to ContentSelectors and Template. Only
	// one must be specified.
	Source string `yaml:"source,omitempty"`
	// ContentSelectors is a sequence of specifications for selecting cotent for this node.
	// The content provided by the list of ContentSelectors is aggregated into a single document.
	// Name is a required property when ContentSelectors are used to assign content to a node.
	//
	// Optional, alternative to ContentSelectors and Template. Only one of them must be specified.
	ContentSelectors []ContentSelector `yaml:"contentSelectors,omitempty"`
	// Template is a specification for content selection and its application to a template, the
	// product of which is this document node's content.
	// Name is a required property when Template are used to assign content to a node.
	//
	// Optional, alternative to ContentSelectors and Source. Only one of them must be specified.
	Template *Template `yaml:"template,omitempty"`
	// Nodes is a list of nodes that are descendants of this Node. This field is applicable
	// only to container nodes and not to document nodes.
	// A folder node must always have a Name.
	//
	// Note: For a non-strict alternative for specifying child nodes, refer to
	//       `NodesSelector`
	// Optional
	Nodes []*Node `yaml:"nodes,omitempty"`
	// NodesSelector is a specification for building a documentation structure hierarchy,
	// descending from this node. The modelled structure is merged into this node's Nodes
	// field, masshing it up with potentially explicitly defined descendants there. The merge
	// strategy identifies identical nodes by their name and in this case performs a merge
	// of their properties. Where there are conflicts, the explicitly defined node wins.
	// A NodeSelector can coexist or be an alternative to an explicitly defined structure,
	// depending on the goal.
	//
	// Note: WiP - proposed, not implemented yet.
	// Optional
	NodeSelector *NodeSelector `yaml:"nodesSelector,omitempty"`
	// Properties are a map of arbitrary, key-value pairs to model custom,
	// untyped node properties. They can be used for various purposes. For example,
	// specifying a "fronatmatter" property on a node will result in applying the value as
	// front matter in the resulting document content. This si applicable only to document
	// nodes.
	Properties map[string]interface{} `yaml:"properties,omitempty"`
	// Links defines the rules for handling links in this node's content. Applicable only
	// to document nodes.
	Links *Links `yaml:"links,omitempty"`

	// private fields
	parent *Node
	stats  []*Stat
}

// NodeSelector is a specification for selecting a descending hierarchy for a node.
// The order in which the documents are selected is not guaranteed. The interpreters
// of NodeSelectors can make use of the resource metadata or other sources to construct
// and populate descendent Nodes dynamically.
//
// Example:
// - Select recursively all documents located at path /a/b/c that have front-matter
//   property `type` with value `faq`:
//   ```
//   nodesSelector: {
//	   path: "path/a/b/c"
//	   frontMatter:
//       "type:faq"
//	 }
//   ```
//  will select markdown documents located at path/a/b/c with front-matter:
//  ---
//  type: faq
//  ---
//
// Note: WiP - proposed, not implemented yet.
type NodeSelector struct {
	// Path is a resource locator to a set of files, i.e. to a resource container.
	// A node selector path defines the scope that will be used to
	// generate a hierarchy. For GitHub paths that is a folder in a GitHub repo
	// and the generated nodes hierarchy corresponds ot the file/folder structure
	// available in the repository at that path.
	// Without any further criteria, all nodes within path are included.
	//
	// Mandatory
	Path string `yaml:"path"`
	// ExcludePath is a set of exclusion rules for node candidates for the hierarchy.
	// Each rule is a regular expression to match a node's path that is relative to the
	// path element.
	//
	// Optional
	ExcludePaths []string `yaml:"excludePaths,omitempty"`
	// ExcludeFrontMatter is an optional expression, filtering documents located at `Path`
	// by their metadata properties. Markdown metadata is commonly provisioned as
	// `front-matter` block at the head of the document delimited by comment
	// tags (`---`).
	// Documents with front matter that matches all map entries of this field
	// are not selected.
	// Note: WiP - proposed, not implemented yet.
	//
	// Optional
	ExcludeFrontMatter map[string]interface{} `yaml:"excludeFrontMatter,omitempty"`
	// FrontMatter is an optional expression, filtering documents located at `Path`
	// by their metadata properties. Markdown metadata is commonly provisioned as
	// `front-matter` block at the head of the document delimited by comment
	// tags (`---`).
	// Documents with front matter that matches all map entries of this field
	// are selected.
	// Note: WiP - proposed, not implemented yet.
	//
	// Optional
	FrontMatter map[string]interface{} `yaml:"frontMatter,omitempty"`
	// Depth a maximum depth of the recursion. If omitted or less than 0, the
	// constraint is not considered
	//
	// Optional
	Depth int32 `yaml:"depth,omitempty"`
}

// ContentSelector specifies a document node content target
// A ContentSelector specification
// that constitute this document node's content. There must be at minimum one. When
// they are multiple, the resulting document is an aggregation of the
// material located at each path.
//
// A ContentSelector specification entries are in the following format:
// `path[#{semantic-block-selector}]`, where:
// - `path` is a valid resource locator for a document.
// - `semantic-block-selector`is an expression that selects semantic block
//   elements from the document similar to CSS selectors (Note: WiP - proposed,
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
type ContentSelector struct {
	// URI of a document
	//
	// Mandatory
	Source string `yaml:"source,omitempty"`
	// Optional filtering expression that selects content from the document content
	// Omiting this file will select the whole document content at Source.
	//
	// Optional
	Selector *string `yaml:"selector,omitempty"`
}

// Template specifies rules for selecting content and applying it
// to a template
type Template struct {
	// Path to the template file.
	// A template file content is valid Golang template content.
	// See https://golang.org/pkg/text/template.
	// The template will have at disposal the variables defined in
	// this specification's Sources. Their values will be the content
	// selected by the coresponding specifications.
	//
	// Mandatory
	Path string `yaml:"path"`
	// Sources maps variable names to ContentSelectors that will be
	// used as specification for the content to fetch and assign ot that
	// these variables
	Sources map[string]*ContentSelector `yaml:"sources,omitempty"`
}

// Links defines how document links are processed.
type Links struct {
	// Rewrites maps regular expressions matching a document links resolved to absolute,
	// with link rewriting rules.
	// A common use is to rewrite resources links versions, if they support that to have
	// them downloaded at a particular state.
	// A rewrite mapping an expression to nil rules (~) is interpreted as request to remove
	// the links matching the expression.
	Rewrites map[string]*LinkRewriteRule
	// Downloads are definition for document referenced resources that will be downloaded
	// in dedicated destination (__resources by default) and optionally renamed.
	// Downloads are performed after rewrites.
	Downloads *Downloads
}

// LinkRewriteRule si a rule definition specifying link properties to be rewritten.
type LinkRewriteRule struct {
	// Rewrites the version of links matching this pattern, e.g. master -> v1.11.3.
	// For GitHub links the version will rewrite the sha path segment in the URL
	// right after organization, repository and resource type.
	// Note that not every link supports version. For example GitHub issues
	// links have different pattern and it has no sha segment.
	// The version will be applied only where applicable.
	Version *string `yaml:"version,omitempty"`
	// Rewrites the destination in a link|image markdown
	//
	// Example:
	// with `destination: "github.tools.sap/kubernetes/gardener"`
	// [a](github.com/gardener/gardener) -> [a](github.tools.sap/kubernetes/gardener)
	//
	// This setting overwrites a version setting if both exist so it makes little sense to use it
	// with version.
	//
	// Note that destinations that are matched by a downloads specification will be converted to
	// relative, using the result of the destination substitution.
	//
	// Setting destination to empty string leads to removing the link, leaving only the text element behind
	//
	// Example:
	// with `destination: ""` [a](github.com/gardener/gardener) -> a
	//
	// Note that for images this will remove the image entirely:
	//
	// Example:
	// with `destination: ""` ![alt-text-here](github.com/gardener/gardener/blob/master/images/b.png) ->
	//
	Destination *string `yaml:"destination,omitempty"`
	// Rewrites or sets a matched link markdown's text component (alt-text for images)
	// If used in combination with destination: "" and value "" this will effectively remove a link
	// completely, leaving nothing behind in the document.
	Text *string `yaml:"text,omitempty"`
	// Rewrites or sets a matched link markdown's title component.
	// Note that this will have no effect with settings destination: "" and text: "" as the whole
	// markdown together with tis title will be removed.
	Title *string `yaml:"title,omitempty"`
}

// Downloads is a definition of the scope of downloadable resources and rules for renaming them.
type Downloads struct {
	// Renames is a set of renaming rules that are globally applicable to all downloads
	// regardless of scope.
	// Example:
	// renames:
	//   "\\.(jpg|gif|png)": "$name-hires-$uuid.$ext"
	Renames ResourceRenameRules `yaml:"renames,omitempty"`
	// Scope defines the scope for downloaded resources with a set of mappings between
	// document links matching regular expressions and (optional) naming patterns.
	// A scope map entry maps a regular expression that matches document links that will
	// be downloaded to an optional rename specification or ~ for default.
	// If no particular rename specification is supplied:
	// 1. the globally supplied renames are tested to match and applied (if supplied)
	// 2. a default rename expression `$uuid.$ext` will be applied to all matched targets.
	//
	// Example: define a download scope (only) that downloads every matching document.
	// scope:
	//   gardener/gardener/(tree|blob|raw)/master/docs: ~
	//
	// Example: define a download scope that downloads every matching document and
	// renames it to a specific pattern if it is an jpg|gif|png image or uses the default
	// naming pattern otherwise.
	// scope:
	//   gardener/gardener/(tree|blob|raw)/master/docs:
	//     "\\.(jpg|gif|png)": "$name-image-$uuid.$ext"
	Scope map[string]ResourceRenameRules `yaml:"scope,omitempty"`
}

// ResourceRenameRules defines a mapping between regular expressions matching
// resource locators and name pattern expressions or exact names.
// The name patter will be used to rename the downloaded resources matching the
// specified regular expression key.
// There is a set of variables that can be used to construct the
// naming expressions:
// - $name: the original name of the resource
// - $uuid: a UUID generated for the resource
// - $ext: a original resource extension
// The default expression applying to all resources is: $uuid.$ext
//
// Example:
// "\\.(jpg|gif|png)": "$name-image-$uuid.$ext"
//
type ResourceRenameRules map[string]string

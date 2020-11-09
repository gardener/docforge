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
	// Defines the global links configuration in this Documentation. Applies to all
	// nodes, unless overridden.
	//
	// Optional
	Links *Links `yaml:"links,omitempty"`
}

// A Node is a node in a documentation structure.
// A Node with a `Nodes` property is a *container node*. Container nodes are
// recursive. They can contain other nodes in their `Nodes` property, which in turn
// can contain other nodes and form a tree hierarchy in this way. On a file system
// it is serialized as directory.
// A Node that defines either of the content assignment properties -  `Source`,
// `ContentSelectors` or `Template` is a *document node*. The three properties are
// alternatives. Only one can be used in a node. On a file system a document node
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
	// Mandatory if this is a document node and neither ContentSelectors nor
	// Template are specified.
	// Alternative to ContentSelectors and Template.
	// Applicable to document nodes only.
	Source string `yaml:"source,omitempty"`
	// ContentSelectors is a sequence of specifications for selecting content for
	// this document node from different locations. The content provided by the
	// list of ContentSelector is aggregated into a single
	// document in the order in which they are declared. Applicable to document
	// nodes only.
	//
	// Mandatory if this is a document node and neither Source nor Template
	// are specified.
	// Alternative to Source and Template.
	ContentSelectors []ContentSelector `yaml:"contentSelectors,omitempty"`
	// Template defines content assignment to a document node with a
	// Template. Applicable to document nodes only.
	//
	// Mandatory when this is document node and neither Source nor ContentSelectors
	// are specified.
	// Alternative to Source and ContentSelectors.
	Template *Template `yaml:"template,omitempty"`
	// The Nodes property is a list of nodes that are descendants of this Node in the
	// documentation structure. Applicable to container nodes only.
	//
	// Mandatory for container nodes
	Nodes []*Node `yaml:"nodes,omitempty"`
	// NodesSelector specifies a NodeSelector to be used for dynamic
	// resolution of nodes into descendants of this container node. The modelled
	// structure is merged into this node's *Nodes* field, mashing it up with
	// potentially explicitly defined descendants there. The merge strategy
	// identifies identical nodes by their name and when there is a match, it performs
	// a deep merge of their properties. When there are merge conflicts, the
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
	// Links defines the rules for handling links in document nodes content. When
	// defined on a container node, it applies to all descendants unless overridden.
	// When defined on a document node it applies to that node specifically. Links
	// defined on any kind of node override the optional globally specified links in
	// a Documentation.
	//
	// Optional
	Links *Links `yaml:"links,omitempty"`

	// private fields
	parent *Node
	stats  []*Stat
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

	// When Path references a supported resource container (GitHub or file system at
	// the moment), the structure of the resource inside will be used to generate a
	// node structure. For GitHub path that is a folder in a GitHub repo, the
	// generated nodes hierarchy corresponds ot the file/folder structure at that
	// path.

	// Without any further criteria, all nodes within path are included, but
	// optionally nodes can be excluded e.g. by defining constraints on accepted paths
	// or the depth of the hierarchy.
	//
	// Mandatory
	Path string `yaml:"path"`
	// ExcludePath is a set of exclusion rules applied to node candidates based on the
	// nodes path. Each rule is a regular expression tested to match on each node's
	// path, relative to the Path property.
	//
	// Optional
	ExcludePaths []string `yaml:"excludePaths,omitempty"`
	// Depth is a maximum depth of the recursion for selecting nodes from hierarchy.
	// If omitted or less than 0, the constraint is not considered.
	//
	// Optional
	Depth int32 `yaml:"depth,omitempty"`
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
}

// ContentSelector specifies a content assignment that can be optionally filtered
// by a selection criteria.
//
// Note: Selection criteria is not implemented yet
type ContentSelector struct {
	// URI of a document
	//
	// Mandatory
	Source string `yaml:"source"`
	// Optional filtering expression that selects content from the document content
	// Omiting this file will select the whole document content at Source.
	//
	// WiP: proposed, not implemented
	// Optional
	Selector *string `yaml:"selector,omitempty"`
}

// Template specifies rules for selecting content assigned to variables that are
// applied to a template at path.
type Template struct {
	// Path to the template file.
	// A template file content is valid Golang template content. See
	// https://golang.org/pkg/text/template.
	// The template will have at disposal the variables defined in Sources.
	//
	// Mandatory
	Path string `yaml:"path"`
	// Sources maps variable names to [ContentSelector](#contentSelector) rules that
	// filter and assign content from source location.
	//
	// Mandatory
	Sources map[string]*ContentSelector `yaml:"sources"`
}

// Links defines how document links are processed.
//
// At least one of Rewrites or Downloads has to be defined.
type Links struct {
	// Rewrites maps regular expressions matching document links resolved to absolute
	// with link rewriting rules (LinkRewriteRule).
	// A common use is to rewrite resource links versions, if they support that, in
	// order to have them downloaded in a particular version.
	// A rewrite that maps an expression to nil rules (`~`) is interpreted as request
	// to remove the links matching the expression.
	Rewrites map[string]*LinkRewriteRule
	// Downloads defines rules which resources linked from downloaded documents need
	// to be downloaded and optionally renamed. Downloads are performed after rewrites.
	// Links to downloaded resources are rewritten as appropriate to match the new
	// downloaded resource relative position to the referencing documents.
	Downloads *Downloads
}

// LinkRewriteRule specifies link components to be rewritten.
type LinkRewriteRule struct {
	// Rewrites the version of links matching this pattern, e.g. `master` -> `v1.11.3`.
	// For GitHub links the version will rewrite the SHA path segment in the URL
	// right after organization, repository and resource type.
	// Note that not every link supports version. For example GitHub issues links
	// have different pattern and it has no sha segment. The version will be applied
	// only where and however applicable.
	//
	// Optional
	Version *string `yaml:"version,omitempty"`
	// Rewrites the destination in a link|image markdown. An explicitly defined empty
	// string (`""`) indicates that the link markdown will be removed, leaving only
	// the text component behind for links and nothing for images.
	//
	// This setting has precedence over version if both are specified.
	//
	// A link destination rewritten by this rule, which is also matched by a
	// downloads specification, will be converted to	relative, using the result
	// of this destination substitution. The final result therefore may be different
	// from the destination substitution defined here.
	//
	// Optional
	Destination *string `yaml:"destination,omitempty"`
	// Rewrites or sets a matched link markdown text component (alt-text for images)
	// If used in combination with destination: `""` and value `""` this will
	// effectively remove a link completely, leaving nothing behind in the document.
	//
	// Optional
	Text *string `yaml:"text,omitempty"`
	// Rewrites or sets a matched link markdown's title component.
	// Note that this will have no effect with settings destination: `""` and text:
	// `""` as the whole markdown together with tis title will be removed.
	//
	// Optional
	Title *string `yaml:"title,omitempty"`
}

// Downloads is a definition of the scope of downloadable resources and rules for
// renaming them.
type Downloads struct {
	// Renames is a set of renaming rules that are globally applicable to all
	// downloads regardless of scope.
	//
	// Optional
	Renames ResourceRenameRules `yaml:"renames,omitempty"`
	// Scope defines a concrete scope for downloaded resources with a set of mappings
	// between document links matching regular expressions and (optional) naming
	// patterns. A scope map entry maps a regular expression tested to match document
	// links that will be downloaded to an optional rename specification or `~` for
	// default. If no particular rename specification is supplied:

	// 1.  the globally supplied renames are tested to match and applied (if supplied)
	// 2.  a default rename expression `$uuid.$ext` will be applied to all matched
	// 	targets.

	// Example: define a download scope that downloads every matching document.

	// ```
	// scope:
	//   gardener/gardener/(tree|blob|raw)/master/docs: ~
	// ```

	// Example: define a download scope that downloads every matching document and
	// renames it to a specific pattern if it is an jpg|gif|png image or uses the
	// default naming pattern otherwise.

	// ```
	// scope:
	//   gardener/gardener/(tree|blob|raw)/master/docs:
	// 	"\\.(jpg|gif|png)": "$name-image-$uuid.$ext"
	// ```
	//
	// Optional
	Scope map[string]ResourceRenameRules `yaml:"scope,omitempty"`
}

// ResourceRenameRules defines a mapping between regular expressions matching
// resource locators and name pattern expressions or exact names.
// The name pattern will be used to rename the downloaded resources matching the
// specified regular expression key.
// There is a set of variables that can be used to construct the naming
// expressions:
//
// - `$name`: the original name of the resource
// - `$uuid`: a UUID generated for the resource
// - `$ext`: a original resource extension
//   The default expression applying to all resources is: `$uuid$ext`
//   Example:
//
// ```
//  "\\.(jpg|gif|png)": "$name-image-$uuid$ext"
// ```
//
// Optional
type ResourceRenameRules map[string]string

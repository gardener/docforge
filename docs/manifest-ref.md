# Documentation Manifests Reference

This is a reference documentation for the elements and structure rules in a 
docforge manifest.

Table of Contents

- [Documentation Manifests Reference](#documentation-manifests-reference)
  - [Documentation](#documentation)
  - [Node](#node)
  - [NodeSelector](#nodeselector)
  - [ContentSelector](#contentselector)
  - [Template](#template)
  - [Links](#links)
  - [LinkRewriteRule](#linkrewriterule)
  - [Downloads](#downloads)
  - [ResourceRenameRules](#resourcerenamerules)

## Documentation

**Type**: Object

The documentation element represents a manifest document and the top-level element 
in the model.

**Properties**:

- **Structure**  
  Type: Array of [Node](#node)  
  _Optional_, if NodeSelector is set

  The root of this Documentation hierarchy that contains the top-level nodes of 
  the structure. Physically, it translates to the path provided with the 
  `--destination` flag to docforge.

- **NodeSelector**  
  Type: [NodeSelector](#nodeselector)  
  _Optional_, if Structure is set

  Declares rules for dynamic resolution of nodes into a Structure. Can be defined 
  together with Structure to combine implicit and explicit definition of structure.

- **Links**  
  Type: [Links](#links)  
  _Optional_

  Defines the global links configuration in this Documentation. Applies to all 
  nodes, unless overridden.

## Node

**Type**: Object

A Node is a node in a documentation structure.   
A Node with a `Nodes` property is a *container node*. Container nodes are 
recursive. They can contain other nodes in their `Nodes` property, which in turn 
can contain other nodes and form a tree hierarchy in this way. On a file system 
it is serialized as directory.   
A Node that defines either of the content assignment properties -  `Source`, 
`ContentSelectors` or `Template` is a *document node*. The three properties are 
alternatives. Only one can be used in a node. On a file system a document node 
is serialized as file.   

**Properties**:

- **Name**  
  Type: [string](https://golang.org/ref/spec#String_types)  
  *Optional* if Source is specified, _Mandatory_ otherwise

  Name is an identifying string for this node that will be used also for its 
  serialization.
  If this is a document node that defines a Source property, and Name is not 
  explicitly defined, the Name is inferred to be the resource name in the Source 
  location.  
  The Name value of a node that defines Source property can be an expression 
  constructed from several variables:

  - `$name`: the original name of the resource provided by Source
  - `$ext`: the extension of the resource provided by Source. May be empty string 
    if the resource has no extension.
  - `$uuid`: a UUID identifier generated and at disposal for each node.
  
  Example: `name: $name-$uuid$ext`
- **Source**  
  Type: [string](https://golang.org/ref/spec#String_types)  
  *Mandatory* if this is a *document node* and neither ContentSelectors nor 
  Template are specified.   
  *Alternative* to ContentSelectors and Template.  
  Applicable to document nodes only.

  Source declares a content assignment to this node from a single location.

- **ContentSelectors**  
  Type: Array of [ContentSelector](#contentSelector)  
  *Mandatory* if this is a *document node* and neither Source nor Template 
  are specified.  
  *Alternative* to Source and Template.  

  ContentSelectors is a sequence of specifications for selecting content for 
  this document node from different locations. The content provided by the 
  list of [ContentSelector](#contentSelector)s is aggregated into a single 
  document in the order in which they are declared. Applicable to document 
  nodes only.

- **Template**  
  Type: [Template](#template)  
  *Mandatory* when this is document node and neither Source nor ContentSelectors
  are specified.  
  *Alternative* to Source and ContentSelectors.  

  Template defines content assignment to a document node with a 
  [Template](#template). Applicable to document nodes only.

- **Nodes**  
  Type: Array of [Node](#node)  
  Mandatory for container nodes  

  The Nodes property is a list of nodes that are descendants of this Node in the
  documentation structure. Applicable to container nodes only.

- **NodeSelector**  
  Type: [NodeSelector](#nodeselector)  
  Optional
  Applicable to container nodes only.

  NodesSelector specifies a [NodeSelector](#nodeselector) to be used for dynamic 
  resolution of nodes into descendants of this container node. The modelled 
  structure is merged into this node's *Nodes* field, mashing it up with 
  potentially explicitly defined descendants there. The merge strategy 
  identifies identical nodes by their name and when there is a match, it performs 
  a deep merge of their properties. When there are merge conflicts, the 
  explicitly defined node wins.   
  Depending on the goal, a NodeSelector can coexist, or be an alternative to an 
  explicitly defined structure.

- **Properties**  
  Type: Map[string][any]  
  Optional

  Properties are a map of arbitrary, key-value pairs to model custom, untyped 
  node properties. The requirements and constraints on the properties depends 
  on the feature that makes use of them.   
  For example, specifying a "frontmatter" 
  property on a node will result in applying the value as front matter in the 
  resulting document content. When Hugo processors are applied this can be 
  applied not only on document, but also on container nodes.

- **Links**  
  Type: [Links](#links)  
  Optional

  Links defines the rules for handling links in document nodes content. When 
  defined on a container node, it applies to all descendants unless overridden.
  When defined on a document node it applies to that node specifically. Links
  defined on any kind of node override the optional globally specified links in
  a Documentation.

## NodeSelector

**Type**: Object

NodeSelector is a specification for selecting nodes from a location that is 
resolved at runtime dynamically.

**Properties**:

- **Path**  
  Type: [string](https://golang.org/ref/spec#String_types)  
  _Mandatory_

  Path specifies the source for generating a node hierarchy. This can be another
  Documentation manifest or path that can be resolved to file/folder list, 
  and potentially recursively into hierarchy.

  When Path references a Documentation manifest, it is resolved recursively and 
  reconciled with the including Documentation manifest with regard to links 
  configuration.

  When Path references a supported resource container (GitHub or file system at 
  the moment), the structure of the resource inside will be used to generate a 
  node structure. For GitHub path that is a folder in a GitHub repo, the 
  generated nodes hierarchy corresponds ot the file/folder structure at that 
  path.

  Without any further criteria, all nodes within path are included, but 
  optionally nodes can be excluded e.g. by defining constraints on accepted paths 
  or the depth of the hierarchy.

- **ExcludePaths**  
  Type: Array of string
  _Optional_

  ExcludePath is a set of exclusion rules applied to node candidates based on the 
  nodes path. Each rule is a regular expression tested to match on each node's 
  path, relative to the Path property.

- **Depth**  
  Type: [int32](https://golang.org/ref/spec#Numeric_types)  
  _Optional_

  Depth is a maximum depth of the recursion for selecting nodes from hierarchy. 
  If omitted or less than 0, the constraint is not considered.
  
- **FrontMatter**
  Type: Map[string][any]
  _Optional_

  FrontMatter is a set of rules for a `nodesSelector` to **include** nodes with
  compliant front-matter. The compliance is positive if a node matches one or 
  more rules. Applies to document nodes only.

  If a node is evaluated to be compliant with both `FrontMatter` and 
  `ExcludeFrontMatter` rules, it will be excluded. 
  
  Markdown metadata is commonly provisioned as `front-matter` block at the head
  of the document delimited by comment tags (`---`). The supported format of the 
  metadata is YAML. 

  The `FrontMatter` rules are mappings between path patterns identifying an 
  element in the front-matter and a value. If the path matches an actual path
  to an element in the front-matter and the value of this element matches the 
  rule value, there is a positive match.
  
  The path patterns are a very simplified form of JSONPath notation.
  An object in path is modeled as dot (`.`). Paths start with the root object, 
  i.e. the most minimal path is `.`.
  An object element value is referenced by its name (key) in the object map: 
  `.a.b.c` is path to element `c` in map `b` in map `a` in root object map.
  Element values can be scalar, object maps or arrays.
  An element in an array is referenced by its index: `.a.b[1]` references `b` 
  array element with index 1.
  Paths can include up to one wildcard `**` symbol that models *any* path node.
  A `.a.**.c` models any path starting with  `.a.` and ending with `.c`.

- **ExcludeFrontMatter**
  Type: Map[string][any]
  _Optional_

  ExcludeFrontMatter is a set of rules for a `nodesSelector` to **exclude** nodes 
  with compliant front-matter. The compliance is positive if a node matches one or
  more rules. Applies to document nodes only.

  If a node is evaluated to be compliant with both `FrontMatter` and 
  `ExcludeFrontMatter` rules, it will be excluded. 
  
  Markdown metadata is commonly provisioned as `front-matter` block at the head
  of the document delimited by comment tags (`---`). The supported format of the 
  metadata is YAML. 

  The `ExcludeFrontMatter` rules are mappings between path patterns identifying an 
  element in the front-matter and a value. If the path matches an actual path
  to an element in the front-matter and the value of this element matches the 
  rule value, there is a positive match.
  
  The path patterns are a very simplified form of JSONPath notation.
  An object in path is modeled as dot (`.`). Paths start with the root object, 
  i.e. the most minimal path is `.`.
  An object element value is referenced by its name (key) in the object map: 
  `.a.b.c` is path to element `c` in map `b` in map `a` in root object map.
  Element values can be scalar, object maps or arrays.
  An element in an array is referenced by its index: `.a.b[1]` references `b` 
  array element with index 1.
  Paths can include up to one wildcard `**` symbol that models *any* path node.
  A `.a.**.c` models any path starting with  `.a.` and ending with `.c`.
  
## ContentSelector

Type: Object

ContentSelector specifies a content assignment that can be optionally filtered 
by a selection criteria.

Note: Selection criteria is not implemented yet

**Properties**:

- **Source**  
  Type: [string](https://golang.org/ref/spec#String_types)  
  _Mandatory_

  URI of a document.

## Template

Type: Object

Template specifies rules for selecting content assigned to variables that are
applied to a template at path.

**Properties**:

- **Path**  
  Type: [string](https://golang.org/ref/spec#String_types)  
  _Mandatory_

  Path to the template file.
  A template file content is valid Golang template content. See 
  https://golang.org/pkg/text/template.
  The template will have at disposal the variables defined in Sources.

- **Sources**  
  Type: Map[string][ContentSelector](#contentSelector)]  
  _Mandatory_

  Sources maps variable names to [ContentSelector](#contentSelector) rules that 
  filter and assign content from source location.

## Links

Type: Object  
At least one of Rewrites or Downloads has to be defined.

Links defines rules how document links are processed.

**Properties**:

- **Rewrites**  
  Type: Map[string][LinkRewriteRule](#linkrewriterule)]  
  _Optional_

  Rewrites maps regular expressions matching document links resolved to absolute
  with link rewriting rules ([LinkRewriteRule](#linkrewriterule)).   
  A common use is to rewrite resource links versions, if they support that, in 
  order to have them downloaded in a particular version.
  A rewrite that maps an expression to nil rules (`~`) is interpreted as request 
  to remove the links matching the expression.

- **Downloads**  
  Type: [Download ](#downloads)
  _Optional_

  Downloads defines rules which resources linked from downloaded documents need
  to be downloaded and optionally renamed. Downloads are performed after rewrites.
  Links to downloaded resources are rewritten as appropriate to match the new 
  downloaded resource relative position to the referencing documents.

## LinkRewriteRule

Type: Object

LinkRewriteRule specifies link components to be rewritten.

**Properties**:

- **Version**  
  Type: [string](https://golang.org/ref/spec#String_types)  
  _Optional_

  Rewrites the version of links matching this pattern, e.g. `master` -> `v1.11.3`.
  For GitHub links the version will rewrite the SHA path segment in the URL
  right after organization, repository and resource type.   
  Note that not every link supports version. For example GitHub issues links 
  have different pattern and it has no sha segment. The version will be applied 
  only where and however applicable.

- **Destination**  
  Type: [string](https://golang.org/ref/spec#String_types)  
  _Optional_

  Rewrites the destination in a link|image markdown. An explicitly defined empty
  string (`""`) indicates that the link markdown will be removed, leaving only
  the text component behind for links and nothing for images.
  
  This setting has precedence over version if both are specified.

  A link destination rewritten by this rule, which is also matched by a 
  downloads specification, will be converted to  relative, using the result 
  of this destination substitution. The final result therefore may be different 
  from the destination substitution defined here.

- **Text**  
  Type: [string](https://golang.org/ref/spec#String_types)  
  _Optional_

  Rewrites or sets a matched link markdown text component (alt-text for images)
  If used in combination with destination: `""` and value `""` this will 
  effectively remove a link completely, leaving nothing behind in the document.

- **Title**  
  Type: [string](https://golang.org/ref/spec#String_types)  
  _Optional_

  Rewrites or sets a matched link markdown's title component.
  Note that this will have no effect with settings destination: `""` and text: 
  `""` as the whole markdown together with tis title will be removed.

## Downloads

Type: Object

Downloads is a definition of the scope of downloadable resources and rules for 
renaming them.

**Properties**:

- **Renames**  
  Type: [ResourceRenameRules](#resourcerenamerules)  
  _Optional_

  Renames is a set of renaming rules that are globally applicable to all 
  downloads regardless of scope.

- **Scope**  
  Type: Map[string][resourcerenamerules](#resourcerenamerules)  
  _Optional_

  Scope defines a concrete scope for downloaded resources with a set of mappings 
  between document links matching regular expressions and (optional) naming 
  patterns. A scope map entry maps a regular expression tested to match document 
  links that will be downloaded to an optional rename specification or `~` for 
  default. If no particular rename specification is supplied:

  1.  the globally supplied renames are tested to match and applied (if supplied)
  2.  a default rename expression `$uuid.$ext` will be applied to all matched 
      targets.

  Example: define a download scope that downloads every matching document.

  ```
  scope:
    gardener/gardener/(tree|blob|raw)/master/docs: ~
  ```

  Example: define a download scope that downloads every matching document and
  renames it to a specific pattern if it is an jpg|gif|png image or uses the 
  default naming pattern otherwise.

  ```
  scope:
    gardener/gardener/(tree|blob|raw)/master/docs:
      "\\.(jpg|gif|png)": "$name-image-$uuid.$ext"
  ```

## ResourceRenameRules

Type: Map[string][string]

ResourceRenameRules defines a mapping between regular expressions matching
resource locators and name pattern expressions or exact names.
The name pattern will be used to rename the downloaded resources matching the
specified regular expression key.
There is a set of variables that can be used to construct the naming 
expressions:

- `$name`: the original name of the resource
- `$uuid`: a UUID generated for the resource
- `$ext`: a original resource extension
  The default expression applying to all resources is: `$uuid$ext`
  Example:

```
 "\\.(jpg|gif|png)": "$name-image-$uuid$ext"
```

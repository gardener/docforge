# Documentation Manifests Reference

This is a reference documentation for the elements in a docforge manifest.

Table of Contents

- [Documentation Manifests Reference](#documentation-manifests-reference)
  - [Documentation](#documentation)
  - [Node](#node)
  - [NodeSelector](#nodeselector)

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

  Declare rules for dynamic resolution of nodes into a Structure. Can be defined
  together with Structure to combine implicit and explicit definition of structure.

## Node

**Type**: Object

A Node is a node in a documentation structure.   
A Node with a `Nodes` property is a *container node*. Container nodes are 
recursive. They can contain other nodes in their `Nodes` property, which in turn 
can contain other nodes and form a tree hierarchy in this way. On a file system 
it is serialized as directory.   
A Node that defines either of the content assignment properties - `source` or
`multiSource` is a *document node*. The two properties are alternatives. Only
one can be used in a node. On a file system a document node is serialized as file.

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
  *Mandatory* if this is a *document node* and MultiSource is not specified.  
  *Alternative* to MultiSource.  
  Applicable to document nodes only.

  Source declares a content assignment to this node from a single location.

- **MultiSource**  
  Type: Array of [string](https://golang.org/ref/spec#String_types)  
  *Mandatory* if this is a *document node* and Source is not specified.  
  *Alternative* to Source.

  The contents provided in the MultiSource list is aggregated into a single
  document in the order in which they are declared.   
  Applicable to document nodes only.

- **Nodes**  
  Type: Array of [Node](#node)  
  *Mandatory* for container nodes  

  The Nodes' property is a list of nodes that are descendants of this Node in the
  documentation structure. Applicable to container nodes only.

- **NodeSelector**  
  Type: [NodeSelector](#nodeselector)  
  *Optional*  
  Applicable to container nodes only.

  NodesSelector specifies a [NodeSelector](#nodeselector) to be used for dynamic 
  resolution of nodes into descendants of this container node. The modelled 
  structure is merged into this node's *Nodes* field, mashing it up with 
  potentially explicitly defined descendants there. The merge strategy 
  identifies identical nodes by their name and when there is a match, it performs 
  a deep merge of their properties. When there are merger conflicts, the 
  explicitly defined node wins.   
  Depending on the goal, a NodeSelector can coexist, or be an alternative to an 
  explicitly defined structure.

- **Properties**  
  Type: Map[string][any]  
  *Optional*

  Properties are a map of arbitrary, key-value pairs to model custom, untyped 
  node properties. The requirements and constraints on the properties depends 
  on the feature that makes use of them.   
  For example, specifying a "frontmatter" 
  property on a node will result in applying the value as front matter in the 
  resulting document content. When Hugo processors are applied this can be 
  applied not only on document, but also on container nodes.

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
  reconciled with the including Documentation manifest.

  When Path references a supported resource container (GitHub or file system at 
  the moment), the structure of the resource inside will be used to generate a 
  node structure. For GitHub path that is a folder in a GitHub repo, the 
  generated nodes' hierarchy corresponds ot the file/folder structure at that 
  path.

  Without any further criteria, all nodes within path are included, but 
  optionally nodes can be excluded e.g. by defining constraints on accepted paths 
  or the depth of the hierarchy.

- **ExcludePaths**  
  Type: Array of string
  _Optional_

  ExcludePath is a set of exclusion rules applied to node candidates based on the 
  nodes' path. Each rule is a regular expression tested to match on each node's 
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

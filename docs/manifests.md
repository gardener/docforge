# Documentation Manifests

Manifests are configuration files that describe the structure of a documentation bundle and the rules how to construct it. This document specifies the options for designing a manifest.

Table of Contents
- [Documentation Manifests](#documentation-manifests)
  - [Basic Concepts](#basic-concepts)
    - [Structure](#structure)
    - [Nodes](#nodes)
  - [Structuring content](#structuring-content)
    - [Explicit structuring with `nodes`](#explicit-structuring-with-nodes)
    - [Implicit structuring with `nodesSelector`](#implicit-structuring-with-nodesselector)
    - [Combining `nodesSelector` and `nodes`](#combining-nodesselector-and-nodes)
  - [Content assignment](#content-assignment)
    - [Single source](#single-source)
    - [Aggregating content](#aggregating-content)
  - [Advanced node selection](#advanced-node-selection)
  - [References](#references)

## Basic Concepts

### Structure

The `structure` element in a manifest represents the intended documentation bundle and describes where to get the material, how to transform it and and how to position it. This element is an (ordered) list type, where its list items are this structure's top-level documentation structure nodes. You can specify between 0 (which makes sense only if node selector is defined) and however you need structure top-level nodes:

Physically, the structure element corresponds to the location you specify with the `--destination` (or `-d`) flag when you run the main `docforge` command.

### Nodes

There are two types of structure nodes - *document* nodes and *container* nodes. The container nodes are recursive structures useful for organizing content hierarchically. Document nodes represent the actual documents in the intended documentation bundle.

## Structuring content

### Explicit structuring with `nodes`

Container nodes can specify other nodes that they *contain*, using the `nodes` element. It is a list structure in which each list item can be either a *container* node or a *document* node.

In this example, a structure is defined to have a list of two top-level nodes - one document node with name `overview` and one container node with name `concepts`. The `concepts` node explicitly defines that it contains another document node with source referencing the apiserver.md document. 
```yaml
structure:
- name: overview
  source: https://github.com/gardener/gardener/blob/master/docs/README.md
- name: concepts
  nodes:
  - source: https://github.com/gardener/gardener/blob/master/docs/concepts/apiserver.md
```
Considering that the documentation bundle is forged destination flag defining path `docforge-docs`, the generated file/folder structure in that folder will look lie this:
```
docforge-docs
├── concepts
│   └── apiserver.md
└── overview.md
```
You will notice that the `structure` element itself can be seen as a `nodes` property of the manifest, since it adheres to the same model.

In the example above the name of the apiserver node was not explicitly specified, but it could be named anything explicitly. The node names are the corresponding file/folder names.

### Implicit structuring with `nodesSelector`

The `nodesSelector` element is a set of rules defining a structure. At runtime, they are resolved by docforge to actual nodes and the structure that descends from them and this result is merged into the `nodes` of the node where the `nodeSelector` is defined.

In fact, the whole manifest structure can be defined with a node selector too. One application of this option would be to replicate one whole resource set from one location to another, rewriting the linked resources in the process. A manifest could be as minimal as this example:

```yaml
nodesSelector:
  path: https://github.com/gardener/gardener/tree/master/docs
```
A more conventional use of `nodeSelector` would be to include certain structures and mash them up with others, defined implicitly or explicitly.

In the following example, the manifests declares three top-level nodes - one document node and two container nodes. The structure under the container nodes is not explicitly specified. Instead, nodeSelectors are used to have it resolved at runtime dynamically.
```yaml
structure:
  - source: https://github.com/gardener/gardener/blob/master/docs/README.md
  - name: concepts
    nodesSelector:
      path: https://github.com/gardener/gardener/tree/master/docs/concepts
  - name: extensions
    nodesSelector:
      path: https://github.com/gardener/gardener/tree/master/docs/extensions
```
Using this manifest with docforge will end up in the following file/folder structure (some lines omitted with `...` for brevity):
```
docforge-docs
├── concepts
│   ├── admission-controller.md
│   ├── apiserver.md
...
│   ├── scheduler.md
│   └── seed-admission-controller.md
├── extensions
│   ├── backupbucket.md
│   ├── backupentry.md
│   ├── cluster.md
...
│   └── worker.md
└── README.md
```
If the file folder structure at any of the node selectors path changes and we run the tool again, we will get a different and up-to-date result.

This `nodesSelector` element is useful to include whole existing structures dynamically. It also allows you to be more selective in what you include which is explored further below in this guide. 

### Combining `nodesSelector` and `nodes`

The `nodesSelector` and `nodes` elements are not alternatives and in fact can be used together for various use cases. That would be useful to add material to existing structures. 
```yaml
structure:
  - name: concepts
    nodesSelector:
      path: https://github.com/gardener/gardener/tree/master/docs/concepts
    nodes:
    - source: https://github.com/gardener/gardener/blob/master/docs/README.md
```
Another useful application would be a fine-grained control on the properties of the nodes resolved by a `nodeSelector`. In this particular case the explicitly defined nodes that we want to control need to match by name and position their targets in the resolved structure.

## Content assignment
Document nodes require content assignment, which will be used when serializing them into files. There are multiple options to assign content, each explored in this section.

### Single source
The simplest content assignment is with the `source` property. When a document node represents a single whole document this is the way to go. In this case specifying a name of the node is optional too. Unless it is necessary to override the original referenced resource name, the name property can be omitted.

```yaml
structure:
  - source: https://github.com/gardener/gardener/blob/master/docs/README.md
```
To override the original name of the resource, specify node name explicitly.
```yaml
structure:
  - name: index.md
    source: https://github.com/gardener/gardener/blob/master/docs/README.md
```

### Aggregating content
A document node content can be constructed of multiple sources too. Using the `multiSource` property it is possible to define where to get the content from, and in what order it must be aggregated.

In the following example the file for the document nodes `overview` will be created with content from each of the sources specified in `multiSource` property in that order
```yaml
structure:
  - name: overview
    multiSource:
    - https://github.com/gardener/gardener/blob/master/docs/README.md
    - https://github.com/gardener/gardener/blob/master/concepts/README.md
```

## Advanced node selection
You can be far more selective with nodeSelector han picking up a path to resolve a structure from.
- use `excludePaths` to exclude branches of the hierarchy. Each entry in the list is a regular expression and the resources that it matches are excluded from the resolved structure. 
- use `depth` to define maximum depth for the resolved structures. Resources that go further down are not included. Use for example to pull only the top-level nodes of a structure.

## References
- [Manifest Reference](manifest-ref.md)
- [Manifest examples](../example) in the project GitHub repository
# Documentation Manifests

Manifests are configuration files that describe the structure of a documentation bundle and the rules how to construct it. This document specifies the options for designing a manifest.

Table of Contents
- [Documentation Manifests](#documentation-manifests)
  - [Basic Concepts](#basic-concepts)
    - [Structure](#structure)
    - [Nodes](#nodes)
    - [Links](#links)
  - [Structuring content](#structuring-content)
    - [Explicit structuring with `nodes`](#explicit-structuring-with-nodes)
    - [Implicit structuring with `nodesSelector`](#implicit-structuring-with-nodesselector)
    - [Combining `nodesSelector` and `nodes`](#combining-nodesselector-and-nodes)
  - [Content assignment](#content-assignment)
    - [Single source](#single-source)
    - [Selecting and aggregating content](#selecting-and-aggregating-content)
    - [Selecting and laying out content in template](#selecting-and-laying-out-content-in-template)
  - [Downloads scope](#downloads-scope)
  - [Rewriting links](#rewriting-links)
  - [Advanced node selection](#advanced-node-selection)
  - [References](#references)

## Basic Concepts

### Structure

The `structure` element in a manifest represents the intended documentation bundle and describes where to get the material, how to transform it and and how to position it. This element is an (ordered) list type, where its list items are this structure's top-level documentation structure nodes. You can specify between 0 (which hardly makes sense) and however you need structure top-level nodes:

Physically, the structure element corresponds to the location you specify with the `--destination` (or `-d`) flag when you run the main `docforge` command.

### Nodes

There are two types of structure nodes - *document* nodes and *container* nodes. The container nodes are recursive structures useful for organizing content hierarchically. Document nodes represent the actual documents in the intended documentation bundle.

### Links

Simply copying (partial) content sets from one location to another without addressing the cross linking inconsistencies that inevitably occur is insufficient for the automation that docforge commits to. It performs an operation called *links reconciliation* that aligns the content links with their new home and the designer's intentions for the documentation structure.

The links element is the configuration instructing docforge how to address links in the document nodes content and the resources they reference. Based on the rules it defines, docforge may rewrite links for example to reference a specific version of resource or download referenced resources and rewrite links to their new home among many other options. Links can be defined globally and on nodes. A cascading override from global to local is implemented to minimize redundancies.

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
```sh
docforge-docs
├── concepts
│   └── apiserver.md
└── overview.md
```
You will notice that the `structure` element itself can be seen as a `nodes` property of the manifest, since it adheres to the same model.

In the example above the name of the apiserver node was not explicitly specified but it could be named anything explicitly. The node names are the corresponding file/folder names.

### Implicit structuring with `nodesSelector`

The `nodesSelector` element is a set of rules defining a structure. At runtime they are resolved by docforge to actual nodes and the structure that descends from them and this result is merged into the `nodes` of the node where the `nodeSelector` is defined. 

In fact, the whole manifest structure can be defined with a node selector too. One application of this option would be to replicate one whole resource set from one location to another, optionally rewriting the linked resources versions in the process. A manifest could be as minimal as this example:

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
To override the original name of the resource, specify node name explicitly. You may add `.md` suffix or, if omitted, it will be added automatically for you when the document node is serialized as file.
```yaml
structure:
  - name: index
    source: https://github.com/gardener/gardener/blob/master/docs/README.md
```

### Selecting and aggregating content
A document node content can be constructed of multiple sources too. Using the `contentSelectors` property it is possible to define rules where to get the content from, how to select it from those sources (not implemented yet) and in what order it must be aggregated.

In the following example the file for the document nodes `overview` will be created with content from each of the `source`s specified in its `contentSelectors` property in that order
```yaml
structure:
  - name: overview
    contentSelectors:
    - source: https://github.com/gardener/gardener/blob/master/docs/README.md
    - source: https://github.com/gardener/gardener/blob/master/concepts/README.md
```

### Selecting and laying out content in template
The `template` element is useful when content from multiple sources needs to be laid out in more sophisticated manner than a sequential aggregation as with `contentSelectors`. It uses the same model as `contentSelectors` to select content from multiple sources, but then the content data is assigned to variables, which are provided to the template available at `path` when the template is resolved.

```yaml
structure:
  - name: overview
    template:
      path: https://github.com/gardener/gardener/blob/master/tempaltes/00.md
      "overview":
        - source: https://github.com/gardener/gardener/blob/master/docs/README.md
      "concepts":
        - source: https://github.com/gardener/gardener/blob/master/concepts/README.md
```

Considering the example above, a template might look like this:

```md
# Documentation example

{{ .overview }}

--------------------

{{ .concepts }}

Find us on [twitter](whatever)
```
Note, that it is essential to reference the variables you defined starting with dot (`.`) and use exactly the name of the variable defined in the manifest.

A single template may be reused multiple times for different nodes.

## Downloads scope
When forging a documentation bundle, docforge will rewrite links for you to keep the consistent both with your intent for the bundle, and the potential changes 
in the relative location between cross-linked resources. Links to locally available resources will be rewritten to relative and other links will be rewritten to
absolute. Defining what's local and what is not therefore has impact not only on downloads, but also on related crosslinks.

In docforge, all document nodes are considered local and the content they reference will be downloaded and serialized regardless of any other settings. You can specify also referenced resources that you want downloaded.
A typical example are images referenced by documents.

Here is an example definition of a download scope.
```yaml
links:
  downloads:
    scope:
      gardener/gardener/(blob|raw)/v1.11.1/docs: ~
```
The scope entry in this example is a mapping between a regular expression matching the absolute form of the links  in the document nodes content in the bundle and the default rule for the downloaded resource names. the regular expression will match all referenced resources form the `gardener` organization and repository that are of type `blob` and `raw`. To download only raster images, the following expression might be of use: `"\\.(jpg|gif|png)"`. Or a combination of the two to have only images from the gardener repository downloaded. you can specify as many scope entries as you need.

The rule mapped to the regular expression of this example is nil (`~`), which defaults to a naming pattern that changes the name to a unique UUID string and preserves the extension. That could be changed by providing a naming pattern using the `$name` (original resource name), `$uuid` (generated UUID) and `$ext` (original resource extension) variables.

Note that links can be defined both globally and on a node with a cascading override behavior where the more local takes precedence. Use this advantage to define global download settings and node-specific ones only where necessary.

## Rewriting links

The link rewrite feature in docforge is a powerful tool that lets you manipulate links in the downloaded documents for various purposes.

- **Rewriting GitHub links versions**    
  Normally, cross-references in documents created on or for GitHub would use the `master` SAH alias for "tip of the history for that resource". More often than not documentation bundles are created for a particular "snapshot" of the state of a set of files sometimes referred to as Bill of Material. In this case, you don't want your cross reference to point ot the `master` of a document from another related component. You want it rather to point to the version of that component form the same Bill of Material. Or in other words you want the cross references to reflect the "snapshot" of the state of components that are bundled together and not "latest". A typical use case is when a system of components is delivered in staged mode. Each stage would have a different version of the component set (or versions if each component has its own versioning). On a landscape that lags behind due to the staged delivery, you will want not the latest documentation but a prior version of it. 
- **Changing link elements or completely removing links**   
  Some links may not e desirable on a pulled document content. There are different strategies how to address that supported by docforge. Links can be completely removed, only the markdown for the links can be removed but their text can e left behind, and/or their text can be changed, the title element can be added/changed/removed.

The example below configures all cross-references to github.com/gardener/gardener resources to be rewritten for version v1.10.0. 
```
links:
  rewrites:
    github.com/gardener/gardener:
      version: v1.10.0
```
> Note that rewriting links is performed by docforge **before** downloads so setting a version will define the version of a matching downloaded resource too.

The `~` symbol mapped to a rewrite pattern is a "remove matched links" instruction to docforge. The example below removes all links that match `/gardener/garden-setup`:
```
links:
  rewrites:
    /gardener/garden-setup: ~
```

The example below removes a link markdown, but leaves its text behind in the document content:
```
links:
  rewrites:
    /gardener/garden-setup: 
      destination: ~
```

Use `text`, `destination` and `title` to change these link markdown components. Such specific instructions normally correspond to very precise link matching patterns and are normally found on node level.
```
links:
  rewrites:
    "https://kubernetes.io/docs/concepts/extend-kubernetes/operator/":
      destination: "https://kubernetes.io/docs/concepts/extend-kubernetes/operator-new"
      text: smooth operator
      title: ust a sample

```

## Advanced node selection
You can be far more selective with nodeSelector han picking up a path to resolve a structure from.
- use `excludePaths` to exclude branches of the hierarchy. Each entry in the list is a regular expression and the resources that it matches are excluded from the resolved structure. 
- use `depth` to define maximum depth for the resolved structures. Resources that go further down are not included. Use for example to pull only the top-level nodes of a structure.


## References
- [Manifest Reference](manifest-ref.md)
- [Manifest examples](../example) in the project GitHub repository
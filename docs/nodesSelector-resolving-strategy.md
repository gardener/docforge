# Understand NodeSelectors

This document describes how nodesSelectors are resolved/transformed to nodes.

## Importing another Documentation structures.

1. Example I

Starting with the following example, where we have a simple documentation_structure.yaml that defines a nodesSelector on the root level (meaning not being part of the `strucutre`),

```yaml
# documentation_structure.yaml
nodesSelector:
    path: https://github.com/gardener/documentation/blob/master/moduleB.yaml
structure:
    - name: api.md
      source: https://github.com/gardener/api/blob/master/docs/api_v1-12-3.md
    - name: references.md
      source: https://github.com/gardener/api/blob/master/docs/references.md
```

When the nodesSelector points to another documentation strcture like `moduleB` that looks like this:

```yaml
# moduleB.yaml
structure:
    - name: api.md
      source: https://github.com/gardener/gardener/blob/master/docs/notAboveApi.md
    - name: references.md
      source: https://github.com/gardener/api/blob/master/docs/references.md
```

executing the command:

    docforge -f documentation_structure.yaml -d docs

will result to a file system structure:

```
./docs
+-- api.md
+-- references.md
+-- notAboveApi.md
```

A couple of remarks on the previous example. The name used for the file is defined by the `name`
property, but in this case the nodesSelector resolves a structure which also has a node that uses
the name `api.md`, that way if the nodes are not exactly equal to each other the one that is being
imported will:

  1. use the name of the file specified by the source property
  2. add the name of the module
  3. will be skipped

1. Example II

## Importing a GitHub tree structure or File System directory


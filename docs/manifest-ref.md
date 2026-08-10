# Manifest Reference

A docforge manifest is a YAML file that declares the documentation structure to build.
Every manifest is a node tree. The top-level keys are nodes listed under `structure`.

> All YAML keys and default values in this document are verified against
> `pkg/manifest/manifest_types.go`, `pkg/manifest/node.go`, and `pkg/manifest/manifest.go`.

## Top-level structure

```yaml
structure:
  - <node>
  - <node>
```

`structure` is the only top-level key. It contains a list of nodes.
<!-- verified: pkg/manifest/manifest_types.go DirType.Structure yaml:"structure" -->

---

## Node types

A node must have exactly one of the four type-determining keys: `file`, `dir`, `fileTree`, or `manifest`.
Combining more than one is an error.
<!-- verified: pkg/manifest/manifest.go decideNodeType() — returns error if len(candidateType) != 1 -->

---

### `file` — single document node

Includes a single file in the output bundle.

```yaml
- file: https://github.com/org/repo/blob/main/docs/guide.md
```

```yaml
- file: guide.md
  source: https://github.com/org/repo/blob/main/docs/guide.md
```

```yaml
- file: combined.md
  multiSource:
    - https://github.com/org/repo/blob/main/docs/part1.md
    - https://github.com/org/repo/blob/main/docs/part2.md
```

| Key | Type | Description |
|---|---|---|
| `file` | string | Output filename. If `source` is omitted, this must be a full resource URL and the filename is derived from it. |
| `source` | string | Source URL. If set, `file` is the output name. Mutually exclusive with `multiSource`. |
| `multiSource` | list of strings | Multiple source URLs whose content is concatenated (in order) into a single output file. Mutually exclusive with `source`. |

<!-- verified: pkg/manifest/manifest_types.go FileType — file yaml:"file", source yaml:"source", multiSource yaml:"multiSource" -->

**Shorthand:** if `file` contains a `/`, the last path segment becomes the filename and the full value is used as the source URL.
<!-- verified: pkg/manifest/manifest.go resolveManifestLinks() — strings.Contains(node.File, "/") → node.Source = node.File; node.File = path.Base(node.File) -->

**Empty section index:** `file: _index.md` with no `source` and no `multiSource` is valid — it produces a file containing only the declared `frontmatter`.
<!-- verified: pkg/manifest/manifest.go resolveManifestLinks() — node.File == "_index.md" && node.Source == "" → return nil -->

---

### `dir` — container node

Creates a directory in the output. Contains child nodes under `structure`.

```yaml
- dir: guides
  structure:
    - file: https://github.com/org/repo/blob/main/docs/guide.md
```

| Key | Type | Description |
|---|---|---|
| `dir` | string | Output directory name. |
| `structure` | list of nodes | Child nodes. |

<!-- verified: pkg/manifest/manifest_types.go DirType — dir yaml:"dir", structure yaml:"structure" -->

---

### `fileTree` — directory tree

Recursively includes all files from a GitHub directory tree.

```yaml
- fileTree: https://github.com/org/repo/tree/main/docs
  excludeFiles:
    - cmd-ref/docforge.md
    - internal/
```

| Key | Type | Description |
|---|---|---|
| `fileTree` | string | URL of a GitHub directory (using `/tree/` path). |
| `excludeFiles` | list of strings | Path prefixes (relative to the `fileTree` root) to exclude. Matching uses `strings.HasPrefix`. |

<!-- verified: pkg/manifest/manifest_types.go FilesTreeType — fileTree yaml:"fileTree", excludeFiles yaml:"excludeFiles" -->
<!-- verified: pkg/manifest/manifest.go constructNodeTree() — strings.HasPrefix(file, excludeFile) -->

---

### `manifest` — include another manifest

Recursively includes another manifest file, merging its `structure` into the current tree.

```yaml
- manifest: https://github.com/org/repo/blob/main/docs/sub-manifest.yaml
```

```yaml
- manifest: ../shared/base.yaml
```

Relative paths are resolved from the location of the including manifest.
<!-- verified: pkg/manifest/manifest.go readManifestContents() — repositoryhost.IsRelative check + r.ResolveRelativeLink -->

Duplicate `dir` nodes from included manifests are merged. When the same directory name appears more than once, their `structure` lists are concatenated. If both have `frontmatter`, docforge returns an error.
<!-- verified: pkg/manifest/manifest.go mergeFolders() -->

---

## Common fields (all node types)

### `frontmatter`

Arbitrary key-value map written as YAML front matter into the output file.

```yaml
- dir: guides
  frontmatter:
    weight: 10
    title: Guides
  structure:
    - file: https://github.com/org/repo/blob/main/docs/guide.md
      frontmatter:
        title: Getting Started
        weight: 1
```

A parent node's `frontmatter` is inherited by all descendants. Child values override parent values on collision. The `aliases` key is never propagated from parent to child.
<!-- verified: pkg/manifestplugins/markdown/plugin.go propagateFrontmatter() -->

**Ordering by `weight`:** if any child node has `frontmatter.weight` (integer or float), children are sorted ascending by weight. Weighted children sort before unweighted ones. Nodes with equal weight preserve manifest order (stable sort). A warning is logged when some siblings have `weight` and others do not.
<!-- verified: pkg/manifest/order.go resolveOrder() and weightOf() -->

---

### `linkResolution`

Map of source URL → destination node path, overriding docforge's automatic link rewriting for specific links within this node's document.

```yaml
- file: https://github.com/org/repo/blob/main/docs/guide.md
  linkResolution:
    https://github.com/org/repo/blob/main/docs/other.md: other/other.md
```

<!-- verified: pkg/manifest/node.go LinkResolution yaml:"linkResolution" -->
<!-- verified: pkg/nodeplugins/markdown/linkresolver/link_resolving.go — node.LinkResolution[destinationResourceURL] -->

---

## Relative links in manifests

If a path starts with `/` it is resolved from the repository root. Otherwise it is resolved relative to the manifest file's own location.

```yaml
structure:
  # resolves relative to this manifest's directory
  - file: ../README.md
  # resolves from repo root
  - fileTree: /docs
```

<!-- verified: pkg/manifest/manifest.go resolveManifestLinks() — repositoryhost.IsRelative check + r.ResolveRelativeLink(manifest.Manifest, link) -->

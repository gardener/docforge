# Plugin architecture

Docforge processes a documentation bundle in two distinct phases. Each phase has its own plugin type.

```
manifest YAML
      │
      ▼
┌─────────────────────────────────┐
│  Phase 1 — manifest resolution  │  ← manifest plugins run here
│  (build the node tree)          │
└─────────────────────────────────┘
      │  list of *Node objects
      ▼
┌─────────────────────────────────┐
│  Phase 2 — node processing      │  ← node plugins run here
│  (fetch content, write files)   │
└─────────────────────────────────┘
      │
      ▼
   output files on disk
```

---

## Phase 1 — Manifest plugins

**When:** after the manifest YAML is parsed into a tree of `*Node` objects, before any file is fetched.

**What they can do:** walk every node and modify it in place — add or change frontmatter fields, set the `Processor` field, or remove nodes from the tree entirely.

**Interface:** `PluginNodeTransformations() []manifest.NodeTransformation`

Each transformation is a function `func(node, parent *Node, registry) (bool, error)` called once per node, depth-first.

### manifest/alias

**Activated by:** `--aliases-enabled`

Propagates Hugo `aliases` from a `dir` node down to its children. For each alias on the parent, it appends a derived path to the child's `aliases` frontmatter list.

Example: a `dir` with alias `/archive/blogs` and a child `post.md` → child gets alias `/archive/blogs/post`.

**Does not touch** nodes that already have their own `aliases` — child values are kept and the parent-derived ones are appended alongside them.

### manifest/markdown

**Activated by:** `markdown-enabled: true` in `~/.docforge/config` (config-file-only setting, no CLI flag)

Runs two transformations:

1. **`setMarkdownProcessor`** — for every `file` node whose filename ends in `.md`, sets `node.Processor = "markdown"`. This tells Phase 2 to use the markdown node plugin instead of the plain downloader.

2. **`propagateFrontmatter`** — copies frontmatter fields from a parent `dir` node into each child node. If both parent and child define the same key, the child's value wins. The `aliases` key is never propagated (that is handled by the alias plugin separately).

### manifest/docsy

**Activated by:** `--docsy-edit-this-page-enabled`

Adds Docsy "Edit this page" frontmatter fields to every `.md` file node:

- `github_repo` — the repository URL
- `github_subdir` — the directory within the repo
- `path_base_for_github_subdir` — maps the output path back to the source filename
- `params.github_branch` — the branch or tag ref

Skips index files without a source, multi-source nodes, and non-markdown nodes.

### manifest/filetypefilter

**Activated by:** `--content-files-formats .md,.html` (only when the flag is non-empty)

Removes from the node tree any `file` node whose source URL does not end with one of the allowed extensions. The node is set to `nil` in the parent's `Structure` slice, so it is never fetched or written.

When the flag is empty (the default), this plugin is not registered at all and all file types pass through.

---

## Phase 2 — Node plugins

**When:** after Phase 1 completes and the final node list is known. Each node is dispatched to the plugin whose `Processor()` name matches `node.Processor`.

**What they do:** fetch the file content from GitHub, transform it if needed, and write it to the destination directory.

**Interface:** `Processor() string` and `Process(*Node) error`

The node's `Processor` field is set during Phase 1:
- `manifest/markdown` sets it to `"markdown"` for `.md` files (when `markdown-enabled` is true)
- `setDefaultProcessor` in `manifest.go` sets it to `"downloader"` for any `file` node that still has no processor assigned

### node/downloader

**Processor name:** `"downloader"`

**Used for:** all non-markdown files (images, PDFs, HTML, etc.), and markdown files when `markdown-enabled` is false.

Schedules the file for download: fetches the raw bytes from `node.Source` and writes them to `node.NodePath()` in the destination directory. No content transformation.

### node/markdown

**Processor name:** `"markdown"`

**Used for:** `.md` files when `markdown-enabled: true`.

Does everything the downloader does, plus:

1. **Fetches the raw Markdown** from GitHub.
2. **Merges frontmatter** — combines the node's manifest-declared `frontmatter` with any frontmatter already present in the source file. Manifest values take precedence on collision.
3. **Resolves links** — rewrites relative Markdown links so they point to the correct paths within the output bundle. Cross-repository links that cannot be resolved are logged as warnings.
4. **Hugo processing** (when `--hugo=true`) — renames section index files to `_index.md`, rewrites `.md` links to pretty-URL format.
5. **GitHub info** (when `--github-info-destination` is set) — fetches commit metadata (author, contributors, lastmod, publishdate, SHA) and writes a `.json` sidecar file.

---

## Full flow with all plugins active

```
docforge -f manifest.yaml -d /out \
  --github-oauth-env-map github.com=TOKEN \
  --aliases-enabled \
  --docsy-edit-this-page-enabled \
  --content-files-formats .md \
  --github-info-destination __github-info
# markdown-enabled: true in ~/.docforge/config
```

```
Parse manifest YAML → *Node tree
          │
          ▼
[manifest/markdown]     setMarkdownProcessor  → node.Processor = "markdown" for *.md
                        propagateFrontmatter  → parent dir frontmatter merged into children
          │
          ▼
[manifest/alias]        calculateAliases      → child aliases derived from parent dir aliases
          │
          ▼
[manifest/docsy]        editThisPage          → github_repo / github_subdir added to frontmatter
          │
          ▼
[manifest/filetypefilter] checkFileTypeFormats → non-.md nodes removed from tree
          │
          ▼
[setDefaultProcessor]   any remaining file node without Processor → "downloader"
          │
          ▼ (Phase 2 — parallel workers)
          │
          ├─ node.Processor == "markdown"   → [node/markdown]
          │      fetch raw .md from GitHub
          │      merge frontmatter (manifest values override source file values)
          │      resolve + rewrite links
          │      Hugo processing (_index.md rename, pretty URLs)
          │      write .md to /out
          │      write .json sidecar to /out/__github-info/
          │
          └─ node.Processor == "downloader" → [node/downloader]
                 fetch raw bytes from GitHub
                 write as-is to /out
```

<!-- verified: cmd/app/exec.go plugin wiring; pkg/manifestplugins/*/plugin.go; pkg/nodeplugins/*/plugin.go -->

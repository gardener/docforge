## Context

Docforge hardcodes `const sectionFile = "_index.md"` to identify section index files — nodes that define section metadata without fetching upstream content. Hugo uses `_index.md` as its section index convention, but VitePress and other modern SSGs use `index.md`. When repos migrate from Hugo to VitePress and rename their section files, docforge silently breaks: it treats `index.md` as a normal content file, attempts source resolution, and fails.

The docsy plugin already checks for both filenames (`pkg/manifestplugins/docsy/plugin.go:22`), proving this gap was recognized but never propagated to the core resolution logic.

## Goals / Non-Goals

**Goals:**

- Recognize `index.md` (without underscore) as a valid section index file alongside `_index.md`
- Maintain identical behavior for existing `_index.md` manifests
- Keep the change minimal and contained — no new configuration surfaces

**Non-Goals:**

- Making the set of section file names user-configurable (the set is well-known and stable)
- Changing `--hugo-section-files` CLI behavior or FSWriter output normalization
- Refactoring the entire index file handling system
- Supporting arbitrary filenames as section indices

## Decisions

### Decision 1: Package-level helper function over configuration

**Choice:** Add `isSectionFile(name string) bool` helper that checks both names.

**Alternatives considered:**

| Option | Description | Why rejected |
|--------|-------------|--------------|
| Wire `IndexFileNames` config into resolution | Thread existing config through manifest resolver | Unnecessary plumbing for a two-element set that won't grow |
| Unify under `--hugo-section-files` flag | Single config point for all section file handling | High-risk refactor, mixes output normalization with input detection |

**Rationale:** The set `{"_index.md", "index.md"}` is a well-known convention pair unlikely to expand. A simple predicate function is easier to audit, test, and maintain than configuration threading.

### Decision 2: Inline checks for cross-package call sites

**Choice:** Use inline `name == "_index.md" || name == "index.md"` in packages outside `pkg/manifest/` (fswriter, frontmatter, alias plugin) rather than exporting the helper.

**Rationale:** Exporting a helper from `pkg/manifest` would create import dependencies from packages that currently don't depend on it. The two-name check is trivial enough that duplication is preferable to coupling.

### Decision 3: Preserve TrimSuffix ordering in HugoPrettyPath

**Choice:** Trim `_index` before `index` in `HugoPrettyPath`.

**Rationale:** If `index` were trimmed first from `_index.md`, it would leave a trailing `_` in the path. The longer prefix must be stripped first.

## Risks / Trade-offs

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| `index.md` with a `source:` field treated as section file | None | N/A | Check requires BOTH `isSectionFile(name)` AND `source == ""` |
| Hugo output produces `index.md` instead of `_index.md` | None | N/A | FSWriter line 27 normalization is unchanged — output always uses `_index.md` in Hugo mode |
| Alias plugin generates wrong suffix for `index.md` | Low | Low | Covered by test — same empty-suffix behavior as `_index.md` |
| Future SSG introduces third convention name | Very low | Low | Add one more case to `isSectionFile` — trivial change |

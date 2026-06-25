## Why

The constant `sectionFile = "_index.md"` is hardcoded throughout docforge. When a node has `file: _index.md` and no `source:`, docforge skips source resolution and auto-generates a frontmatter-only section index. This is the mechanism that allows downstream manifests to define section metadata without fetching content from upstream sources.

The problem: `index.md` (without the underscore) does NOT trigger this special case. VitePress and other modern static site generators use `index.md` as their section index convention instead of Hugo's `_index.md`. When downstream repos rename `_index.md` to `index.md`, docforge treats the file as a normal content file, resolves its source, and fetches real content — breaking the overlay mechanism silently.

The docsy plugin (`pkg/manifestplugins/docsy/plugin.go:22`) already checks for both filenames, proving this was a known gap that was never propagated to the other checks.

## What Changes

- Replace `const sectionFile = "_index.md"` with a package-level helper function `isSectionFile(name string) bool` that matches both `_index.md` and `index.md`
- Update all hardcoded `_index.md` checks in production code to use the new helper or check both names
- Update `HugoPrettyPath` in `node.go` to also strip the `index` prefix (resolving an existing TODO)

## Design

### Approach chosen: Minimal helper function

Three approaches were considered:

| Approach | Description | Trade-off |
|----------|-------------|-----------|
| **A (chosen)** | Add `isSectionFile` helper, check both names | Minimal change, low risk, solves the problem |
| B | Wire `IndexFileNames` config into manifest resolution | More plumbing, configurable but unnecessary complexity |
| C | Unify all index file handling under `--hugo-section-files` | Full refactor, high risk for minimal gain |

**Rationale:** The set of valid section file names (`_index.md`, `index.md`) is well-known and unlikely to grow. A simple helper function is easier to understand, test, and maintain than threading configuration through the manifest resolution layer.

### Implementation

```go
// pkg/manifest/manifest.go

// isSectionFile returns true if the given filename is a section index file.
// Both Hugo (_index.md) and VitePress (index.md) conventions are supported.
func isSectionFile(name string) bool {
    return name == "_index.md" || name == "index.md"
}
```

**Call sites to update:**

1. `pkg/manifest/manifest.go:256` — `resolveManifestLinks` early return
   ```go
   // Before:
   if node.File == sectionFile && node.Source == "" {
   // After:
   if isSectionFile(node.File) && node.Source == "" {
   ```

2. `pkg/writers/fswriter.go:30` — frontmatter generation guard
   ```go
   // Before:
   if f.Hugo && name == "_index.md" && node != nil && node.Frontmatter != nil && docBlob == nil {
   // After:
   if f.Hugo && (name == "_index.md" || name == "index.md") && node != nil && node.Frontmatter != nil && docBlob == nil {
   ```
   Note: Line 27 (`name = "_index.md"`) stays unchanged — that's the output normalization for Hugo mode via `IndexFileNames`, which is a separate concern.

3. `pkg/nodeplugins/markdown/document/frontmatter/frontmatter.go:108` — fallback index detection
   ```go
   // Before:
   return name == "_index.md"
   // After:
   return name == "_index.md" || name == "index.md"
   ```

4. `pkg/manifestplugins/alias/plugin.go:40` — alias suffix for section files
   ```go
   // Before:
   if child.Name() == "_index.md" {
   // After:
   if child.Name() == "_index.md" || child.Name() == "index.md" {
   ```

5. `pkg/manifest/node.go:67` — `HugoPrettyPath` prefix stripping
   ```go
   // Before:
   name = strings.TrimSuffix(name, "_index")
   // After:
   name = strings.TrimSuffix(name, "_index")
   name = strings.TrimSuffix(name, "index")
   ```
   Note: order matters — `_index` must be trimmed first, otherwise `index` would leave a trailing `_`.

6. `pkg/manifestplugins/docsy/plugin.go:22` — already handles both, no change needed.

### What does NOT change

- The `const sectionFile` is removed, but its semantics are preserved via `isSectionFile`
- `--hugo-section-files` CLI flag behavior is unchanged (still controls output rename in FSWriter)
- The FSWriter's line 27 (`name = "_index.md"`) stays — this is Hugo output normalization, separate from section file detection
- All existing `_index.md` manifests continue working identically

## Capabilities

### New Capabilities

- Manifests can use `file: index.md` (without source) as a section index, equivalent to `file: _index.md`

### Modified Capabilities

- `section-file-detection`: Both `_index.md` and `index.md` without a `source:` field are treated as auto-generated section indices
- `alias-generation`: Alias suffix calculation treats `index.md` the same as `_index.md` (empty suffix)
- `frontmatter-generation`: The FSWriter generates frontmatter-only content for both `_index.md` and `index.md` when `docBlob` is nil
- `index-file-detection`: The frontmatter processor's `nodeIsIndexFile` fallback recognizes both names
- `hugo-pretty-path`: URL path generation correctly strips `index` prefix (not just `_index`)

## Impact

- **CLI**: No flag changes
- **Manifest schema**: `file: index.md` without `source:` now behaves identically to `file: _index.md` without `source:` — previously it would attempt (and likely fail) source resolution
- **Code**: ~5 files modified, no packages added or removed
- **Downstream consumers**: Repos that renamed `_index.md` to `index.md` (e.g., VitePress migrations) will work correctly without needing to revert the rename
- **Backwards compatibility**: Existing `_index.md` usage is entirely unaffected
- **Risk**: Low — the change is additive and each call site is a simple boolean expansion

## Testing

- Update existing `manifest_test.go` `_index.md` test entry to also cover `index.md` variant
- Add a test case in `resolveManifestLinks` for `file: index.md` with no source (should return nil)
- Update alias plugin tests to verify `index.md` gets empty suffix
- Update frontmatter tests to verify `index.md` is detected as index file

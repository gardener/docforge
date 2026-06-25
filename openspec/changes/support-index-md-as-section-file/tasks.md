## 1. Core Helper

- [ ] 1.1 Add `isSectionFile(name string) bool` helper in `pkg/manifest/manifest.go` and remove `const sectionFile`
- [ ] 1.2 Update `resolveManifestLinks` to use `isSectionFile(node.File)` instead of `node.File == sectionFile`

## 2. Cross-Package Call Sites

- [ ] 2.1 Update `pkg/writers/fswriter.go` frontmatter guard to check both `_index.md` and `index.md`
- [ ] 2.2 Update `pkg/nodeplugins/markdown/document/frontmatter/frontmatter.go` `nodeIsIndexFile` to return true for `index.md`
- [ ] 2.3 Update `pkg/manifestplugins/alias/plugin.go` to treat `index.md` as section file for alias suffix

## 3. HugoPrettyPath

- [ ] 3.1 Update `pkg/manifest/node.go` `HugoPrettyPath` to strip `index` prefix after `_index` (order matters)

## 4. Tests

- [ ] 4.1 Add test case in `manifest_test.go` for `file: index.md` with no source (should skip resolution)
- [ ] 4.2 Update alias plugin tests to verify `index.md` gets empty suffix
- [ ] 4.3 Update frontmatter tests to verify `index.md` is detected as index file
- [ ] 4.4 Add/update `HugoPrettyPath` test for `index.md` stripping

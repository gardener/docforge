# Changelog

## Unreleased

### New features

#### Content policy (`content-policy` branch)

Docforge now enforces a content-security policy on all rendered documents.

**What changed:**

- Raw `<script>` and `<style>` HTML blocks in untrusted Markdown sources are replaced
  with `<!-- docforge: stripped block -->` comments.
- Inline `<script>` or `<style>` tags cause the whole document to be replaced with a
  frontmatter-only placeholder.
- Non-Markdown sources with extensions `.vue`, `.js`, `.mjs`, `.ts` are refused (skipped);
  `.html` sources are included with a warning.
- The `head` frontmatter key is stripped from untrusted documents to prevent arbitrary
  `<link>`/`<script>` injection into the generated site's `<head>`.

**New manifest key — `allowScriptBlocks`:**

Set `allowScriptBlocks: true` on a `file`, `dir`, or `fileTree` node to mark it as
trusted and exempt it from the policy.
Trust cannot cross manifest boundaries.

```yaml
structure:
  - file: api-reference.md
    source: https://github.com/org/repo/blob/main/docs/api.md
    allowScriptBlocks: true
```

**New CLI flag — `--fail-on-stripped-content`:**

Pass `--fail-on-stripped-content` to exit with a non-zero status whenever content is
stripped or suppressed. Without this flag (the default), violations are logged as
warnings and processing continues.

**Migration note:**

No action is required for the common case. If your source repositories contain
intentional raw `<script>` blocks (e.g. API reference docs generated from code), add
`allowScriptBlocks: true` to the relevant manifest nodes to suppress the warnings and
retain the original output. See [Content policy](docs/content-policy.md) for details.

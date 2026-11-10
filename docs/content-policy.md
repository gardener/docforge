# Content policy

Docforge enforces a content-security policy on all rendered documents.
The policy prevents executable content — mainly raw `<script>` and `<style>` HTML — from silently flowing from external source repositories into the generated documentation bundle.

## What the policy does

| Situation | Default action |
|---|---|
| Top-level `<script>` or `<style>` HTML block | Replace with `<!-- docforge: stripped block -->` |
| HTML block that *contains* a `<script>` or `<style>` tag (e.g. inside `<pre>`) | Log a warning, keep the block unchanged |
| Inline `<script>` or `<style>` in paragraph text | Replace the whole document with a placeholder |
| Non-markdown source with extension `.vue`, `.js`, `.mjs`, `.ts` | Skip (do not include in output) |
| Non-markdown source with extension `.html` | Log a warning, include unchanged |
| Frontmatter `head` key in untrusted document | Remove the key (prevent arbitrary `<link>`/`<script>` injection into the site head) |

Code blocks (fenced or indented) are never inspected — only raw HTML in the rendered Markdown AST.

## Trusted documents

By default every node is **untrusted** and the policy applies.
A node can be marked trusted by its **parent** manifest author using `allowScriptBlocks`:

```yaml
structure:
  - file: api-reference.md
    source: https://github.com/org/repo/blob/main/docs/api.md
    allowScriptBlocks: true    # this file may contain <script>/<style> blocks
```

Trust is **not** inherited across manifest boundaries — a sub-manifest included via `manifest:` cannot grant itself trust by declaring `allowScriptBlocks: true` at its own top-level.
Only the root manifest author (or an intermediary that already holds trust) can grant it.

See [Manifest reference — `allowScriptBlocks`](../manifest-ref.md#allowscriptblocks) for the full trust model.

## Opting in to strict mode

Pass `--fail-on-stripped-content` to make docforge exit with a non-zero status whenever any content is stripped or suppressed by the policy.
Without this flag, violations are logged as warnings but processing continues.

```sh
docforge --fail-on-stripped-content --manifest manifest.yaml --destination out/
```

## Known limitations

- Only the **default** policy is supported; there is no way to configure custom rules in the manifest today.
- WarnBlock findings are always warnings, even with `--fail-on-stripped-content`.
  Use `allowScriptBlocks: true` on trusted sources to suppress them.
- Inline script/style in a **multi-source** node suppresses only the single source that triggered it; remaining sources are still rendered.
  If you need the full document to be replaced, all sources must be assessed together.

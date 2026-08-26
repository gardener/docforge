# How-to guides

Practical step-by-step guides for common docforge tasks.

---

## Your first forge run (quick start)

This walkthrough takes you from zero to a working bundle in four steps.

### Step 1 — Get a GitHub token

You need a Personal Access Token from GitHub (or from your corporate GitHub Enterprise host).

1. Go to **github.com → Settings → Developer settings → Personal access tokens**
2. Create a token with **repo (read)** scope
3. Export it as an environment variable:

```sh
export GITHUB_OAUTH_TOKEN=ghp_yourtoken
```

### Step 2 — Point `-f` at your manifest

Docforge accepts a **local filesystem path** or a **GitHub `blob` URL** for `-f`.

**Local path (no GitHub token needed for the manifest itself):**

```sh
docforge \
  -f ./example/getting-started.yaml \
  -d /tmp/docforge-output \
  --github-oauth-env-map github.com=GITHUB_OAUTH_TOKEN \
  --dry-run
```

Any relative or absolute path is detected automatically and resolved to an absolute path before loading. Sources referenced *inside* the manifest are still fetched normally — absolute GitHub URLs go to the GitHub host, and relative paths (e.g. `./README.md`) are resolved relative to the manifest's directory and read from disk.

**Remote GitHub URL (the previous default, still fully supported):**

```sh
docforge \
  -f https://github.com/gardener/docforge/blob/master/example/getting-started.yaml \
  -d /tmp/docforge-output \
  --github-oauth-env-map github.com=GITHUB_OAUTH_TOKEN \
  --dry-run
```

If you want to remap a GitHub URL prefix to a local directory for the *source files* (e.g. to avoid fetching sources over the network during development), add a `resourceMappings` entry to `~/.docforge/config`:

```yaml
# ~/.docforge/config
resourceMappings:
  https://github.com/myorg/myrepo/blob/main: /path/to/local/checkout
```

### Step 3 — Dry run (inspect the resolved node tree)

```sh
docforge \
  -f https://github.com/gardener/docforge/blob/master/example/getting-started.yaml \
  -d /tmp/docforge-output \
  --github-oauth-env-map github.com=GITHUB_OAUTH_TOKEN \
  --dry-run
```

`--dry-run` prints the resolved manifest node tree to stdout. **It does not prevent files from being written** — node processing and downloads still run. Its only effect is to print the node tree and suppress `--clean-destination`.

### Step 4 — Real run

Remove `--dry-run`. Docforge writes the bundle to `/tmp/docforge-output`.

```sh
docforge \
  -f https://github.com/gardener/docforge/blob/master/example/getting-started.yaml \
  -d /tmp/docforge-output \
  --github-oauth-env-map github.com=GITHUB_OAUTH_TOKEN
```

**`--github-oauth-env-map`** tells docforge which environment variable holds the token for which host. For a corporate GitHub Enterprise instance it looks like this:

```sh
--github-oauth-env-map github.com=GITHUB_OAUTH_TOKEN,github.mycompany.com=GHE_TOKEN
```

---

## Build a manifest with mixed sources

Real-world manifests combine `dir`, `file`, and `fileTree` nodes. Here is a worked example that mirrors a common pattern: a multi-section portal where some files are cherry-picked and one whole directory tree is included but a few files are excluded and re-included at a different location.

```yaml
structure:
- dir: Installation
  structure:
  - file: https://github.com/org/repo/blob/main/installation-guide/README.md
    frontmatter:
      title: Installation
      weight: 1
  - fileTree: https://github.com/org/repo/tree/main/installation-guide/template_files

- dir: Operations
  structure:
  - fileTree: https://github.com/org/repo/tree/main/concepts
    excludeFiles:
    - README.md
    - gardener-architecture.md
  - file: README.md
    frontmatter:
      title: Operations
      weight: 2
    source: https://github.com/org/repo/blob/main/concepts/gardener-architecture.md

- dir: Troubleshooting
  structure:
  - fileTree: https://github.com/org/repo/tree/main/debugging
    excludeFiles:
    - README.md
  - file: https://github.com/org/repo/blob/main/debugging/README.md
    frontmatter:
      title: Troubleshooting
      weight: 3
```

**What each part does:**

- **`dir:`** — creates a named output directory. Everything nested under its `structure:` lands inside that directory.

- **`file:`** with a URL — fetches a single file. The output filename is derived from the URL unless you give the node an explicit name (the value of `file:`).

- **`file: README.md` + `source:`** — names the output file `README.md` but fetches content from the URL in `source:`. Use this to rename a file or to place a file from one location under a predictable output name.

- **`fileTree:`** — fetches an entire GitHub directory recursively, preserving the subdirectory structure.

- **`excludeFiles:`** — list of path prefixes to skip within the `fileTree`. Matching uses a prefix check, so `README.md` excludes every file whose path within the tree starts with `README.md`.

- **`frontmatter:`** — injects YAML frontmatter into the output file. `weight:` controls sort order in Hugo and gen-toc output: lower numbers sort first; weighted entries sort before unweighted ones.

**Why exclude and then re-include?**

In the `Operations` section above, `gardener-architecture.md` is excluded from the `fileTree` and then re-added as a standalone `file` node with a custom `frontmatter`. This is the standard pattern for *promoting* a file: pull it out of the alphabetically-ordered tree and place it at a specific position with a controlled title and weight.

The same pattern is used in `Troubleshooting`: the `README.md` at the root of the `debugging/` tree would be placed inside `Troubleshooting/debugging/README.md` by the `fileTree`. Excluding it and re-adding it as a plain `file` node places it directly under `Troubleshooting/README.md` instead.

---

## Forge a bundle from GitHub sources

**Prerequisites:** a GitHub personal access token with `repo` (read) scope.

```sh
export GITHUB_TOKEN=<your-token>

docforge \
  -f manifest.yaml \
  -d /tmp/docforge-out \
  --github-oauth-env-map github.com=GITHUB_TOKEN
```

<!-- verified: cmd/app/initilization.go initRepositoryHosts() — host=EnvCredentials key, envVar=value, os.Getenv(envVar) -->

**Dry run first** — preview the output tree without writing files:

```sh
docforge \
  --dry-run \
  -f manifest.yaml \
  --github-oauth-env-map github.com=GITHUB_TOKEN
```

<!-- verified: cmd/app/flags.go --dry-run bool false; exec.go config.DryRun check -->

---

## Use GitHub Enterprise

Pass the GHE hostname as the key in `--github-oauth-env-map`:

```sh
export GHE_TOKEN=<your-ghe-token>

docforge \
  -f manifest.yaml \
  -d /tmp/docforge-out \
  --github-oauth-env-map github.mycompany.com=GHE_TOKEN
```

docforge registers the hostname as a known host at startup and uses the GitHub Enterprise API endpoint automatically.

<!-- verified: cmd/app/initilization.go buildClient() — host != "https://github.com" → github.NewEnterpriseClient(host, ...) -->
<!-- verified: pkg/registry/repositoryhost/resource_url.go RegisterHost() — adds host to knownHosts, rebuilds URL regexps -->

You can supply multiple hosts in a single flag:

```sh
--github-oauth-env-map github.com=GITHUB_TOKEN,github.mycompany.com=GHE_TOKEN
```

**Limitation:** the token environment variable must be non-empty. If the variable is set but empty, docforge exits with an error.
<!-- verified: cmd/app/initilization.go — oAuthToken == "" → return nil, fmt.Errorf("%s's OAUTH ENV variable is empty", host) -->

---

## Build a Hugo-compatible bundle

`--hugo` is enabled by default. The following transformations are applied:

- Files named `readme.md` or `README.md` are renamed to `_index.md`.
- Links are rewritten to Hugo pretty-URL format (`guide.md` → `/guide/`).

```sh
docforge \
  -f manifest.yaml \
  -d /tmp/docforge-out \
  --github-oauth-env-map github.com=GITHUB_TOKEN
```

To customise which filenames become `_index.md`:

```sh
--hugo-section-files readme.md,README.md,index.md
```

<!-- verified: cmd/app/flags.go --hugo-section-files default [readme.md,README.md] -->

To rewrite all relative links to root-relative (useful when the bundle is served from a sub-path):

```sh
--hugo-base-url /docs
```

<!-- verified: cmd/app/flags.go --hugo-base-url string "" -->

---

## Build a non-Hugo bundle

Pass `--hugo=false` to disable all Hugo-specific processing. Files are written with their original names and links are not rewritten to pretty-URL format.

```sh
docforge \
  -f manifest.yaml \
  -d /tmp/docforge-out \
  --hugo=false \
  --github-oauth-env-map github.com=GITHUB_TOKEN
```

<!-- verified: cmd/hugo/option.go Hugo.Enabled mapstructure:"hugo"; cmd/app/flags.go default true -->

---

## Filter by file type

By default docforge processes all file types found in a `fileTree`. To restrict to specific extensions:

```sh
--content-files-formats .md,.html
```

Files with extensions not in the list are excluded from the output.

**Note:** when the flag is empty (the default), all files pass through — this is the opposite of what the flag name "supported formats" implies.
<!-- verified: cmd/app/exec.go len(options.Options.ContentFileFormats) > 0 guard; pkg/manifestplugins/filetypefilter/plugin.go -->

---

## Fetch GitHub commit metadata (author, dates)

To write a `.json` sidecar next to each output file containing commit metadata:

```sh
docforge \
  -f manifest.yaml \
  -d /tmp/docforge-out \
  --github-oauth-env-map github.com=GITHUB_TOKEN \
  --github-info-destination __github-info
```

The sidecar files are written under `<destination>/<github-info-destination>/` with a `.json` extension. Each file contains:

```json
{
  "lastmod": "2024-03-01 12:00:00",
  "publishdate": "2022-01-10 09:00:00",
  "author": { ... },
  "contributors": [ ... ],
  "weburl": "https://github.com/org/repo",
  "sha": "abc123",
  "shaalias": "main",
  "path": "docs/guide.md"
}
```

<!-- verified: pkg/registry/repositoryhost/github_info.go GitInfo struct json tags -->
<!-- verified: cmd/app/initilization.go getReactorConfig() — GitInfoWriter = FSWriter{Root: dest+ghInfoDest, Ext: "json"} -->

---

## Generate a navigation TOC

```sh
docforge gen-toc \
  -f manifest.yaml \
  --github-oauth-env-map github.com=GITHUB_TOKEN \
  -o toc.yaml
```

To strip the top-level directory from all paths in the output:

```sh
docforge gen-toc \
  -f manifest.yaml \
  --github-oauth-env-map github.com=GITHUB_TOKEN \
  --strip-root \
  -o toc.yaml
```

<!-- verified: cmd/app/gentoc.go --strip-root flag -->

---

## Limitations and edge cases

### `markdown-enabled` is required for Markdown processing

By default (`markdown-enabled` not set), `.md` files are treated as opaque blobs: they are downloaded and written as-is with **no link rewriting, no frontmatter propagation, and no Hugo transformations**. All the link-rewriting and Hugo behavior described in this guide requires `markdown-enabled: true` in `~/.docforge/config`.

This is a config-file-only setting — there is no `--markdown-enabled` CLI flag.

```yaml
# ~/.docforge/config
markdown-enabled: true
```

<!-- verified: cmd/app/exec.go:72-75 — manifest/markdown plugin only registered when MarkdownEnabled=true -->
<!-- verified: pkg/manifest/manifest.go:402-404 setDefaultProcessor — assigns "downloader" to all file nodes when markdown plugin is absent -->

### `--dry-run` does not prevent file writes

`--dry-run` prints the resolved manifest node tree to stdout and suppresses `--clean-destination`. It does **not** skip node processing or downloads — files are still written to `--destination`.

<!-- verified: cmd/app/exec.go:90-92 — only effect is fmt.Println(documentNodes[0]); core.Run still executes -->

### No cascading document downloads

Docforge only downloads files explicitly listed in the manifest (via `file`, `fileTree`, or `multiSource`). It does **not** follow hyperlinks in downloaded documents and download their targets. This is intentional — it ensures predictable, reproducible output.
<!-- verified: docs/consistency.md — "Cascading download of documents based on hyperlinks in their content is not supported intentionally" -->

### Only GitHub and GitHub Enterprise are supported as remote hosts

Docforge recognises URLs matching `https://github.com/...`, `https://raw.githubusercontent.com/...`, and any host registered via `--github-oauth-env-map`. GitLab, Bitbucket, and other hosts are not supported.
<!-- verified: pkg/registry/repositoryhost/resource_url.go knownHosts — default ["github.com"], comment "TODO: extend for GitLab" -->

### The token environment variable must be non-empty

Every host in `--github-oauth-env-map` must map to an environment variable that is set and non-empty at the time docforge starts. An empty variable is a startup error, not a warning.
<!-- verified: cmd/app/initilization.go oAuthToken == "" → return nil, fmt.Errorf -->

### No repository hosts loaded = startup error

If `--github-oauth-env-map` is not provided or all entries fail, docforge exits immediately with `"no resource handlers were loaded"`.
<!-- verified: cmd/app/initilization.go len(rhs) == 0 → return fmt.Errorf("no resource handlers were loaded...") -->

### Duplicate file names in the same directory are an error

If two nodes in the same `dir` resolve to the same output filename, docforge exits with a collision error before writing any files.
<!-- verified: pkg/manifest/manifest.go mergeFolders() — nodeNameToNode collision check → fmt.Errorf("file...causes collision with...") -->

### Multiple dirs with the same name and frontmatter is an error

When two `dir` nodes merge (same name under the same parent), both having `frontmatter` set, docforge exits with an error. Only one of the merged dirs may carry `frontmatter`.
<!-- verified: pkg/manifest/manifest.go mergeFolders() → fmt.Errorf("there are multiple dirs with name %s...that have frontmatter") -->

### `hugo-structural-dirs` must be directory names, not paths

Entries in `--hugo-structural-dirs` must be bare directory names (e.g. `content`). If any entry contains a `/`, docforge exits at startup.
<!-- verified: cmd/app/exec.go — strings.Contains(dir, "/") → return fmt.Errorf("hugo-structural-dirs contains a path instead a directory name") -->

### Fault-tolerant vs fail-fast processing

By default, docforge is fault-tolerant: if a single file fails to download or process, it logs the error and continues with the remaining files. Pass `--fail-fast` to stop immediately on the first error.
<!-- verified: pkg/workers/taskqueue/taskqueue.go failFast field — stops queue on first error when true -->

### HTTP response cache

Docforge caches GitHub HTTP responses on disk under `--cache-dir` (default `~/.docforge`). Stale cache entries are served across runs. To force a fresh fetch, remove the cache directory before running.
<!-- verified: cmd/app/initilization.go buildClient() — diskcache backed by diskv at cachePath; cmd/app/cmd.go DocforgeHomeDir = ".docforge" -->

### Worker count limits

`--document-workers` and `--download-workers` must be in the range `[1, 100]`. Passing `0` or a value above `100` is a startup error.

`--download-workers` also controls the concurrency of the GitHub commit-info fetcher (used when `--github-info-destination` is set) — it is not only for resource downloads.
<!-- verified: pkg/workers/taskqueue/taskqueue.go minWorkerSize=1 maxWorkerSize=100; pkg/nodeplugins/markdown/plugin.go:30 githubinfo uses resourceDownloadWorkersCount -->

### Local manifest paths and remote sources

Passing a local filesystem path to `-f` is supported directly — no `resourceMappings` config is required for the manifest itself.

```sh
docforge -f ./my-manifest.yaml -d /tmp/out --github-oauth-env-map github.com=GITHUB_TOKEN
```

Detection is automatic: if the value passed to `-f` has no URL scheme (or uses `file://`), it is treated as a local path and resolved to an absolute path. A GitHub URL (`https://...`) continues to be fetched from the network as before.

**Four manifest × source combinations**

| Manifest | Sources in manifest | How it works |
|---|---|---|
| Local path (`-f ./manifest.yaml`) | Absolute GitHub URLs | Manifest read from disk; sources fetched from GitHub host as usual |
| Local path (`-f ./manifest.yaml`) | Relative paths (`./README.md`) | Manifest and sources both read from disk; relative paths resolved relative to the manifest directory |
| GitHub URL | Absolute GitHub URLs | Existing behaviour — fully unchanged |
| GitHub URL | Remapped via `resourceMappings` | Existing behaviour — fully unchanged |

`resourceMappings` is still useful when you want to remap *source* GitHub URLs to a local directory, regardless of where the manifest lives:

```yaml
# ~/.docforge/config
resourceMappings:
  https://github.com/myorg/myrepo/blob/main: /path/to/local/checkout
```
<!-- verified: cmd/app/exec.go IsLocalPath + NewLocalPath registration; pkg/registry/repositoryhost/local_path.go LocalPath host; cmd/app/local_manifest_test.go TestCaseA–D -->

### Configuration file and environment overrides

The default config file location is `~/.docforge/config`. Override it with the `DOCFORGE_CONFIG` environment variable:

```sh
DOCFORGE_CONFIG=/path/to/my/config.yaml docforge -f ...
```

Any config-file setting can also be supplied as an environment variable via Viper's automatic env binding. For example, `MARKDOWN_ENABLED=true` is equivalent to `markdown-enabled: true` in the config file.
<!-- verified: cmd/app/cmd.go:84-88 vip.AutomaticEnv(), DOCFORGE_CONFIG env var, vip.AddConfigPath/SetConfigName/ReadInConfig -->

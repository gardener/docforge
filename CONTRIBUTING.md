# Contributing to docforge

## Prerequisites

- Go 1.23+ (go.mod declares `go 1.21.0` but `toolchain go1.23.0` — a clean 1.21 install will pull 1.23 automatically)
- `greadlink` on macOS (`brew install coreutils`)
- A GitHub personal access token for integration and e2e tests
<!-- verified: go.mod — go 1.21.0; toolchain go1.23.0 -->

## Build

```sh
# Local build (output: bin/docforge)
make build-local

# CI build (all platforms, output: bin/rel/)
make build
```

<!-- verified: Makefile build-local → LOCAL_BUILD=1 .ci/build; build → .ci/build -->

## Unit tests

```sh
make test
```

Runs all tests under `cmd/` and `pkg/` using the Ginkgo framework.
<!-- verified: .ci/test — ginkgo -r cmd pkg -->

With coverage:

```sh
make test-cov
# opens docforge.coverage.html
```

<!-- verified: Makefile test-cov → COVERAGE=1 .ci/test; go tool cover -html → docforge.coverage.html -->

## Lint and vet

```sh
make check
```

Runs the following in order, installing tools on first run:

1. `go vet` on all packages
2. `gofmt -l -w` — formats all source files **in place** (rewrites them, not just checks)
3. `golangci-lint run`
4. `golint` on all non-test `.go` files

<!-- verified: .ci/check — go vet, then gofmt -l -w, then golangci-lint run, then golint loop -->

## Integration tests

The integration test builds `bin/docforge`, writes a temporary config file, and runs docforge with the manifest fetched from `https://github.com/gardener/docforge/blob/master/integration-test/manifest.yaml`. A `resourceMappings` entry in that config maps the GitHub URL prefix back to the local checkout, so local changes are tested without pushing. The output is diffed against `integration-test/expected-tree/`.

```sh
export GITHUB_OAUTH_TOKEN=<your-token>
make integration-test
```

<!-- verified: .ci/integration-test — GIT_OAUTH_TOKEN="${GITHUB_OAUTH_TOKEN:-...}"; writes config with manifest GitHub URL + resourceMappings pointing to local repo; diffs output against integration-test/expected-tree -->

## E2e tests

The e2e test diffs real docforge output against golden files.

```sh
make e2e
```

<!-- verified: Makefile e2e → test/e2e/diff.sh -->

## Run all checks at once

```sh
make verify
```

Runs `check`, `test`, `integration-test`, and `e2e` in sequence.
<!-- verified: Makefile verify: check test integration-test e2e -->

## Regenerate command reference docs

After changing any flag or subcommand, regenerate the Markdown reference under `docs/cmd-ref/`:

```sh
make build   # required first — docs-gen uses bin/rel/docforge-<os>-amd64
make docs-gen
```

`make docs-gen` exits immediately if `bin/rel/` does not exist.
<!-- verified: .ci/docs-gen — checks [[ ! -d "${BINARY}/rel" ]] → exit 1; then runs bin/rel/docforge-<os>-amd64 gen-cmd-docs -d docs/cmd-ref -->

## Check manifest

```sh
make check-manifest
```

Downloads and runs the shared `check-manifest` script from `gardener/documentation` against `.docforge/manifest.yaml`, checking that the manifest and the `docs/` directory are consistent.
<!-- verified: .ci/check-manifest — curls check-manifest entrypoint + check-manifest-config from gardener/documentation master; runs entrypoint with --repo-name docforge --use-token false --manifest-path .docforge/manifest.yaml --diff-dirs .docforge/;docs/ (plus --repo-path and --config-path pointing to local checkout and downloaded config) -->

## Generate counterfeiter fakes

```sh
make generate
```

Runs `go generate ./...` to regenerate fakes under `*fakes/` directories.
<!-- verified: Makefile generate → go generate ./... -->

## Commit workflow

Use `task.sh` to run all checks before committing:

```sh
git add <files>          # stage everything first — task.sh exits if there are unstaged or untracked files
./task.sh "your commit message"
```

`task.sh` does the following:

1. Exits if there are any unstaged changes or untracked files — it does **not** stage files for you.
2. Runs `.ci/check`, `.ci/test`, `.ci/integration-test` in sequence.
3. Creates a temporary commit `task-changes-for-e2e` on the current branch (required by `test/e2e/diff.sh`), runs the e2e test, then removes it with `git reset --soft HEAD~1`.
4. Computes the longest common **directory** prefix of the staged file paths (falling back to `[tested] ` when no common directory exists) and prepends it to your commit message, then creates the final commit.

<!-- verified: task.sh — exits when created_files or modified_files non-empty; git commit -m"task-changes-for-e2e" before e2e; git reset --soft HEAD~1 after; longest_common_prefix strips path segments until common prefix found or falls back to "[tested] " -->

## Gardener contributor guide

For PR process, code review, and community guidelines see the [Gardener contributor guide](https://gardener.cloud/docs/contribute).

## Why

The link validation feature makes outbound HTTP HEAD/GET requests to validate absolute URLs found in markdown documents. This feature is not used in production (always run with `--skip-link-validation: true`) and exposes the host environment to blind SSRF — any absolute URL in processed markdown triggers server-side HTTP requests with insufficient filtering.

## What Changes

- **BREAKING**: Remove the `--validation-workers` CLI flag
- **BREAKING**: Remove the `--skip-link-validation` CLI flag
- **BREAKING**: Remove the `--hosts-to-report` CLI flag
- **BREAKING**: Remove the `skipValidation` field from manifest node definitions
- Remove the entire `pkg/nodeplugins/markdown/linkvalidator/` package
- Remove link validation wiring from the markdown node plugin
- Remove `SkipValidation` propagation from the markdown manifest plugin

## Capabilities

### New Capabilities

_None — this is a removal._

### Modified Capabilities

- `markdown-processing`: The markdown document worker will no longer attempt to validate absolute URLs via HTTP. Links are still resolved and rewritten, but external reachability is no longer checked.

## Impact

- **CLI**: Three flags removed (`--validation-workers`, `--skip-link-validation`, `--hosts-to-report`). Existing configs that set these will produce unknown-flag errors.
- **Manifest schema**: `skipValidation` field on nodes becomes invalid YAML (silently ignored by strict parsing or erroring depending on decoder settings).
- **Code**: ~6 files modified, 1 package directory deleted.
- **Downstream consumers**: https://github.com/gardener/documentation runs docforge with link validation disabled (`skip-link-validation: true`), so no behavioral change.
- **Security**: Eliminates the blind SSRF attack surface entirely.

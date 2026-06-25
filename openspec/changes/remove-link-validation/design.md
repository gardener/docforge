## Context

Docforge processes markdown documentation from distributed repositories. During markdown processing, the link resolver identifies absolute URLs that don't point to known repository resources and forwards them to a link validator. The validator issues outbound HTTP HEAD/GET requests to check link reachability.

In practice, all known consumers run with `skip-link-validation: true`, meaning the feature is never exercised. The feature also introduces a blind SSRF vector since attacker-controlled URLs in markdown can trigger server-side HTTP requests.

## Goals / Non-Goals

**Goals:**
- Eliminate the SSRF attack surface by removing the link validation feature entirely
- Simplify the codebase by removing unused code paths
- Ensure zero change to documentation output (the feature only logs warnings, never modifies content)

**Non-Goals:**
- Replacing link validation with a safer alternative (not needed since it's unused)
- Changing link resolution behavior (relative links, repository links stay untouched)
- Modifying the downloader or any other HTTP-making component

## Decisions

### 1. Full removal vs. hardening

**Decision**: Remove entirely rather than adding IP filtering/allowlists.

**Rationale**: The feature is unused in all known deployments. Adding SSRF protections (private IP blocking, DNS rebinding protection, metadata endpoint blocking) adds complexity to dead code. Removal is simpler, smaller, and more secure by default.

**Alternative considered**: Add a comprehensive denylist (RFC 1918, link-local, cloud metadata IPs) — rejected because it still requires maintenance and the feature provides no value.

### 2. Breaking CLI flags vs. soft deprecation

**Decision**: Remove flags immediately with no deprecation period.

**Rationale**: The feature runs disabled. Any config that sets `skip-link-validation: true` will get an unknown-flag error, but this is easy to fix (delete the line). A deprecation period adds code for a feature nobody uses.

### 3. Remove SkipValidation from Node struct

**Decision**: Remove the `SkipValidation` field from the manifest Node type.

**Rationale**: With no validator, there's nothing to skip. The field serves no purpose. YAML manifests using this field will have it silently ignored by Go's YAML decoder (it uses struct tags, unknown fields are dropped).

## Risks / Trade-offs

- **[Unknown consumers]** → If any undiscovered deployment uses link validation actively, they lose the feature. Mitigation: the feature only logs warnings, never fails the build or modifies output. Impact is zero functional change.
- **[Config file errors]** → Configs with `--skip-link-validation` or `--validation-workers` will fail on CLI parse. Mitigation: easy one-line fix; document in release notes.
- **[Future need]** → If link validation is ever needed again, it must be reimplemented. Mitigation: git history preserves the code; reimplementation with proper SSRF protections would be a better starting point anyway.

## Verification

Build docforge after removal and run it against the gardener/documentation project:
- https://github.com/gardener/documentation

Delete `hugo/` if it exists, run docforge to produce the output, SHA-256 hash the `hugo/` directory, then delete it. Repeat with the pre-change binary. Compare hashes to confirm zero output difference.

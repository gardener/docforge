## REMOVED Requirements

### Requirement: Link validation via HTTP requests
The system SHALL NOT validate absolute URL reachability by issuing outbound HTTP requests.

**Reason**: The feature is unused in all known deployments and introduces a blind SSRF vulnerability. All consumers run with link validation disabled.

**Migration**: Remove `skip-link-validation`, `validation-workers`, and `hosts-to-report` flags from configuration files. These flags are no longer recognized.

#### Scenario: Absolute URLs are not validated via HTTP
- **WHEN** a markdown document contains an absolute URL (e.g., `https://example.com/page`)
- **THEN** the system SHALL NOT issue any outbound HTTP request to that URL

#### Scenario: Removed CLI flags produce error
- **WHEN** docforge is invoked with `--skip-link-validation` or `--validation-workers` or `--hosts-to-report`
- **THEN** the CLI SHALL reject the command with an unknown flag error

### Requirement: SkipValidation manifest field removed
The system SHALL NOT recognize `skipValidation` as a node property in manifest YAML.

**Reason**: With no link validator, this field has no effect.

**Migration**: Remove `skipValidation` from manifest YAML files. The field is silently ignored by the Go YAML decoder if present.

#### Scenario: Manifest with skipValidation field
- **WHEN** a manifest YAML contains `skipValidation: true` on a node
- **THEN** the system SHALL silently ignore the field (no error, no behavioral effect)

## MODIFIED Requirements

### Requirement: Markdown link resolution
The markdown document worker SHALL resolve links in documents by rewriting relative paths and repository references. Absolute URLs that do not reference known repository resources SHALL be returned unmodified without any reachability check.

#### Scenario: Absolute URL not in repository
- **WHEN** a markdown document contains an absolute URL that is not a known repository resource
- **THEN** the system SHALL return the URL unchanged without issuing any HTTP request

#### Scenario: Repository resource URL
- **WHEN** a markdown document contains an absolute URL that matches a known repository resource
- **THEN** the system SHALL resolve it according to the manifest structure (existing behavior, unchanged)

#### Scenario: Relative link resolution
- **WHEN** a markdown document contains a relative link
- **THEN** the system SHALL resolve it relative to the node's position in the manifest tree (existing behavior, unchanged)

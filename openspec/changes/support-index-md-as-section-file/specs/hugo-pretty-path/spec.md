## MODIFIED Requirements

### Requirement: HugoPrettyPath strips both index prefixes
The `HugoPrettyPath` function SHALL strip both `_index` and `index` prefixes from filenames when generating URL paths. The `_index` prefix MUST be stripped before `index` to avoid leaving a trailing underscore.

#### Scenario: _index.md is stripped to produce clean path
- **WHEN** `HugoPrettyPath` processes a node with file `_index.md`
- **THEN** the resulting path SHALL have the `_index` portion removed

#### Scenario: index.md is stripped to produce clean path
- **WHEN** `HugoPrettyPath` processes a node with file `index.md`
- **THEN** the resulting path SHALL have the `index` portion removed

#### Scenario: Regular filename is not affected by index stripping
- **WHEN** `HugoPrettyPath` processes a node with file `getting-started.md`
- **THEN** the resulting path SHALL retain the filename (minus `.md` extension)

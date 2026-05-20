## MODIFIED Requirements

### Requirement: Fallback index file detection recognizes both conventions
The frontmatter processor's `nodeIsIndexFile` fallback SHALL return true for files named `_index.md` OR `index.md`.

#### Scenario: _index.md is detected as index file
- **WHEN** the frontmatter processor checks if `_index.md` is an index file
- **THEN** it SHALL return true

#### Scenario: index.md is detected as index file
- **WHEN** the frontmatter processor checks if `index.md` is an index file
- **THEN** it SHALL return true

#### Scenario: Regular file is not detected as index file
- **WHEN** the frontmatter processor checks if `overview.md` is an index file
- **THEN** it SHALL return false

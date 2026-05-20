## MODIFIED Requirements

### Requirement: Alias suffix for section index files
The alias plugin SHALL assign an empty suffix to nodes whose name equals `_index.md` OR `index.md`. All other nodes SHALL receive a suffix derived from their name.

#### Scenario: Node named _index.md gets empty alias suffix
- **WHEN** a child node has name `_index.md`
- **THEN** the alias plugin SHALL assign an empty string as the alias suffix

#### Scenario: Node named index.md gets empty alias suffix
- **WHEN** a child node has name `index.md`
- **THEN** the alias plugin SHALL assign an empty string as the alias suffix

#### Scenario: Node with regular name gets normal alias suffix
- **WHEN** a child node has name `getting-started.md`
- **THEN** the alias plugin SHALL assign a suffix derived from the node name

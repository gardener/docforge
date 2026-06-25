## MODIFIED Requirements

### Requirement: Section file detection skips source resolution
The manifest resolver SHALL treat a node as a section index file when its `file` field equals `_index.md` OR `index.md` AND its `source` field is empty. Section index files MUST NOT undergo source resolution — they are auto-generated with frontmatter only.

#### Scenario: Node with file _index.md and no source is a section file
- **WHEN** a manifest node has `file: _index.md` and `source: ""`
- **THEN** the resolver SHALL skip source resolution and return nil

#### Scenario: Node with file index.md and no source is a section file
- **WHEN** a manifest node has `file: index.md` and `source: ""`
- **THEN** the resolver SHALL skip source resolution and return nil

#### Scenario: Node with file index.md and a source is NOT a section file
- **WHEN** a manifest node has `file: index.md` and `source: "https://example.com/content.md"`
- **THEN** the resolver SHALL proceed with normal source resolution

#### Scenario: Node with a non-index filename is NOT a section file
- **WHEN** a manifest node has `file: readme.md` and `source: ""`
- **THEN** the resolver SHALL proceed with normal resolution (not treated as section file)

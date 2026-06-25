## MODIFIED Requirements

### Requirement: Frontmatter-only generation for section index files
The FSWriter in Hugo mode SHALL generate frontmatter-only content for nodes named `_index.md` OR `index.md` when the document blob is nil and frontmatter is present.

#### Scenario: Hugo mode generates frontmatter for _index.md with nil blob
- **WHEN** Hugo mode is enabled AND name is `_index.md` AND node has frontmatter AND docBlob is nil
- **THEN** the FSWriter SHALL write a file containing only the frontmatter

#### Scenario: Hugo mode generates frontmatter for index.md with nil blob
- **WHEN** Hugo mode is enabled AND name is `index.md` AND node has frontmatter AND docBlob is nil
- **THEN** the FSWriter SHALL write a file containing only the frontmatter

#### Scenario: Non-index file with nil blob does not get frontmatter generation
- **WHEN** Hugo mode is enabled AND name is `readme.md` AND docBlob is nil
- **THEN** the FSWriter SHALL NOT generate frontmatter-only content

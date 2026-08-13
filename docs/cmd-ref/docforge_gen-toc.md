## docforge gen-toc

Generate a navigation YAML from a Docforge manifest

### Synopsis

Reads a Docforge manifest and derives a navigation structure from it.

The generated YAML reflects the dir / file / fileTree hierarchy defined in the
manifest. It can be used as input for VitePress, MkDocs, or any other site
generator that consumes a navigation file.

Each entry includes a title resolved from (in order of priority):
  1. manifest frontmatter.title
  2. document frontmatter.title (read from the source .md file)
  3. filename derivation (hyphens/underscores → spaces, Title case)

```
docforge gen-toc [flags]
```

### Options

```
      --cache-dir string                      Directory for the repository HTTP cache. (default "$HOME/.docforge")
      --github-oauth-env-map stringToString   Map between GitHub instances and ENV variable names that hold access tokens. (default [])
  -h, --help                                  help for gen-toc
      --index-file-names strings              Filenames treated as section index files (promoted to section entry). (default [readme.md,README.md,index.md])
  -f, --manifest string                       Manifest URL (required).
  -o, --output string                         Output file path. Prints to stdout when omitted.
      --strip-root                            Strip the top-level directory prefix from all filenames.
```

### SEE ALSO

* [docforge](docforge.md)	 - Forge a documentation bundle


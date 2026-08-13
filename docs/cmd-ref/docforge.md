## docforge

Forge a documentation bundle

```
docforge [flags]
```

### Options

```
      --add_dir_header                        If true, adds the file directory to the header of the log messages
      --aliases-enabled                       Propagate Hugo aliases from parent dir frontmatter to child files.
      --alsologtostderr                       log to standard error as well as files (no effect when -logtostderr=true)
      --cache-dir string                      Directory for the repository HTTP cache. (default "$HOME/.docforge")
      --clean-destination                     Remove the destination directory before writing. Ignored when --dry-run is set.
      --content-files-formats strings         File extensions to include in the output (e.g. .md,.html). When empty (the default), all file types are included.
  -d, --destination string                    Path to the directory where the forged documentation bundle will be written.
      --docsy-edit-this-page-enabled          Add Docsy 'Edit this page' frontmatter fields to output files.
      --document-workers int                  Number of parallel workers for document processing. (default 25)
      --download-workers int                  Number of workers downloading document resources in parallel. (default 10)
      --dry-run                               Print the resolved manifest node tree to stdout and skip cleaning the destination. Does NOT prevent files from being written — node processing and downloads still run.
      --fail-fast                             Stop immediately on the first processing error. Default (false) is fault-tolerant: log the error and continue with remaining files.
      --github-info-destination string        If set, write a .json sidecar per source file with GitHub commit metadata (author, contributors, lastmod, publishdate, SHA) into this subdirectory of --destination.
      --github-oauth-env-map stringToString   Map of GitHub host to environment variable name holding the access token (e.g. github.com=GITHUB_TOKEN). Required for authenticated requests. (default [])
  -h, --help                                  help for docforge
      --hugo                                  Enable Hugo-specific processing: rename section index files to _index.md and rewrite links to pretty-URL format. Pass --hugo=false for non-Hugo targets. (default true)
      --hugo-base-url string                  Rewrites the relative links of documentation files to root-relative where possible.
      --hugo-pretty-urls                      Rewrite .md links to directory-style pretty URLs (./sample.md -> ../sample/). Only active when --hugo=true. (default true)
      --hugo-section-files strings            Files with a name matching any entry in this list are renamed to _index.md in the output. Only active when --hugo=true. (default [readme.md,README.md])
      --hugo-structural-dirs strings          List of directories that are part of the hugo bundle structure and should not be included in the resolved links.
      --log_backtrace_at traceLocation        when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                        If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log_file string                       If non-empty, use this log file (no effect when -logtostderr=true)
      --log_file_max_size uint                Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                           log to standard error instead of files (default true)
  -f, --manifest string                       Path or URL of the documentation manifest file.
      --one_output                            If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --skip_headers                          If true, avoid header prefixes in the log messages
      --skip_log_headers                      If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity              logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true) (default 2)
  -v, --v Level                               number for the log level verbosity
      --vmodule moduleSpec                    comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [docforge gen-cmd-docs](docforge_gen-cmd-docs.md)	 - Generates commands reference documentation
* [docforge gen-toc](docforge_gen-toc.md)	 - Generate a navigation YAML from a Docforge manifest
* [docforge version](docforge_version.md)	 - Print the version


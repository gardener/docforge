## docforge

Forge a documentation bundle

```
docforge [flags]
```

### Flags

```
      --add_dir_header                        If true, adds the file directory to the header of the log messages
      --aliases-enabled                       Set this flag when you want to enable aliases for files.
      --alsologtostderr                       log to standard error as well as files (no effect when -logtostderr=true)
      --cache-dir string                      Cache directory, used for repository cache. (default "/root/.docforge")
      --content-files-formats strings         Supported content format extensions (example: .md)
  -d, --destination string                    Destination path.
      --docsy-edit-this-page-enabled          Set this flag when you are using edit this page in the docsy theme
      --document-workers int                  Number of parallel workers for document processing. (default 25)
      --download-workers int                  Number of workers downloading document resources in parallel. (default 10)
      --dry-run                               Runs the command end-to-end but instead of writing files, it will output the projected file/folder hierarchy to the standard output and statistics for the processing of each file.
      --fail-fast                             Fail-fast vs fault tolerant operation.
      --github-info-destination string        If specified, docforge will download also additional github info for the files from the documentation structure into this destination.
      --github-oauth-env-map stringToString   Map between GitHub instances and ENV var names that will be used for access tokens (default [])
  -h, --help                                  help for docforge
      --hosts-to-report strings               When a link has a host from the given array it will get reported
      --hugo                                  Build documentation bundle for hugo.
      --hugo-base-url string                  Rewrites the relative links of documentation files to root-relative where possible.
      --hugo-pretty-urls                      Build documentation bundle for hugo with pretty URLs (./sample.md -> ../sample). Only useful with --hugo=true (default true)
      --hugo-section-files strings            When building a Hugo-compliant documentation bundle, files with filename matching one form this list (in that order) will be renamed to _index.md. Only useful with --hugo=true (default [readme.md,README.md])
      --hugo-structural-dirs strings          List of directories that are part of the hugo bundle structure and should not be included in the resolved links.
      --log_backtrace_at traceLocation        when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                        If non-empty, write log files in this directory (no effect when -logtostderr=true)
      --log_file string                       If non-empty, use this log file (no effect when -logtostderr=true)
      --log_file_max_size uint                Defines the maximum size a log file can grow to (no effect when -logtostderr=true). Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                           log to standard error instead of files (default true)
  -f, --manifest string                       Manifest path.
      --one_output                            If true, only write logs to their native severity level (vs also writing to each lower severity level; no effect when -logtostderr=true)
      --persona-filter-enabled                Set this flag when you want to filter content by personas.
      --skip-link-validation                  Links validation will be skipped
      --skip_headers                          If true, avoid header prefixes in the log messages
      --skip_log_headers                      If true, avoid headers when opening log files (no effect when -logtostderr=true)
      --stderrthreshold severity              logs at or above this threshold go to stderr when writing to files and stderr (no effect when -logtostderr=true or -alsologtostderr=true) (default 2)
  -v, --v Level                               number for the log level verbosity
      --validation-workers int                Number of parallel workers to validate the markdown links (default 10)
      --vmodule moduleSpec                    comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [docforge completion](docforge_completion.md)	 - Generate completion script
* [docforge gen-cmd-docs](docforge_gen-cmd-docs.md)	 - Generates commands reference documentation
* [docforge version](docforge_version.md)	 - Print the version


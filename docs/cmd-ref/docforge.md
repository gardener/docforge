## docforge

Build documentation bundle

```
docforge [flags]
```

### Options

```
      --add_dir_header                              If true, adds the file directory to the header of the log messages
      --alsologtostderr                             log to standard error as well as files
  -d, --destination string                          Destination path.
      --download-workers int                        Number of workers downloading document resources in parallel. (default 10)
      --dry-run                                     Runs the command end-to-end but instead of writing files, it will output the projected file/folder hierarchy to the standard output and statistics for the processing of each file.
      --fail-fast                                   Fail-fast vs fault tolerant operation.
      --github-info-destination string              If specified, docforge will download also additional github info for the files from the documentation structure into this destination.
      --github-oauth-token string                   GitHub personal token authorizing read access from GitHub.com repositories. For authorization credentials for multiple GitHub instances, see --gtihub-oauth-token-map
      --github-oauth-token-map github-oauth-token   GitHub personal tokens authorizing read access from repositories per GitHub instance. Note that if the GitHub token is already provided by github-oauth-token it will be overrided by it. (default [])
  -h, --help                                        help for docforge
      --hugo                                        Build documentation bundle for hugo.
      --hugo-pretty-urls                            Build documentation bundle for hugo with pretty URLs (./sample.md -> ../sample). Only useful with --hugo=true (default true)
      --hugo-section-files strings                  When building a Hugo-compliant documentaton bundle, files with filename matching one form this list (in that order) will be renamed to _index.md. Only useful with --hugo=true (default [readme,read.me,index])
      --log_backtrace_at traceLocation              when logging hits line file:N, emit a stack trace (default :0)
      --log_dir string                              If non-empty, write log files in this directory
      --log_file string                             If non-empty, use this log file
      --log_file_max_size uint                      Defines the maximum size a log file can grow to. Unit is megabytes. If the value is 0, the maximum file size is unlimited. (default 1800)
      --logtostderr                                 log to standard error instead of files (default true)
  -f, --manifest string                             Manifest path.
      --max-workers int                             Maximum number of parallel workers. (default 25)
      --min-workers int                             Minimum number of parallel workers. (default 10)
      --resolve                                     Resolves the documentation structure and prints it to the standard output. The resolution expands nodeSelector constructs into node hierarchies.
      --resources-download-path string              Resources download path. (default "__resources")
      --rewrite-embedded-to-raw                     Rewrites absolute link destinations for embedded resources (images) to reference embedable media (e.g. for GitHub - reference to a 'raw' version of an image). (default true)
      --skip_headers                                If true, avoid header prefixes in the log messages
      --skip_log_headers                            If true, avoid headers when opening log files
      --stderrthreshold severity                    logs at or above this threshold go to stderr (default 2)
  -v, --v Level                                     number for the log level verbosity
      --vmodule moduleSpec                          comma-separated list of pattern=N settings for file-filtered logging
```

### SEE ALSO

* [docforge completion](docforge_completion.md)	 - Generate completion script
* [docforge gen-cmd-docs](docforge_gen-cmd-docs.md)	 - Generates commands reference documentation
* [docforge version](docforge_version.md)	 - Print the version


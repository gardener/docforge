## docforge gen-cmd-docs

Generates commands reference documentation

### Synopsis

Introspects all docforge commands and their flags and generates reference
documentation from them.

By default the output is Markdown (one file per subcommand), suitable for
publishing in a documentation portal. Pass --format man to generate Unix man
pages instead.

The destination directory is created if it does not exist. Existing files in
the destination are overwritten.

This command is intended for maintainers and CI pipelines — run it after
changing any flag or adding a new subcommand, then commit the updated files
under docs/cmd-ref/.

```
docforge gen-cmd-docs [flags]
```

### Options

```
  -d, --destination string   Path to directory where the documentation will be generated. If it does not exist, it will be created. Required flag.
  -f, --format md            Specifies the generated documentation format. Must be one of: md (for markdown) or `man` (for man pages). (default "md")
  -h, --help                 help for gen-cmd-docs
```

### SEE ALSO

* [docforge](docforge.md)	 - Forge a documentation bundle


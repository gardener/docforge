## docforge completion

Generate completion script

### Synopsis

To load completions:

**Bash**:

$ source <(docforge completion bash)

To load completions for each session, execute once:
- Linux:
  $ docforge completion bash > /etc/bash_completion.d/docforge
- MacOS:
  $ docforge completion bash > /usr/local/etc/bash_completion.d/docforge

**Zsh**:

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

$ echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions for each session, execute once:
$ docforge completion zsh > "${fpath[1]}/_docforge"

You will need to start a new shell for this setup to take effect.

**Fish**:

$ docforge completion fish | source

To load completions for each session, execute once:
$ docforge completion fish > ~/.config/fish/completions/docforge.fish


```
docforge completion [bash|zsh|fish|powershell]
```

### Options

```
  -h, --help   help for completion
```

### SEE ALSO

* [docforge](docforge.md)	 - Forge a documentation bundle


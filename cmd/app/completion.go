// SPDX-FileCopyrightText: 2020 SAP SE or an SAP affiliate company and Gardener contributors
//
// SPDX-License-Identifier: Apache-2.0

package app

import (
	"os"

	"github.com/spf13/cobra"
)

// completionCmd represents the completion command
var completionCmd = &cobra.Command{
	Use:   "completion [bash|zsh|fish|powershell]",
	Short: "Generate completion script",
	Long: `To load completions:

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
`,
	DisableFlagsInUseLine: true,
	ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
	Args:                  cobra.ExactValidArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		switch args[0] {
		case "bash":
			cmd.Root().GenBashCompletion(os.Stdout)
		case "zsh":
			cmd.Root().GenZshCompletion(os.Stdout)
		case "fish":
			cmd.Root().GenFishCompletion(os.Stdout, true)
		case "powershell":
			cmd.Root().GenPowerShellCompletion(os.Stdout)
		}
	},
}

func newCompletionCmd() *cobra.Command {
	return completionCmd
}

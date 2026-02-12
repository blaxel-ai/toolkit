package core

import (
	"bytes"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// bashCompletionShim provides a fallback implementation of _get_comp_words_by_ref
// for systems (like macOS with bash 3.2) that don't have bash-completion installed.
// Without this, cobra's generated bash completion fails with:
//   _get_comp_words_by_ref: command not found
const bashCompletionShim = `# Shim: provide _get_comp_words_by_ref if bash-completion is not installed.
# This allows completions to work on macOS default bash (3.2) without
# requiring 'brew install bash-completion'.
if ! type _get_comp_words_by_ref >/dev/null 2>&1; then
    _get_comp_words_by_ref() {
        local exclude cur_ words_ cword_
        if [ "$1" = "-n" ]; then
            exclude=$2
            shift 2
        fi
        __git_reassemble_comp_words_by_ref_n_exclude="$exclude"
        while [ $# -gt 0 ]; do
            case "$1" in
                cur)   cur="${COMP_WORDS[COMP_CWORD]}" ;;
                prev)  prev="${COMP_WORDS[COMP_CWORD-1]}" ;;
                words) eval words='("${COMP_WORDS[@]}")' ;;
                cword) cword=$COMP_CWORD ;;
            esac
            shift
        done
    }
fi

`

func init() {
	// Disable cobra's default completion command so we can provide our own
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	RegisterCommand("completion", func() *cobra.Command {
		return completionCmd()
	})
}

func completionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: `Generate shell completion scripts for bl.
To load completions:

Bash:
  eval "$(bl completion bash)"

  # To load completions for each session, execute once:
  # Linux:
  mkdir -p ~/.local/share/bash-completion/completions
  bl completion bash > ~/.local/share/bash-completion/completions/bl

  # macOS:
  bl completion bash > $(brew --prefix)/etc/bash_completion.d/bl

Zsh:
  eval "$(bl completion zsh)"

  # To load completions for each session, execute once:
  mkdir -p ~/.zsh/completions
  bl completion zsh > ~/.zsh/completions/_bl

Fish:
  bl completion fish | source

  # To load completions for each session, execute once:
  bl completion fish > ~/.config/fish/completions/bl.fish

PowerShell:
  bl completion powershell | Out-String | Invoke-Expression

  # To load completions for each session, execute once:
  bl completion powershell > bl.ps1
  # and source this file from your PowerShell profile.
`,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{"bash", "zsh", "fish", "powershell"},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "bash":
				return genBashCompletionWithShim(cmd.Root(), os.Stdout)
			case "zsh":
				return cmd.Root().GenZshCompletion(os.Stdout)
			case "fish":
				return cmd.Root().GenFishCompletion(os.Stdout, true)
			case "powershell":
				return cmd.Root().GenPowerShellCompletionWithDesc(os.Stdout)
			default:
				return fmt.Errorf("unsupported shell: %s", args[0])
			}
		},
	}

	return cmd
}

// genBashCompletionWithShim generates bash completion with a prepended shim
// that defines _get_comp_words_by_ref if the bash-completion package is not
// installed. This makes completions work on macOS default bash (3.2) without
// requiring external packages.
func genBashCompletionWithShim(root *cobra.Command, out *os.File) error {
	var buf bytes.Buffer
	if err := root.GenBashCompletionV2(&buf, true); err != nil {
		return err
	}

	// Prepend the shim before cobra's generated script
	if _, err := out.WriteString(bashCompletionShim); err != nil {
		return err
	}
	_, err := out.Write(buf.Bytes())
	return err
}

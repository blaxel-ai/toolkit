---
title: "bl completion"
slug: bl_completion
---
## bl completion

Generate shell completion scripts

### Synopsis

Generate shell completion scripts for bl.
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


```
bl completion [bash|zsh|fish|powershell]
```

### Options

```
  -h, --help   help for completion
```

### Options inherited from parent commands

```
  -o, --output string          Output format. One of: pretty,yaml,json,table
      --skip-version-warning   Skip version warning
  -u, --utc                    Enable UTC timezone
  -v, --verbose                Enable verbose output
  -w, --workspace string       Specify the workspace name
```

### SEE ALSO

* [bl](bl.md)	 - Blaxel CLI is a command line tool to interact with Blaxel APIs.


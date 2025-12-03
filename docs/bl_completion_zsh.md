---
title: "bl completion zsh"
slug: bl_completion_zsh
---
## bl completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(bl completion zsh)

To load completions for every new session, execute once:

#### Linux:

	bl completion zsh > "${fpath[1]}/_bl"

#### macOS:

	bl completion zsh > $(brew --prefix)/share/zsh/site-functions/_bl

You will need to start a new shell for this setup to take effect.


```
bl completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
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

* [bl completion](bl_completion.md)	 - Generate the autocompletion script for the specified shell


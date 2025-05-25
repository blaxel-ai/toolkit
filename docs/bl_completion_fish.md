---
title: "bl completion fish"
slug: bl_completion_fish
---
## bl completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	bl completion fish | source

To load completions for every new session, execute once:

	bl completion fish > ~/.config/fish/completions/bl.fish

You will need to start a new shell for this setup to take effect.


```
bl completion fish [flags]
```

### Options

```
  -h, --help              help for fish
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


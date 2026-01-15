---
title: "bl completion bash"
slug: bl_completion_bash
---
## bl completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

```bash
eval "$(bl completion bash)"
```

To load completions for every new session, execute once:

#### Linux:

```bash
bl completion bash > /etc/bash_completion.d/bl
```

#### macOS:

```bash
bl completion bash > $(brew --prefix)/etc/bash_completion.d/bl
```

You will need to start a new shell for this setup to take effect.


```
bl completion bash
```

### Options

```
  -h, --help              help for bash
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


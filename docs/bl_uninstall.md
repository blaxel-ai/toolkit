---
title: "bl uninstall"
slug: bl_uninstall
---
## bl uninstall

Remove the skills and MCP servers bl setup added to coding agents

### Synopsis

Undo bl setup: remove the Blaxel agent skills and MCP servers that bl setup,
bl skills and bl mcp added to the coding agents on this machine, and turn off
their automatic updates, so CLI upgrades and the install scripts do not add
them back.

Only what Blaxel wrote and nobody changed since is removed. MCP server entries
you configured or edited yourself, skills you edited, and skills managed by
another tool (a link in ~/.agents/skills) are kept, and listed. A configuration
file that setup created and that is empty afterwards is deleted.

In a terminal, uninstall lists what it removes and asks first; --yes removes it
without asking. Without a terminal and without --yes, it only lists it.
--dry-run lists it and changes nothing.

Uninstall never deletes the running bl. It ends with the commands that remove
the CLI for the way it was installed (Homebrew, the install script, or
install.ps1 on Windows). If you keep bl, bl setup adds the skills and MCP
servers back and turns their automatic updates on again.

```
bl uninstall [flags]
```

### Examples

```
  # See what would be removed, without changing anything
  bl uninstall --dry-run

  # Remove the skills and MCP servers without asking
  bl uninstall --yes

  # Remove only the MCP servers
  bl uninstall --skip-skills
```

### Options

```
      --dry-run       List what would be removed and change nothing
  -h, --help          help for uninstall
      --skip-mcp      Keep the Blaxel MCP servers
      --skip-skills   Keep the Blaxel agent skills and automatic skills updates
  -y, --yes           Remove without asking
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

* [bl](bl.md)	 - Blaxel CLI - manage and deploy AI agents, sandboxes, and resources


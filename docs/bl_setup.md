---
title: "bl setup"
slug: bl_setup
---
## bl setup

Set up Blaxel for your coding agents and log in

### Synopsis

Set up everything Blaxel needs on this machine, then log in.

For the coding agents found on this machine (Claude Code, Codex, Cursor, ...),
setup installs the Blaxel agent skills and adds two MCP servers: blaxel, to
manage your workspace resources, and blaxel-docs, to search the Blaxel
documentation. It then logs you in to Blaxel in your browser. The agents use
that login through bl mcp, so they need no sign-in of their own.

In a terminal, setup shows everything it found, selected, and installs it
when you press Enter; --yes installs it without showing the plan. Without a
terminal, setup installs the same defaults and skips the browser login, so
run bl login afterwards. Anonymous error reports are on unless you turn them
off (or set DO_NOT_TRACK=1).

Setup only adds what is missing, and it is safe to run again after
installing another agent. MCP server entries you configured yourself are left
unchanged; the hosted blaxel server added by earlier versions is switched to
bl mcp. A blaxel server that the Blaxel plugin provides is left to the plugin.

```
bl setup [flags]
```

### Examples

```
  # See what setup found, then press Enter to install
  bl setup

  # Install the defaults without showing the plan
  bl setup --yes

  # Set up only Claude Code and Codex, without logging in
  bl setup --agent claude-code,codex --skip-login
```

### Options

```
      --agent strings   Coding agents to set up instead of the detected ones (for example claude-code,codex)
  -h, --help            help for setup
      --skip-login      Do not log in to Blaxel
      --skip-mcp        Do not add the Blaxel MCP servers
      --skip-skills     Do not install the Blaxel agent skills
  -y, --yes             Install the defaults without showing the plan
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


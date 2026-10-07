---
title: "bl setup"
slug: bl_setup
---
## bl setup

Set up Blaxel for your coding agents and log in

### Synopsis

Set up everything Blaxel needs on this machine, then log in.

For the coding agents found on this machine (Claude Code, Codex, Cursor, ...),
setup installs the Blaxel agent skills and adds two hosted MCP servers:
blaxel, to manage your workspace resources, and blaxel-docs, to search the
Blaxel documentation. It then logs you in to Blaxel in your browser.

In a terminal, setup shows everything it found, selected, and installs it
when you press Enter; --yes installs it without showing the plan. Without a
terminal, setup installs the same defaults and skips the browser login, so
run bl login afterwards. The usage and error reports toggle controls the saved
tracking preference. New setup plans keep reports selected by default; an
existing saved choice is retained. Anonymous usage capture is disabled in CI,
by any nonempty DO_NOT_TRACK, or by BL_INSTALL_TRACKING=false. Error reports
keep the SDK's existing DO_NOT_TRACK semantics.

Events, properties and opt-outs: https://docs.blaxel.ai/Security/Data-collection-and-privacy

Setup only adds what is missing. Existing MCP server entries are left
unchanged, and it is safe to run again after installing another agent.

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


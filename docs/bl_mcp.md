---
title: "bl mcp"
slug: bl_mcp
---
## bl mcp

Serve the Blaxel MCP server to a coding agent, signed in with your bl login

### Synopsis

Serve the Blaxel MCP server to a coding agent over stdio, signed in with your bl login.

Coding agents start this command themselves: bl setup adds it to their MCP
configuration. Requests go to the hosted Blaxel MCP server, so agents need no
sign-in of their own and no token is stored in their configuration.

The workspace is the current one when the agent starts (see bl workspaces),
--workspace or BL_WORKSPACE. It stays the same for the agent's session; tools
take a workspace argument to use another workspace you belong to. BL_API_KEY
and BL_CLIENT_CREDENTIALS work as for every other bl command.

Without a login, the agent gets one tool, blaxel_login, which opens the Blaxel
login page in your browser. Once you confirm, the Blaxel tools appear in the
agent; you can also run bl login in a terminal.

```
bl mcp [flags]
```

### Examples

```
  # Claude Code
  claude mcp add --scope user blaxel -- bl mcp

  # Any agent that starts local (stdio) MCP servers
  {"mcpServers": {"blaxel": {"command": "bl", "args": ["mcp"]}}}
```

### Options

```
  -h, --help   help for mcp
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


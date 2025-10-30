---
title: "bl create-mcp-server"
slug: bl_create-mcp-server
---
## bl create-mcp-server

Create a new blaxel mcp server

### Synopsis

Create a new Model Context Protocol (MCP) server.

MCP servers extend agent capabilities by providing custom tools, data sources,
and integrations. They expose functions that agents can call to perform actions
or retrieve information.

Common use cases:
- Custom API integrations (GitHub, Jira, CRM systems)
- Database connectors and query tools
- File system operations
- Data transformation and analysis tools
- External service orchestration

MCP is a standard protocol for agent-tool communication. Your MCP server can
be used by any agent that supports the protocol, making it reusable across
multiple agents and projects.

The command scaffolds a complete MCP server project with:
- Server setup and configuration
- Example tool implementations
- Protocol handling code
- Testing utilities

After creation: cd into the directory, implement your tools, test with
'bl serve --hotreload', then 'bl deploy' to make available to agents.

Note: Prefer using 'bl new mcp' which provides a unified creation experience.

```
bl create-mcp-server [directory] [flags]
```

### Examples

```
  # Interactive creation
  bl create-mcp-server my-tools

  # With specific template
  bl create-mcp-server my-tools --template template-mcp-hello-world-py

  # Non-interactive with defaults
  bl create-mcp-server my-tools -y

  # Recommended: Use unified 'new' command instead
  bl new mcp my-tools
```

### Options

```
  -h, --help              help for create-mcp-server
  -t, --template string   Template to use for the mcp server (skips interactive prompt)
  -y, --yes               Skip interactive prompts and use defaults
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


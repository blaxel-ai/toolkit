---
title: "bl get mcp-hub"
slug: bl_get_mcp-hub
---
## bl get mcp-hub

List pre-built MCP servers available in the Blaxel Hub

### Synopsis

List pre-built MCP servers from the Blaxel Hub.

These provide ready-to-use tool integrations (e.g. GitHub, Slack,
databases). Connect one to your agent by creating an integration
connection with 'bl apply -f connection.yaml':

  apiVersion: blaxel/v1alpha1
  kind: IntegrationConnection
  metadata:
    name: my-github
  spec:
    integration: <integration-from-hub>

Output formats:
  -o json   Machine-readable JSON array
  -o yaml   YAML output
  default   Table with NAME, INTEGRATION, DESCRIPTION columns

```
bl get mcp-hub [flags]
```

### Examples

```
  # List all available MCP hub servers
  bl get mcp-hub

  # List as JSON (for automation/agents)
  bl get mcp-hub -o json

  # List as YAML
  bl get mcp-hub -o yaml
```

### Options

```
  -h, --help   help for mcp-hub
```

### Options inherited from parent commands

```
  -o, --output string          Output format. One of: pretty,yaml,json,table
      --skip-version-warning   Skip version warning
  -u, --utc                    Enable UTC timezone
  -v, --verbose                Enable verbose output
      --watch                  After listing/getting the requested object, watch for changes.
  -w, --workspace string       Specify the workspace name
```

### SEE ALSO

* [bl get](bl_get.md)	 - List or retrieve Blaxel resources in your workspace


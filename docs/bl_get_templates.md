---
title: "bl get templates"
slug: bl_get_templates
---
## bl get templates

List available project templates

### Synopsis

List available templates that can be used with 'bl new'.

Templates are grouped by type (agent, mcp, sandbox, job, volume-template).
Use an optional type argument to filter results.

Output formats:
  -o json   Machine-readable JSON array
  -o yaml   YAML output
  default   Table with NAME, TYPE, LANGUAGE, DESCRIPTION columns

```
bl get templates [type] [flags]
```

### Examples

```
  # List all templates
  bl get templates

  # List agent templates only
  bl get templates agent

  # List templates as JSON
  bl get templates -o json

  # List MCP templates
  bl get templates mcp
```

### Options

```
  -h, --help   help for templates
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


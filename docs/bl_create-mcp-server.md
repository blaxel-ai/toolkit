---
title: "bl create-mcp-server"
slug: bl_create-mcp-server
---
## bl create-mcp-server

Create a new blaxel mcp server

### Synopsis

Create a new blaxel mcp server

```
bl create-mcp-server directory [flags]
```

### Examples

```

bl create-mcp-server my-mcp-server
bl create-mcp-server my-mcp-server --template template-mcp-hello-world-py
bl create-mcp-server my-mcp-server --template template-mcp-hello-world-py -y
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


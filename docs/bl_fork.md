---
title: "bl fork"
slug: bl_fork
---
## bl fork

Fork a sandbox into a new sandbox or application

### Synopsis

Create a new sandbox or application by forking an existing sandbox.

Arguments use the type/name format:
  sbx/name or sandbox/name  — sandbox resource
  app/name or application/name — application resource

If the source has no type prefix, it defaults to sandbox.

```
bl fork <source> <target> [flags]
```

### Examples

```
  # Fork a sandbox into a new sandbox
  bl fork sbx/my-sandbox sbx/my-sandbox-fork

  # Fork a sandbox into an application
  bl fork sbx/my-sandbox app/my-app

  # Fork with canary traffic and port
  bl fork sbx/my-sandbox app/my-app --traffic 20 --port 8080

  # Fork with custom memory
  bl fork sbx/my-sandbox sbx/my-fork --memory 4096

  # Short form (source defaults to sandbox)
  bl fork my-sandbox app/my-app
```

### Options

```
  -h, --help          help for fork
      --memory int    Memory in MB (inherits from source if not specified)
      --port int      Port to expose
      --traffic int   Canary traffic percentage for the new revision
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


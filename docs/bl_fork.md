---
title: "bl fork"
slug: bl_fork
---
## bl fork

Fork a sandbox into a new sandbox or application

### Synopsis

Create a new sandbox or application by forking an existing sandbox.

```
bl fork <source-sandbox> <target-name> [flags]
```

### Examples

```
  # Fork a sandbox into a new sandbox
  bl fork my-sandbox my-sandbox-fork

  # Fork a sandbox into an application
  bl fork my-sandbox my-app --type application

  # Fork with canary traffic percentage and port
  bl fork my-sandbox my-app --type application --traffic 20 --port 8080

  # Fork with custom memory
  bl fork my-sandbox my-sandbox-fork --memory 4096
```

### Options

```
  -h, --help          help for fork
      --memory int    Memory in MB (inherits from source if not specified)
      --port int      Port to expose
      --traffic int   Canary traffic percentage for the new revision
      --type string   Target resource type (sandbox or application) (default "sandbox")
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


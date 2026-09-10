---
title: "bl fork"
slug: bl_fork
---
## bl fork

Fork a sandbox into a new sandbox or application

### Synopsis

Create a new sandbox or application by forking an existing sandbox.

Forking into a sandbox copies the source's live state — its filesystem,
running processes and memory — straight into the new sandbox. No snapshot is
created, and none is left behind on the source. Forking a running source
pauses it for the moment its memory is captured.

Pass --snapshot-id to fork from a snapshot you took earlier instead of from
the source's live state. Forking into an application always goes through a
snapshot, which the application's revision references so you can re-activate
that revision later.

A fork inherits the source's image, memory and environment; only its
environment can be changed, and only through the API.

Arguments use the type/name format:
  sbx/name or sandbox/name  — sandbox resource
  app/name or application/name — application resource

If the source has no type prefix, it defaults to sandbox.

```
bl fork <source> <target> [flags]
```

### Examples

```
  # Fork a sandbox into a new sandbox, copying its live state
  bl fork sbx/my-sandbox sbx/my-sandbox-fork

  # Fork from a snapshot taken earlier instead of the live state
  bl fork sbx/my-sandbox sbx/my-sandbox-fork --snapshot-id snap_abc123

  # Fork a sandbox into an application
  bl fork sbx/my-sandbox app/my-app

  # Fork with canary traffic and port
  bl fork sbx/my-sandbox app/my-app --traffic 20 --port 8080

  # Short form (source defaults to sandbox)
  bl fork my-sandbox app/my-app
```

### Options

```
  -h, --help                 help for fork
      --memory int           Deprecated: a fork always inherits the source sandbox's memory
      --port int             Port to expose
      --snapshot-id string   Fork from this snapshot instead of the source's live state
      --traffic int          Canary traffic percentage for the new revision
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


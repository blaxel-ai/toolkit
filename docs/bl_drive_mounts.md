---
title: "bl drive mounts"
slug: bl_drive_mounts
---
## bl drive mounts

List mounted drives in a sandbox

### Synopsis

List all currently mounted drives in a sandbox environment.

```
bl drive mounts [flags]
```

### Examples

```
  # List all mounted drives
  bl drive mounts --sandbox my-sandbox
```

### Options

```
  -h, --help             help for mounts
      --sandbox string   Name of the sandbox
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

* [bl drive](bl_drive.md)	 - Manage drives and drive mounts on sandboxes


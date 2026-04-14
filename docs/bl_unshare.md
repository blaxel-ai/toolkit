---
title: "bl unshare"
slug: bl_unshare
---
## bl unshare

Unshare a resource from another workspace

### Synopsis

Remove shared Blaxel resources from other workspaces.
Currently supports unsharing container images.

### Examples

```
  # Unshare an image from another workspace
  bl unshare image agent/my-agent --workspace other-workspace
```

### Options

```
  -h, --help   help for unshare
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
* [bl unshare image](bl_unshare_image.md)	 - Unshare an image from another workspace


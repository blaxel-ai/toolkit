---
title: "bl share"
slug: bl_share
---
## bl share

Share a resource with another workspace

### Synopsis

Share Blaxel resources with other workspaces in your account.
Currently supports sharing container images.

### Examples

```
  # Share an image with another workspace
  bl share image agent/my-agent --workspace other-workspace
```

### Options

```
  -h, --help   help for share
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
* [bl share image](bl_share_image.md)	 - Share an image with another workspace


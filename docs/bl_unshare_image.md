---
title: "bl unshare image"
slug: bl_unshare_image
---
## bl unshare image

Unshare an image from another workspace

### Synopsis

Remove a shared image from another workspace.
This removes the metadata copy from the target workspace.
The original image in the source workspace is not affected.

The image reference format is: resourceType/imageName
- resourceType: Type of resource (e.g., agent, function, job, sandbox)
- imageName: The name of the image

```
bl unshare image resourceType/imageName [flags]
```

### Examples

```
  # Unshare an image from another workspace
  bl unshare image agent/my-agent --workspace other-workspace
```

### Options

```
  -h, --help               help for image
  -w, --workspace string   Target workspace to unshare from (required)
```

### Options inherited from parent commands

```
  -o, --output string          Output format. One of: pretty,yaml,json,table
      --skip-version-warning   Skip version warning
  -u, --utc                    Enable UTC timezone
  -v, --verbose                Enable verbose output
```

### SEE ALSO

* [bl unshare](bl_unshare.md)	 - Unshare a resource from another workspace


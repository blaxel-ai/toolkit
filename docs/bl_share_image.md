---
title: "bl share image"
slug: bl_share_image
---
## bl share image

Share an image with another workspace

### Synopsis

Share a container image with another workspace in your account.
Only the metadata is copied — the image data stays in the source workspace.

The image reference format is: resourceType/imageName
- resourceType: Type of resource (e.g., agent, function, job, sandbox)
- imageName: The name of the image

```
bl share image resourceType/imageName [flags]
```

### Examples

```
  # Share an image with another workspace
  bl share image agent/my-agent --workspace other-workspace
```

### Options

```
  -h, --help               help for image
  -w, --workspace string   Target workspace to share with (required)
```

### Options inherited from parent commands

```
  -o, --output string          Output format. One of: pretty,yaml,json,table
      --skip-version-warning   Skip version warning
  -u, --utc                    Enable UTC timezone
  -v, --verbose                Enable verbose output
```

### SEE ALSO

* [bl share](bl_share.md)	 - Share a resource with another workspace


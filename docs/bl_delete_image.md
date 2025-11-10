---
title: "bl delete image"
slug: bl_delete_image
---
## bl delete image

Delete images or image tags

### Synopsis

Delete container images or specific tags.

Usage patterns:
  bl delete image agent/my-image          Delete image with all its tags
  bl delete image agent/my-image:v1.0     Delete only the specified tag

The image reference format is: resourceType/imageName[:tag]
- resourceType: The type of resource (e.g., agent, function, job)
- imageName: The name of the image
- tag: Optional tag to delete only that specific version

WARNING: Deleting an image without specifying a tag will remove ALL tags.

```
bl delete image resourceType/imageName[:tag] [resourceType/imageName[:tag]...] [flags]
```

### Examples

```
  # Delete an entire image (all tags)
  bl delete image agent/my-agent
  
  # Delete only a specific tag
  bl delete image agent/my-agent:v1.0
  
  # Delete multiple images/tags
  bl delete image agent/img1:v1 agent/img2:v2
```

### Options

```
  -h, --help   help for image
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

* [bl delete](bl_delete.md)	 - Delete a resource


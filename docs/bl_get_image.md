---
title: "bl get image"
slug: bl_get_image
---
## bl get image

Get image information

### Synopsis

Get information about container images.

Usage patterns:
  bl get images                          List all images (without tags)
  bl get image agent/my-image            Get image details for a specific resource type
  bl get image agent/my-image:v1.0       Get specific tag information
  bl get image sandbox/my-image --latest Get the latest tag reference for an image

The image reference format is: resourceType/imageName[:tag]
- resourceType: Type of resource (e.g., agent, function, job, sandbox)
- imageName: The name of the image
- tag: Optional tag to filter for a specific version

The --latest flag returns the image reference with the most recent tag,
formatted as resourceType/imageName:tag. This is useful for scripting
and for retrieving the IMAGE_ID to use when creating sandboxes from templates.

```
bl get image [resourceType/imageName[:tag]] [flags]
```

### Examples

```
  # List all images
  bl get images

  # Get all tags for a specific image
  bl get image agent/my-agent

  # Get a specific tag
  bl get image agent/my-agent:latest

  # Get the latest tag reference (useful for sandbox templates)
  bl get image sandbox/mytemplate --latest

  # Use different output formats
  bl get images -o json
  bl get image agent/my-agent -o pretty
```

### Options

```
  -h, --help     help for image
      --latest   Return only the most recent tag reference (e.g., sandbox/mytemplate:tag)
```

### Options inherited from parent commands

```
  -o, --output string          Output format. One of: pretty,yaml,json,table
      --skip-version-warning   Skip version warning
  -u, --utc                    Enable UTC timezone
  -v, --verbose                Enable verbose output
      --watch                  After listing/getting the requested object, watch for changes.
  -w, --workspace string       Specify the workspace name
```

### SEE ALSO

* [bl get](bl_get.md)	 - Get a resource


---
title: "bl push"
slug: bl_push
---
## bl push

Build and push a container image to the Blaxel registry

### Synopsis

Build and push a container image to the Blaxel registry without creating a deployment.

This command packages your code, uploads it, and builds a container image that
is stored in the workspace registry. Unlike 'bl deploy', this command does NOT
create or update any resource (agent, function, sandbox, or job).

The process includes:
1. Reading configuration from blaxel.toml
2. Packaging source code (respects .blaxelignore)
3. Uploading to Blaxel's build system via presigned URL
4. Building container image
5. Streaming build logs until the image is ready

You must run this command from a directory containing a blaxel.toml file.

```
bl push [flags]
```

### Examples

```
  # Push current directory as an image
  bl push

  # Push with a custom name
  bl push --name my-image

  # Push a specific subdirectory
  bl push -d ./packages/my-agent

  # Push specifying a resource type
  bl push --type agent

  # Push with a longer timeout for large images
  bl push --timeout 30m
```

### Options

```
  -d, --directory string            Source directory path
      --docker-config string        Path to a Docker config.json file with registry credentials
  -h, --help                        help for push
  -n, --name string                 Name for the image (defaults to directory name)
  -c, --registry-cred stringArray   Registry credentials (format: registry=username:password, repeatable)
      --timeout string              Timeout for build log monitoring (e.g. 30m, 1h). Defaults to 15m
  -t, --type string                 Resource type (agent, function, sandbox, job). Defaults to blaxel.toml type; required if not set
  -y, --yes                         Skip interactive mode
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


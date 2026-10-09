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

If the blaxel.toml contains an 'image' field pointing to a registry image
(e.g. docker.io/myorg/myapp:latest), the platform will pull the image and
transform it for the target runtime via metamorph. If the same image was
already built, the build is triggered again by default. Use --skip-build to
skip the build if the image was already built.

Use --memory and --volume (MiB) to size the temporary build or import worker.
These flags override [build].memoryMb and [build].volumeMb in blaxel.toml.
Omitting both uses project settings or platform defaults; --volume 0 requests
memory-backed scratch. These settings do not change runtime resources.

Use --dockerfile to select a Dockerfile, or set build.dockerfile in blaxel.toml
(the flag wins). Paths are relative to the source directory (-d, or the current
directory) and must stay inside it. The build context is unchanged, so COPY paths
are unaffected. A custom Dockerfile needs a source build, not an image import.
It and its adjacent .dockerignore are uploaded even if .blaxelignore excludes them.

Use --config to read a config file other than blaxel.toml, such as
blaxel-v2.toml. The path is relative to the source directory (-d, or the current
directory), and the file must exist and parse. A source build uploads it as
blaxel.toml, so the build reads it too.

For private registries, supply credentials via --registry-cred or --docker-config.

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

  # Build with a custom Dockerfile
  bl push --dockerfile blaxel.Dockerfile

  # Build a variant from its own config file (which can set build.dockerfile)
  bl push --config blaxel-v2.toml

  # Push from a private registry (credentials for blaxel.toml image field)
  bl push --registry-cred ghcr.io=user:token

  # Skip rebuild if image was already built
  bl push --skip-build

  # Import a registry image with 16 GiB memory and 32 GiB scratch disk
  bl push --image docker.io/myorg/myapp:latest --type sandbox --memory 16384 --volume 32768

  # Push with a longer timeout for large images
  bl push --timeout 30m
```

### Options

```
      --build-env-file string       Path to a build env file with Docker build args (default: auto-detect .env.build)
      --config string               Config file relative to the source directory (default blaxel.toml)
  -d, --directory string            Source directory path
      --docker-config string        Path to a Docker config.json file with registry credentials
      --dockerfile string           Dockerfile path relative to the source directory (overrides build.dockerfile in blaxel.toml)
  -h, --help                        help for push
      --image string                Existing registry image to import; overrides blaxel.toml image
      --memory int                  Build or import worker memory in MiB (1-32768); overrides [build].memoryMb
  -n, --name string                 Name for the image (defaults to directory name)
  -c, --registry-cred stringArray   Registry credentials (format: registry=username:password, repeatable)
      --skip-build                  Skip the image build step (use existing built image if available)
      --timeout string              Timeout for build log monitoring (e.g. 30m, 1h). Defaults to 1h
  -t, --type string                 Resource type (agent, function, sandbox, job). Defaults to blaxel.toml type; required if not set
      --volume int                  Build or import scratch disk in MiB (0-131072); 0 uses memory-backed scratch; overrides [build].volumeMb
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


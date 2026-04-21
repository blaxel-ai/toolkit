---
title: "bl get sandbox-hub"
slug: bl_get_sandbox-hub
---
## bl get sandbox-hub

List pre-built sandbox images available in the Blaxel Hub

### Synopsis

List pre-built sandbox images from the Blaxel Hub.

Each image comes with pre-installed tools, runtimes, and configurations.
Use the 'image' field value in your sandbox YAML spec when deploying
with 'bl apply -f sandbox.yaml':

```yaml
  apiVersion: blaxel/v1alpha1
  kind: Sandbox
  metadata:
    name: my-sandbox
  spec:
    image: <image-from-hub>
```

Output formats:
  -o json   Machine-readable JSON array
  -o yaml   YAML output
  default   Table with NAME, IMAGE, MEMORY, DESCRIPTION columns

```
bl get sandbox-hub [flags]
```

### Examples

```
  # List all available sandbox hub images
  bl get sandbox-hub

  # List as JSON (for automation/agents)
  bl get sandbox-hub -o json

  # List as YAML
  bl get sandbox-hub -o yaml
```

### Options

```
  -h, --help   help for sandbox-hub
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

* [bl get](bl_get.md)	 - List or retrieve Blaxel resources in your workspace


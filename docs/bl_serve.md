---
title: "bl serve"
slug: bl_serve
---
## bl serve

Serve a blaxel project

### Synopsis

Serve a blaxel project

```
bl serve [flags]
```

### Examples

```
  bl serve --remote --hotreload --port 1338
```

### Options

```
  -h, --help            help for serve
  -H, --host string     Bind socket to this port. If 0, an available port will be picked (default "0.0.0.0")
      --hotreload       Watch for changes in the project
  -m, --module string   Module to serve, can be an agent or a function
  -p, --port int        Bind socket to this host (default 1338)
  -r, --remote          Serve the project remotely. It will use functions deployed on blaxel cloud
```

### Options inherited from parent commands

```
  -o, --output string      Output format. One of: pretty,yaml,json,table
  -u, --utc                Enable UTC timezone
  -v, --verbose            Enable verbose output
  -w, --workspace string   Specify the workspace name
```

### SEE ALSO

* [bl](bl.md)	 - Blaxel CLI is a command line tool to interact with Blaxel APIs.


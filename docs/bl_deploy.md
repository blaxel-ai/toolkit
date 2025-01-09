---
title: "bl deploy"
slug: bl_deploy
---
## bl deploy

Deploy a beamlit agent app

### Synopsis

Deploy a beamlit agent app, you must be in a beamlit agent app directory.

```
bl deploy [flags]
```

### Examples

```
bl deploy
```

### Options

```
  -d, --directory string   Directory to deploy, defaults to current directory (default "src")
  -h, --help               help for deploy
  -m, --module string      Module to serve, can be an agent or a function (default "agent.main")
```

### Options inherited from parent commands

```
  -e, --env string         Environment. One of: development,production
  -o, --output string      Output format. One of: pretty,yaml,json,table
  -v, --verbose            Enable verbose output
  -w, --workspace string   Specify the workspace name
```

### SEE ALSO

* [bl](bl.md)	 - Beamlit CLI is a command line tool to interact with Beamlit APIs.


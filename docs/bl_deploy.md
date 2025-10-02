---
title: "bl deploy"
slug: bl_deploy
---
## bl deploy

Deploy on blaxel

### Synopsis

Deploy agent, mcp or job on blaxel, you must be in a blaxel directory.

```
bl deploy [flags]
```

### Examples

```
bl deploy
```

### Options

```
  -d, --directory string   Deployment app path, can be a sub directory
      --dryrun             Dry run the deployment
  -e, --env-file strings   Environment file to load (default [.env])
  -h, --help               help for deploy
  -n, --name string        Optional name for the deployment
  -r, --recursive          Deploy recursively (default true)
  -s, --secrets strings    Secrets to deploy
      --skip-build         Skip the build step
  -y, --yes                Skip interactive mode
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

* [bl](bl.md)	 - Blaxel CLI is a command line tool to interact with Blaxel APIs.


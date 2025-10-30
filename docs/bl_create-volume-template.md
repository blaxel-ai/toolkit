---
title: "bl create-volume-template"
slug: bl_create-volume-template
---
## bl create-volume-template

Create a new blaxel volume template

### Synopsis

Create a new blaxel volume template

```
bl create-volume-template [directory] [flags]
```

### Examples

```

bl create-volume-template my-volume-template
bl create-volume-template my-volume-template --template template-volume-template-py
bl create-volume-template my-volume-template --template template-volume-template-py -y
```

### Options

```
  -h, --help              help for create-volume-template
  -t, --template string   Template to use for the volume template (skips interactive prompt)
  -y, --yes               Skip interactive prompts and use defaults
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


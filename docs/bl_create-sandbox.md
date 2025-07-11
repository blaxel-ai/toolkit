---
title: "bl create-sandbox"
slug: bl_create-sandbox
---
## bl create-sandbox

Create a new blaxel sandbox

### Synopsis

Create a new blaxel sandbox

```
bl create-sandbox directory [flags]
```

### Examples

```

bl create-sandbox my-sandbox
bl create-sandbox my-sandbox --template template-sandbox-ts
bl create-sandbox my-sandbox --template template-sandbox-ts -y
```

### Options

```
  -h, --help              help for create-sandbox
  -t, --template string   Template to use for the sandbox (skips interactive prompt)
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


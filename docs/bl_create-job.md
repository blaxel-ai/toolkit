---
title: "bl create-job"
slug: bl_create-job
---
## bl create-job

Create a new blaxel job

### Synopsis

Create a new blaxel job

```
bl create-job directory [flags]
```

### Examples

```

bl create-job my-job
bl create-job my-job --template template-jobs-ts
bl create-job my-job --template template-jobs-ts -y
```

### Options

```
  -h, --help              help for create-job
  -t, --template string   Template to use for the job (skips interactive prompt)
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


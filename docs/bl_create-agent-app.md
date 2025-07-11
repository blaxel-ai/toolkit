---
title: "bl create-agent-app"
slug: bl_create-agent-app
---
## bl create-agent-app

Create a new blaxel agent app

### Synopsis

Create a new blaxel agent app

```
bl create-agent-app directory [flags]
```

### Examples

```

bl create-agent-app my-agent-app
bl create-agent-app my-agent-app --template template-google-adk-py
bl create-agent-app my-agent-app --template template-google-adk-py -y
```

### Options

```
  -h, --help              help for create-agent-app
  -t, --template string   Template to use for the agent app (skips interactive prompt)
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


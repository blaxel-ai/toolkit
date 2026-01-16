---
title: "bl connect sandbox"
slug: bl_connect_sandbox
---
## bl connect sandbox

Connect to a sandbox environment

### Synopsis

Connect to a sandbox environment with an interactive terminal session.

This command opens a direct terminal connection to your sandbox, similar to SSH.
The terminal supports full ANSI colors, cursor movement, and interactive applications.

Press Ctrl+D to disconnect from the sandbox.

Examples:
  bl connect sandbox my-sandbox
  bl connect sb my-sandbox
  bl connect sbx production-env

```
bl connect sandbox [sandbox-name] [flags]
```

### Options

```
  -h, --help   help for sandbox
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

* [bl connect](bl_connect.md)	 - Connect into your sandbox resources


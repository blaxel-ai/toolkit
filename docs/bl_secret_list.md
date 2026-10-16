---
title: "bl secret list"
slug: bl_secret_list
---
## bl secret list

List workspace secrets

### Synopsis

List the workspace secrets with their destinations and dates. Values are never returned.

```
bl secret list [flags]
```

### Examples

```
  bl secret list
  bl secret list -o json
```

### Options

```
  -h, --help   help for list
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

* [bl secret](bl_secret.md)	 - Manage workspace secrets injected by the sandbox proxy


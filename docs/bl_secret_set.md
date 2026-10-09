---
title: "bl secret set"
slug: bl_secret_set
---
## bl secret set

Create or update a workspace secret

### Synopsis

Create or update a workspace secret.

The value comes from exactly one of --value, --from-env, --from-file, or stdin
when none of the flags is given. Prefer --from-env, --from-file or stdin over
--value so the secret does not end up in your shell history.

```
bl secret set NAME [flags]
```

### Options

```
      --from-env string    Read the value from this environment variable
      --from-file string   Read the value from this file
  -h, --help               help for set
      --value string       Secret value (visible in shell history; prefer --from-env, --from-file or stdin)
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

* [bl secret](bl_secret.md)	 - Manage workspace secrets


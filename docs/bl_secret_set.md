---
title: "bl secret set"
slug: bl_secret_set
---
## bl secret set

Create or rotate a workspace secret

### Synopsis

Create a workspace secret, or rotate the value of an existing one.

The value comes from --value, or from stdin when --value is omitted (a
trailing newline is dropped). At least one --destination is required: a host
such as api.stripe.com, optionally followed by a path glob such as
api.example.com/v1/*. The secret is only injected toward those destinations.

```
bl secret set <name> [flags]
```

### Examples

```
  bl secret set stripe-key --value sk_live_... --destination api.stripe.com
  echo -n "$TOKEN" | bl secret set github-token -d api.github.com -d api.github.com/repos/*
  bl secret set multi -d "api.a.com,api.b.com/v2/*" --value ...
```

### Options

```
  -d, --destination stringArray   Destination the secret may be sent to: host or host/path glob (repeatable, or comma-separated)
  -h, --help                      help for set
      --value string              Secret value (read from stdin when omitted)
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


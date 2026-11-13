---
title: "bl secret"
slug: bl_secret
---
## bl secret

Manage workspace secrets injected by the sandbox proxy

### Synopsis

Manage workspace secrets.

A workspace secret is referenced from a sandbox proxy routing rule as
{{SECRET:name}} and injected by the proxy, so sandbox code never sees the
value. Each secret lists the destinations it may be sent to (a host,
optionally followed by a path glob, same syntax as routing destinations);
it is never substituted toward any other destination. Values are write-only:
they can be set or rotated but never read back.

### Examples

```
  # List the workspace secrets (names and destinations, never values)
  bl secret list

  # Create or rotate a secret scoped to one host
  bl secret set stripe-key --value sk_live_... --destination api.stripe.com

  # Read the value from stdin, scope to several destinations
  cat key.txt | bl secret set github-token -d api.github.com -d "uploads.github.com/repos/*"

  # Delete a secret
  bl secret delete stripe-key
```

### Options

```
  -h, --help   help for secret
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

* [bl](bl.md)	 - Blaxel CLI - manage and deploy AI agents, sandboxes, and resources
* [bl secret delete](bl_secret_delete.md)	 - Delete a workspace secret
* [bl secret list](bl_secret_list.md)	 - List workspace secrets
* [bl secret set](bl_secret_set.md)	 - Create or rotate a workspace secret


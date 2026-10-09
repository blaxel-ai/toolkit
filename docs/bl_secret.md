---
title: "bl secret"
slug: bl_secret
---
## bl secret

Manage workspace secrets

### Synopsis

Manage workspace secrets.

A workspace secret is a named value that is encrypted at rest and can never be
read back once set. Reference it from a sandbox's agent-proxy routing rules with
`{{SECRET:name}}`: the proxy injects the latest value into outbound requests at
runtime, so the credential never reaches the sandbox itself.

Setting a secret that already exists replaces its value for new requests.

### Examples

```
  # Set a secret from a flag, an environment variable, a file or stdin
  bl secret set OPENAI_API_KEY --value sk-...
  bl secret set OPENAI_API_KEY --from-env OPENAI_API_KEY
  bl secret set TLS_CERT --from-file ./cert.pem
  cat token.txt | bl secret set GITHUB_TOKEN

  # List secret names (values are never shown)
  bl secret list

  # Delete a secret
  bl secret delete OPENAI_API_KEY
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
* [bl secret delete](bl_secret_delete.md)	 - Delete a workspace secret and all its versions
* [bl secret list](bl_secret_list.md)	 - List workspace secrets (names only, never values)
* [bl secret set](bl_secret_set.md)	 - Create or update a workspace secret


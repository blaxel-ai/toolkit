---
title: "bl token"
slug: bl_token
---
## bl token

Retrieve authentication token for a workspace

### Synopsis

Retrieve the authentication token for the specified workspace.

The token command displays the current authentication token used by the CLI
for API requests. This token is automatically managed and refreshed as needed.

Authentication Methods:
- API Key: Returns the API key
- OAuth (Browser Login): Returns the access token (refreshed if needed)
- Client Credentials: Returns the access token (refreshed if needed)

The token is retrieved from your stored credentials and will be automatically
refreshed if it's expired or about to expire.

Examples:
  # Get token for current workspace
  bl token

  # Get token for specific workspace
  bl token my-workspace

  # Use in scripts (get just the token value)
  export TOKEN=$(bl token)

```
bl token [workspace] [flags]
```

### Options

```
  -h, --help   help for token
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


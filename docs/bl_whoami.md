---
title: "bl whoami"
slug: bl_whoami
---
## bl whoami

Show which account and workspace the CLI is logged in as

### Synopsis

Show who the CLI is logged in as: the user or service account, the
current workspace, the authentication method (device login, API key or client
credentials) and the API URL.

bl whoami reads the saved login, or BL_API_KEY / BL_CLIENT_CREDENTIALS, and
makes one API call to confirm it and look up the account. If that call fails,
it still prints what it knows locally and says what failed. It never prints
tokens or keys.

It exits with status 1 when you are not logged in or the API could not confirm
the login, and tells you what to run next. With -o json the output is always a
JSON object with a logged_in field.

```
bl whoami [flags]
```

### Examples

```
  # Show the current login
  bl whoami

  # Machine-readable output for scripts and agents
  bl whoami -o json

  # Check the login for another workspace
  bl whoami -w my-workspace
```

### Options

```
  -h, --help   help for whoami
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


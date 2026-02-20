---
title: "bl login"
slug: bl_login
---
## bl login

Login to Blaxel

### Synopsis

Authenticate with Blaxel to access your workspace.

A workspace is your organization's isolated environment in Blaxel that contains
all your resources (agents, jobs, sandboxes, models, etc.). You must login before
using most Blaxel CLI commands.

Authentication Methods:
1. Browser OAuth (default) - Interactive login via web browser
2. API Key - For automation and scripts (set BL_API_KEY environment variable)
3. Client Credentials - For CI/CD pipelines (set BL_CLIENT_CREDENTIALS)

The CLI automatically detects which authentication method to use:
- If BL_CLIENT_CREDENTIALS is set, uses client credentials
- If BL_API_KEY is set, uses API key authentication
- Otherwise, shows interactive menu to choose browser or API key login

Credentials are stored securely in your system's credential store and persist
across sessions. Use 'bl logout' to remove stored credentials.

Examples:
  # Interactive login (shows menu to choose method)
  bl login my-workspace

  # Login without workspace (will prompt for workspace)
  bl login

  # API key authentication (non-interactive)
  `export BL_API_KEY=your-api-key`
  bl login my-workspace

  # Client credentials for CI/CD
  `export BL_CLIENT_CREDENTIALS=your-credentials`
  bl login my-workspace

After logging in, all commands will use this workspace by default.
Override with --workspace flag: bl get agents --workspace other-workspace

```
bl login [workspace] [flags]
```

### Options

```
  -h, --help   help for login
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


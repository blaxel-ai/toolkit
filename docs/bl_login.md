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

Without a terminal (for example when a coding agent runs the command), nothing
can be asked. With a workspace argument, bl login uses BL_CLIENT_CREDENTIALS or
BL_API_KEY when one is set, and the browser login otherwise. Without a workspace
argument it uses the browser login, then your current workspace if the login can
use it, or else the first of your workspaces by name, and says which one. The
browser login prints the login URL on its own line and how long it waits for you
to confirm in the browser.

Credentials are stored securely in your system's credential store and persist
across sessions. Use 'bl logout' to remove stored credentials.

Examples:

```bash
# Interactive login (shows menu to choose method)
bl login my-workspace

# Login without workspace (will prompt for workspace)
bl login

# API key authentication (non-interactive)
export BL_API_KEY=your-api-key
bl login my-workspace

# Client credentials for CI/CD
export BL_CLIENT_CREDENTIALS=your-credentials
bl login my-workspace
```

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

* [bl](bl.md)	 - Blaxel CLI - manage and deploy AI agents, sandboxes, and resources


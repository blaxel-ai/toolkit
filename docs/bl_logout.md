---
title: "bl logout"
slug: bl_logout
---
## bl logout

Logout from Blaxel

### Synopsis

Remove stored credentials for a workspace.

This command clears local authentication tokens and credentials from your
system's credential store. Your deployed resources (agents, jobs, sandboxes)
continue running and are not affected by logout.

If you have multiple workspaces authenticated, you can logout from:
- A specific workspace by providing its name
- Any workspace interactively by running 'bl logout' without arguments

After logging out, you'll need to run 'bl login WORKSPACE' again to
authenticate before using other commands for that workspace.

Note: Logout is a local operation only. It does not:
- Stop running agents or jobs
- Delete any deployed resources
- Revoke tokens on the server (they will expire naturally)
- Affect other authenticated workspaces

Examples:
  # Logout from current workspace (interactive selection)
  bl logout

  # Logout from specific workspace
  bl logout my-workspace

  # Login again after logout
  bl login my-workspace

```
bl logout [workspace] [flags]
```

### Options

```
  -h, --help   help for logout
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


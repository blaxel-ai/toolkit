---
title: "bl upgrade"
slug: bl_upgrade
---
## bl upgrade

Upgrade the Blaxel CLI to the latest version

### Synopsis

Upgrade the Blaxel CLI to the latest version.

This command automatically detects your installation method and updates
the CLI in the correct location to avoid version conflicts.

Supported installation methods:
  - Homebrew (brew)
  - Manual installation (install.sh)
  - Direct binary download

Examples:
  # Upgrade to the latest version
  bl upgrade

  # Upgrade to a specific version
  bl upgrade --version v1.2.3

  # Force reinstall even if already on latest version
  bl upgrade --force

```
bl upgrade [flags]
```

### Options

```
  -f, --force            Force reinstall even if already on latest version
  -h, --help             help for upgrade
      --version string   Target version to upgrade to (e.g., v1.2.3)
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


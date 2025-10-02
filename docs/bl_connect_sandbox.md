---
title: "bl connect sandbox"
slug: bl_connect_sandbox
---
## bl connect sandbox

Connect to a sandbox environment

### Synopsis

Connect to a sandbox environment using an interactive shell interface.

This command provides a terminal-like interface for:
- Executing commands in the sandbox
- Browsing files and directories
- Managing the sandbox environment

The shell connects to your sandbox via MCP (Model Control Protocol) over WebSocket.

Limitations:
- Interactive commands (vim, nano, less, top) are not supported
- Long-running commands may experience timeouts or interruptions
- Use non-interactive alternatives (cat, echo, ps) instead

Keyboard Shortcuts:
- Enter: Execute command
- ↑/↓: Navigate command history
- Ctrl+L: Clear screen
- Ctrl+C: Exit sandbox shell

Examples:
  bl connect sandbox my-sandbox
  bl connect sb my-sandbox
  bl connect sandbox production-env
  bl connect sandbox my-sandbox --url wss://custom.domain.com/sandbox/my-sandbox

```
bl connect sandbox [sandbox-name] [flags]
```

### Options

```
      --debug        Enable debug mode
  -h, --help         help for sandbox
      --url string   Custom WebSocket URL for MCP connection (defaults to wss://run.blaxel.ai/$WORKSPACE/sandboxes/$SANDBOX_NAME)
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

* [bl connect](bl_connect.md)	 - Connect into your sandbox resources


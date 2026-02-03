---
title: "bl serve"
slug: bl_serve
---
## bl serve

Serve a blaxel project

### Synopsis

Start a local development server for your Blaxel project.

This runs your agent or MCP server locally on your machine for rapid
development and testing. Perfect for the inner development loop where you
want to iterate quickly without deploying to the cloud.

Supported Languages:
- Python (requires pyproject.toml or requirements.txt)
- TypeScript/JavaScript (requires package.json)
- Go (requires go.mod)

Hot Reload:
Enable --hotreload to automatically restart your server when code changes
are detected. This dramatically speeds up development by eliminating manual
restarts.

Testing Locally:
While your server is running, test it with:
- bl chat agent-name --local   (for agents)
- bl run agent agent-name --local --data '{}'   (for agents)

Workflow:
1. bl serve --hotreload        Start local server with auto-reload
2. Edit your code               Make changes
3. Test immediately             Server reloads automatically
4. bl deploy                    Deploy when ready

```
bl serve [flags]
```

### Examples

```
  # Basic serve with hot reload (recommended)
  bl serve --hotreload

  # Serve on custom port
  bl serve --port 8080

  # Serve specific subdirectory in monorepo
  bl serve -d packages/my-agent

  # Serve with environment variables
  bl serve -e .env.local

  # Serve with secrets (for testing)
  bl serve -s API_KEY=test-key -s DB_PASSWORD=secret

  # Full development workflow
  bl serve --hotreload          # Terminal 1: Run server
  bl chat my-agent --local      # Terminal 2: Test agent
```

### Options

```
  -d, --directory string   Serve the project from a sub directory
  -e, --env-file strings   Environment file to load (default [.env])
  -h, --help               help for serve
  -H, --host string        Bind socket to this host. If 0.0.0.0, listens on all interfaces (default "0.0.0.0")
      --hotreload          Watch for changes in the project
  -p, --port int           Bind socket to this port (default 1338)
  -r, --recursive          Serve the project recursively (default true)
  -s, --secrets strings    Secrets to deploy
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


# Blaxel Toolkit

> A powerful CLI and Go SDK for building, deploying, and managing AI agents on the Blaxel platform.

## What is Blaxel?

Blaxel is a platform for deploying production-ready AI agents, MCP servers, sandboxes, and jobs. The Blaxel Toolkit provides everything you need to interact with the Blaxel platform from your command line or Go applications.

## Installation

### macOS and Linux

```bash
curl -fsSL https://blaxel.ai/install.sh | sh
```

### Windows (PowerShell)

```powershell
irm https://blaxel.ai/install.ps1 | iex
```

The installer verifies the download against the release checksums, puts `bl` on your PATH with shell completions, then runs `bl setup`: it finds your coding agents (Claude Code, Codex, Cursor and more), installs the Blaxel skills and MCP servers into them, and logs you in. Run `bl setup` again any time to set up new agents. When a coding agent runs the installer (no terminal, outside CI), it sets up the agents without screens, then starts the browser login and prints its URL for you to confirm; `BL_INSTALL_LOGIN=false` skips the login. Otherwise, without a terminal or in CI, it only installs the CLI; set `BL_INSTALL_SETUP=true` to run the setup with its defaults, or `BL_INSTALL_SETUP=false` to skip it. `VERSION=v0.1.118` installs a given release and `BINDIR=/usr/local/bin` another location.

### Homebrew

```bash
brew tap blaxel-ai/blaxel
brew install blaxel
bl setup
```

Releases are also on [GitHub](https://github.com/blaxel-ai/toolkit/releases); see [docs.blaxel.ai](https://docs.blaxel.ai/cli-reference/introduction) for more.

## Quick Start

```bash
# Login to your workspace
bl login my-workspace

# Create a new agent
bl new agent my-agent

# Deploy your agent
bl deploy

# Connect to a sandbox
bl connect sandbox my-sandbox

# Chat with your agent
bl chat my-agent
```

## Key Features

- **🤖 Agent Management**: Create, deploy, and chat with AI agents
- **🔌 MCP Servers**: Build Model Context Protocol servers for tool integration
- **📦 Sandboxes**: Interactive shell environments for remote execution
- **⚙️ Jobs**: Schedule and run background tasks
- **🚀 Deployment**: Interactive deployment with real-time logs
- **📊 Resource Management**: Get, delete, and manage all your Blaxel resources
- **🔐 Authentication**: Multiple auth methods (API key, device flow, client credentials)

## Core Commands

| Command | Description |
|---------|-------------|
| `bl login` | Authenticate with Blaxel workspace |
| `bl new` | Create agents, MCP servers, sandboxes, or jobs |
| `bl deploy` | Deploy your projects to Blaxel |
| `bl get` | List resources (agents, sandboxes, models, etc.) |
| `bl connect sandbox` | Interactive shell for sandbox environments |
| `bl chat` | Chat with deployed agents |
| `bl run` | Execute jobs or agents |
| `bl serve` | Run projects locally |

Run `bl --help` or `bl <command> --help` for detailed usage.

## Documentation

- 📖 [Full CLI Reference](https://docs.blaxel.ai/cli-reference)
- 🔧 [Command Examples](./docs/bl.md)
- 🚀 [Getting Started Guide](https://docs.blaxel.ai)
- 📦 [Sample Configurations](./samples)
- 💻 [ZSH Prompt Plugin](./contrib/zsh-blaxel-prompt) — Display your current Blaxel workspace in the shell prompt

## Repository Structure

```
.
├── cli/          # CLI command implementations
├── docs/         # Auto-generated command documentation
├── samples/      # Example configurations
└── test/         # Integration tests
```

## Development

See [CONTRIBUTING.md](./CONTRIBUTING.md) for development setup and guidelines.

```bash
# Build the CLI
make build

# Run tests
make test

# Update SDK dependency
go get -u github.com/blaxel-ai/sdk-go@latest
```

## Go SDK

The toolkit uses the Blaxel Go SDK for programmatic access to Blaxel APIs:

```go
import (
    blaxel "github.com/blaxel-ai/sdk-go"
    "github.com/blaxel-ai/sdk-go/option"
)

// Create a client with API key
client := blaxel.NewClient(option.WithAPIKey("your-api-key"))

// Or use environment-based authentication
client, _ := blaxel.NewDefaultClient()

// Use the SDK
agents, _ := client.Agents.List(ctx)
```

## License

See [LICENSE](./LICENSE) for details.
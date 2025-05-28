# Blaxel Toolkit

This repository contains the Blaxel CLI toolkit for interacting with Blaxel APIs.

## Installation

Follow the installation instructions at [docs.blaxel.ai](https://docs.blaxel.ai/cli-reference/introduction).

## Commands

### Explore Sandbox

The `explore sandbox` command provides an interactive shell interface for executing commands in sandbox environments using MCP (Model Control Protocol) over WebSocket. It works like a terminal where you can:

- Execute any shell command in the sandbox
- View command output in real-time
- Navigate through command history
- Use built-in shell features like `cd`, `pwd`, etc.
- Browse files and directories
- Manage the sandbox environment

#### Usage

```bash
bl explore sandbox [sandbox-name] [flags]
```

#### Examples

```bash
# Open an interactive shell for a sandbox
bl explore sandbox my-sandbox

# Open sandbox shell with debug mode enabled
bl explore sandbox my-sandbox --debug

# Connect to a custom WebSocket URL
bl explore sandbox my-sandbox --url wss://custom.domain.com/sandbox/my-sandbox
```

#### Flags

- `--debug`: Enable debug mode for verbose output and connection details
- `--url string`: Custom WebSocket URL for MCP connection (defaults to `wss://run.blaxel.ai/$WORKSPACE/sandboxes/$SANDBOX_NAME`)

#### Interactive Controls

**Shell Interface:**
- Type any command and press `Enter` to execute
- `↑/↓`: Navigate through command history
- `Ctrl+L`: Clear screen
- `Ctrl+C`: Exit sandbox shell

**Built-in Commands:**
- `help`: Show available commands and usage
- `clear` or `cls`: Clear the screen
- `pwd`: Show current directory
- `cd <path>`: Change directory
- `cat <file>`: Display file contents
- `echo <text>`: Display text
- `exit` or `quit`: Exit the sandbox shell

**Standard Shell Commands:**
All standard Unix/Linux commands are available and executed in the sandbox environment:
- `ls`, `ll`, `find`, `grep`: File operations and search
- `mkdir`, `touch`, `rm`: File/directory management
- `ps`, `top`, `kill`: Process management
- `git`, `npm`, `python`, etc.: Development tools (if available in sandbox)
- And many more...

#### How it Works

1. **Health Check**: Before connecting, the CLI converts the WebSocket URL to HTTP and checks the `/health` endpoint to verify the service is available
2. **MCP Connection**: Establishes a WebSocket connection to `wss://run.blaxel.ai/$WORKSPACE/sandboxes/$SANDBOX_NAME`
3. **Command Execution**: Uses MCP tools (`processExecute`) to run commands in the sandbox
4. **Real-time Output**: Retrieves command output directly from the execute response with `includeLogs: true`
5. **File Operations**: Browse and manage files using MCP tools (`get_file_or_directory`, `create_file`, etc.)
6. **Directory Tracking**: Automatically tracks current directory for navigation commands

#### Authentication

The command uses the same authentication system as other Blaxel CLI commands. Make sure you're logged in to the appropriate workspace:

```bash
bl auth login
```

The MCP connection uses your stored credentials (API key, access token, or client credentials) for authentication.

## Other Commands

For a full list of available commands, run:

```bash
bl --help
```

## Install cli on MacOS
```sh
brew tap blaxel-ai/blaxel
brew install blaxel
```
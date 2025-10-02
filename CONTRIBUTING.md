# Contributing to Blaxel Toolkit

Thank you for your interest in contributing to the Blaxel Toolkit!

## Development Setup

### Prerequisites

- Go 1.24 or higher
- Make
- GoReleaser (for building releases)

### Getting Started

1. Clone the repository:
```bash
git clone https://github.com/blaxel-ai/toolkit.git
cd toolkit
```

2. Install dependencies:
```bash
go mod download
```

3. Build the CLI:
```bash
make build-dev
```

This builds a development binary and installs it to `~/.local/bin/blaxel` (and symlinks as `bl`).

## Development Workflow

### Building

```bash
# Quick development build
make build-dev

# Full release build with goreleaser
make build

# Build SDK only
make build-sdk
```

### Testing

```bash
# Run unit tests
make test

# Run integration tests (requires BL_API_KEY)
export BL_API_KEY=your-api-key
export BL_WORKSPACE=your-workspace
make test-integration
```

### Generating Documentation

Auto-generate command documentation from CLI:

```bash
make doc
```

This creates markdown files in the `docs/` directory.

### Updating SDK

The SDK is auto-generated from the Blaxel API OpenAPI spec:

```bash
make sdk
```

This downloads the latest API definition and regenerates `sdk/blaxel.go`.

## Project Structure

```
cli/            # CLI command implementations
├── auth/       # Authentication commands
├── chat/       # Chat interface
├── core/       # Core utilities and config
├── deploy/     # Deployment logic
└── sandbox/    # Sandbox connection

sdk/            # Go SDK
├── blaxel.go   # Auto-generated API client
├── mcp/        # MCP protocol implementation
└── *.go        # Authentication and utilities

vscode/         # VSCode extension source
docs/           # Auto-generated CLI docs
samples/        # Example YAML configurations
templates/      # Code generation templates
test/           # Integration tests
```

## Adding New Commands

1. Create a new file in `cli/` (e.g., `cli/mycommand.go`)
2. Implement using the Cobra framework
3. Register the command in the appropriate parent command
4. Add tests if applicable
5. Run `make doc` to generate documentation

## Code Style

- Follow Go best practices and conventions
- Use `gofmt` for formatting
- Run `make lint` before committing

## Submitting Changes

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Ensure all tests pass
6. Submit a pull request

## Questions?

Feel free to open an issue for questions or discussions.

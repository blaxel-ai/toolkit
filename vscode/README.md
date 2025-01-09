<p align="center">
  <img alt="Beamlit Logo" src="https://beamlit.com/logo_short.png" height="140" />
  <h3 align="center">Visual Studio Code extension for Beamlit</h3>
  <p align="center">The complete platform to build AI agents and deploy them in secure
and sandboxed environments, for low-latency and high-availability.</p>
</p>

## Features

Add an explorer view where you can:
- List Beamlit resources
  - Agents
  - Functions
  - Models
  - Environments
  - Policies
  - Integration
- Describe every resources

## Requirements

1. Install Beamlit CLI
```bash
curl -fsSL https://raw.githubusercontent.com/beamlit/toolkit/main/install.sh | BINDIR=$HOME/.local/bin sh
```
2. Login to Beamlit's Platform
```
bl login YOUR_WORKSPACE
```

## Known Issues

- Have to login with CLI before using the extension
- You cannot switch workspace, to do so you need to use the cli to login or switch context to the workspace you want
- You cannot create resources on Beamlit platform

## Following extension guidelines

Ensure that you've read through the extensions guidelines and follow the best practices for creating your extension.

* [Extension Guidelines](https://code.visualstudio.com/api/references/extension-guidelines)

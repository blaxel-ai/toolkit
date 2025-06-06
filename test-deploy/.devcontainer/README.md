# DevContainer for Template DeepResearch

This DevContainer configuration provides a consistent development environment for the Template DeepResearch project.

## Features

- Python 3.12 environment
- UV package manager pre-installed
- Blaxel CLI pre-installed
- VS Code extensions for Python development
- Automatic dependency installation

## Getting Started

1. Ensure you have [VS Code](https://code.visualstudio.com/) and the [Remote - Containers](https://marketplace.visualstudio.com/items?itemName=ms-vscode-remote.remote-containers) extension installed.
2. Open this project in VS Code with the Remote - Containers extension installed
3. VS Code will prompt you to reopen the project in a container - accept this prompt
4. The container will build and set up the environment automatically
5. Once the container is ready, you'll need to:
   - Create a `.env` file from `.env-sample` and add your API keys
   - Login to Blaxel with `bl login YOUR-WORKSPACE`
   - Apply your Blaxel configuration with `bl apply -R -f .blaxel`
6. You can now:
   - Use the integrated terminal to run TypeScript commands
   - Edit files with full TypeScript language support
   - Run and debug your application with Blaxel CLI `bl serve --hotreload`

## Customization

You can customize the development container by modifying the `devcontainer.json` file. Common customizations include:

- Adding additional VS Code extensions
- Changing Node.js/TypeScript versions
- Adding environment variables
- Installing additional tools

## Troubleshooting

If you encounter issues with the development container:

1. Rebuild the container using the command palette: "Remote-Containers: Rebuild Container"
2. Check that Docker is running properly on your system
3. Verify your VS Code Remote - Containers extension is up to date
4. If Blaxel CLI commands fail, try reinstalling with: `curl -fsSL https://raw.githubusercontent.com/blaxel-ai/toolkit/main/install.sh | BINDIR=~/.local/bin sh`

---
title: "bl deploy"
slug: bl_deploy
---
## bl deploy

Deploy on blaxel

### Synopsis

Deploy your Blaxel project to the cloud.

This command packages your code, builds a container image, and deploys it
to your workspace. The deployment process includes:
1. Reading configuration from blaxel.toml
2. Packaging source code (respects .blaxelignore)
3. Building container image with your runtime and dependencies
4. Uploading to Blaxel's container registry
5. Creating or updating the resource in your workspace
6. Streaming build and deployment logs (interactive mode)

You must run this command from a directory containing a blaxel.toml file.

Interactive vs Non-Interactive:
- Interactive (default): Shows live logs and deployment progress with TUI
- Non-interactive (--yes or CI): Runs without interactive UI, suitable for automation

Environment Variables and Secrets:
Use -e to load .env files or -s to pass secrets directly via command line.
Secrets are injected into your container at runtime and never stored in images.

Monorepo Support:
Use -d to deploy a specific subdirectory, or -R to recursively deploy
all projects in a monorepo (looks for blaxel.toml in subdirectories).

```
bl deploy [flags]
```

### Examples

```
  # Basic deployment (interactive mode with live logs)
  bl deploy

  # Non-interactive deployment (for CI/CD)
  bl deploy --yes

  # Deploy with environment variables
  bl deploy -e .env.production

  # Deploy with command-line secrets
  bl deploy -s API_KEY=xxx -s DB_PASSWORD=yyy

  # Deploy without rebuilding (reuse existing image)
  bl deploy --skip-build

  # Dry run to validate configuration
  bl deploy --dryrun

  # Deploy specific subdirectory in monorepo
  bl deploy -d ./packages/my-agent

  # Recursively deploy all projects in monorepo
  bl deploy -R
```

### Options

```
  -d, --directory string   Deployment app path, can be a sub directory
      --dryrun             Dry run the deployment
  -e, --env-file strings   Environment file to load (default [.env])
  -h, --help               help for deploy
  -n, --name string        Optional name for the deployment
  -r, --recursive          Deploy recursively (default true)
  -s, --secrets strings    Secrets to deploy
      --skip-build         Skip the build step
  -y, --yes                Skip interactive mode
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


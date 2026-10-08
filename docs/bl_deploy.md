---
title: "bl deploy"
slug: bl_deploy
---
## bl deploy

Build, push, and deploy your project to Blaxel

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

A blaxel.toml configuration file is required. By default, the command looks
for it in the current directory. Use -d to specify a subdirectory containing
the blaxel.toml (useful for monorepo setups).

Use --dockerfile to select a Dockerfile, or set build.dockerfile in blaxel.toml
(the flag wins). Paths are relative to the project directory (-d, or the current
directory) and must stay inside it. The build context is unchanged, so COPY paths
are unaffected. A custom Dockerfile needs a source build and one project per
deploy (use --recursive=false when the config lists child packages). It and its
adjacent .dockerignore are uploaded even if .blaxelignore excludes them.

If the blaxel.toml contains an 'image' field pointing to a registry image,
the platform will pull the image and transform it via metamorph before deploying.
For private registries, supply credentials via --registry-cred or --docker-config.

Interactive vs Non-Interactive:
- Interactive (default): Shows live logs and deployment progress with TUI
- Non-interactive (--yes or CI): Runs without interactive UI, suitable for automation.
  Returns once the code is submitted: exit 0 means accepted, not deployed.
- Non-interactive with --wait: Waits for this build and rollout (up to --timeout)
  and exits 1 with the cause, failing Dockerfile step, a short log tail and a next
  command if it fails or times out. Interactive mode always waits.

With --wait and -o json (or yaml), a failure adds resources[].diagnostics (code,
phase, cause, step, source, logTail, next). An unchanged redeploy creates no new revision:
--wait reports that in resources[].note and exits 0. If no build starts within 3
minutes of the upload, --wait stops with DEPLOY_TIMEOUT instead of waiting for --timeout.

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

  # Non-interactive deployment (for CI/CD); returns once submitted
  bl deploy --yes

  # Non-interactive deployment that waits for build and rollout and explains failures
  bl deploy --yes --wait --timeout 20m

  # Deploy using a custom Dockerfile
  bl deploy --dockerfile blaxel.Dockerfile

  # Select a Dockerfile relative to a project subdirectory
  bl deploy -d sandbox --dockerfile Dockerfile.v2

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

  # Deploy specifying a resource type
  bl deploy --type sandbox

  # Deploy with Docker build args from a .env.build file
  bl deploy --build-env-file .env.build.production

  # Recursively deploy all projects in monorepo
  bl deploy -R
```

### Options

```
      --build-env-file string       Path to a build env file with Docker build args (default: auto-detect .env.build)
  -d, --directory string            Deployment app path, can be a sub directory
      --docker-config string        Path to a Docker config.json file with registry credentials
      --dockerfile string           Dockerfile path relative to the project directory (overrides build.dockerfile in blaxel.toml)
      --dryrun                      Dry run the deployment
  -e, --env-file strings            Environment file to load (default [.env])
      --experimental                Enable experimental features (e.g. USER directive support)
  -h, --help                        help for deploy
  -n, --name string                 Optional name for the deployment
  -r, --recursive                   Deploy recursively (default true)
  -c, --registry-cred stringArray   Registry credentials (format: registry=username:password, repeatable)
  -s, --secrets strings             Secrets to deploy
      --skip-build                  Skip the build step
      --timeout string              Timeout for build and deployment monitoring (e.g. 30m, 1h). Defaults to 1h
  -t, --type string                 Resource type (sandbox, agent, function, job, application). Defaults to blaxel.toml type or 'sandbox'
      --wait                        In non-interactive mode, wait for build and rollout, exit 1 on failure and explain it (see --timeout)
  -y, --yes                         Skip interactive mode
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

* [bl](bl.md)	 - Blaxel CLI - manage and deploy AI agents, sandboxes, and resources


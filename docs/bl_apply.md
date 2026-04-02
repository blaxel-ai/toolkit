---
title: "bl apply"
slug: bl_apply
---
## bl apply

Apply a configuration to a resource by file

### Synopsis

Apply configuration changes to resources declaratively using YAML files.

This command is similar to Kubernetes 'kubectl apply' - it creates resources
if they don't exist, or updates them if they do (idempotent operation).

Use 'apply' for Infrastructure as Code workflows where you:
- Manage resources via configuration files
- Version control your infrastructure
- Deploy multiple related resources together
- Implement GitOps practices

Difference from 'deploy':
- 'apply' manages resource configuration (metadata, settings, specs)
- 'deploy' builds and uploads code as container images

For deploying code changes to agents/jobs, use 'bl deploy'.
For managing resource configuration, use 'bl apply'.

The command respects environment variables and secrets, which can be injected
via -e flag for .env files or -s flag for command-line secrets.

```
bl apply [flags]
```

### Examples

```
  # Apply a single resource
  bl apply -f agent.yaml

  # Apply all resources in directory
  bl apply -f ./resources/ -R

  # Apply with environment variable substitution
  bl apply -f deployment.yaml -e .env.production

  # Apply from stdin (useful for CI/CD)
  cat config.yaml | bl apply -f -

  # Apply with secrets
  bl apply -f config.yaml -s API_KEY=xxx -s DB_PASSWORD=yyy

  # Example YAML structure for an agent:
  # apiVersion: blaxel.ai/v1alpha1
  # kind: Agent
  # metadata:
  #   name: my-agent
  # spec:
  #   runtime:
	#     image: agent/my-template-agent:latest
  #     memory: 4096

  # Create a sandbox with the default base image
  bl apply -f - <<EOF
  apiVersion: blaxel.ai/v1alpha1
  kind: Sandbox
  metadata:
    name: my-sandbox
  spec:
    runtime:
      image: blaxel/base-image:latest
      memory: 2048
  EOF

  # Create a sandbox with a custom pushed image
  bl apply -f - <<EOF
  apiVersion: blaxel.ai/v1alpha1
  kind: Sandbox
  metadata:
    name: my-sandbox
  spec:
    runtime:
      image: sandbox/my-sandbox:latest
      memory: 4096
  EOF

  # Create a sandbox in a specific region with a volume
  bl apply -f - <<EOF
  apiVersion: blaxel.ai/v1alpha1
  kind: Sandbox
  metadata:
    name: my-sandbox
  spec:
    region: eu-lon-1
    volumes:
      - name: my-volume
        mountPath: /data
    runtime:
      image: blaxel/base-image:latest
      ports:
        - name: sandbox-api
          target: 8080
          protocol: HTTP
      memory: 4096
  EOF
```

### Options

```
  -e, --env-file strings   Environment file to load (default [.env])
  -f, --filename string    Path to YAML file to apply
  -h, --help               help for apply
  -R, --recursive          Process the directory used in -f, --filename recursively. Useful when you want to manage related manifests organized within the same directory.
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


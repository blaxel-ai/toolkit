---
title: "bl get"
slug: bl_get
---
## bl get

Get a resource

### Synopsis

Retrieve information about Blaxel resources in your workspace.

A "resource" in Blaxel refers to any deployable or manageable entity:
- agents: AI agent applications
- functions/mcp: Model Context Protocol servers (tool providers)
- jobs: Batch processing tasks
- sandboxes: Isolated execution environments
- models: AI model configurations
- policies: Access control policies
- volumes: Persistent storage
- integrationconnections: External service integrations

Output Formats:
Use -o flag to control output format:
- pretty: Human-readable colored output (default)
- json: Machine-readable JSON (for scripting)
- yaml: YAML format
- table: Tabular format with columns

Watch Mode:
Use --watch to continuously monitor a resource and see updates in real-time.
Useful for tracking deployment status or watching for changes.

The command can list all resources of a type or get details for a specific one.

### Examples

```
  # List all agents
  bl get agents

  # Get specific agent details
  bl get agent my-agent

  # Get in JSON format (useful for scripting)
  bl get agent my-agent -ojson

  # Watch agent status in real-time
  bl get agent my-agent --watch

  # List all resources with table output
  bl get agents -otable

  # Get MCP servers (also called functions)
  bl get functions
  bl get mcp

  # List jobs
  bl get jobs

  # Get specific job
  bl get job my-job

  # List executions for a job (nested resource)
  bl get job my-job executions

  # Get specific execution for a job
  bl get job my-job execution <execution-id>

  # Monitor sandbox status
  bl get sandbox my-sandbox --watch
```

### Options

```
  -h, --help    help for get
      --watch   After listing/getting the requested object, watch for changes.
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
* [bl get agents](bl_get_agents.md)	 - Get a Agent
* [bl get functions](bl_get_functions.md)	 - Get a Function
* [bl get integrationconnections](bl_get_integrationconnections.md)	 - Get a IntegrationConnection
* [bl get jobs](bl_get_jobs.md)	 - Get a Job
* [bl get models](bl_get_models.md)	 - Get a Model
* [bl get policies](bl_get_policies.md)	 - Get a Policy
* [bl get sandboxes](bl_get_sandboxes.md)	 - Get a Sandbox
* [bl get volumes](bl_get_volumes.md)	 - Get a Volume
* [bl get volumetemplates](bl_get_volumetemplates.md)	 - Get a VolumeTemplate


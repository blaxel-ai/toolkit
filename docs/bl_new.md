---
title: "bl new"
slug: bl_new
---
## bl new

Create a new blaxel resource (agent, mcp, sandbox, job)

### Synopsis

Create a new Blaxel resource from templates.

This command scaffolds a new project with the necessary configuration files,
dependencies, and example code to get you started quickly.

Resource Types:
  agent     - AI agent application that can chat, use tools, and access data
              Use cases: Customer support bots, coding assistants, data analysts

  mcp       - Model Context Protocol server that extends agent capabilities
              Use cases: Custom tools, API integrations, database connectors

  sandbox   - Isolated execution environment for testing and running code
              Use cases: Code execution, testing, isolated workloads

  job       - Batch processing task that runs on-demand or on schedule
              Use cases: ETL pipelines, data processing, scheduled workflows

Interactive Mode (Recommended):
When called without arguments, the CLI guides you through:
1. Choosing a resource type
2. Selecting a template (language/framework)
3. Naming your project directory
4. Setting up initial configuration

Non-Interactive Mode:
Use --template and --yes flags for automation and CI/CD workflows.

After Creation:
1. cd into your new directory
2. Review and customize the generated blaxel.toml configuration
3. Develop your resource locally with 'bl serve --hotreload'
4. Test it works as expected
5. Deploy to Blaxel with 'bl deploy'

```
bl new [type] [directory] [flags]
```

### Examples

```
  # Interactive creation (recommended for beginners)
  bl new

  # Create agent interactively
  bl new agent

  # Create agent with specific template
  bl new agent my-agent -t template-google-adk-py

  # Create MCP server with default template (non-interactive)
  bl new mcp my-mcp-server -y

  # Create job with specific template
  bl new job my-batch-job -t template-job-py

  # Full workflow example:
  bl new agent my-assistant
  cd my-assistant
  bl serve --hotreload    # Test locally
  bl deploy               # Deploy to Blaxel
  bl chat my-assistant    # Chat with deployed agent
```

### Options

```
  -h, --help              help for new
  -t, --template string   Template to use (skips interactive prompt)
  -y, --yes               Skip interactive prompts and use defaults
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


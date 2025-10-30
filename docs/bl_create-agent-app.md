---
title: "bl create-agent-app"
slug: bl_create-agent-app
---
## bl create-agent-app

Create a new blaxel agent app

### Synopsis

Create a new AI agent application from templates.

An agent is a conversational AI system that can interact with users, access tools,
maintain context across conversations, and integrate with external services.

Common use cases:
- Customer support chatbots
- Data analysis assistants
- Code review helpers
- Personal productivity assistants
- Domain-specific expert systems

The command scaffolds a complete agent project with configuration, dependencies,
and example code. You can choose from multiple templates supporting different
frameworks (Google ADK, LangChain, custom, etc.) and languages (Python, TypeScript).

After creation: cd into the directory, run 'bl serve --hotreload' for local
development, then 'bl deploy' when ready to deploy.

Note: Prefer using 'bl new agent' which provides a unified creation experience.

```
bl create-agent-app [directory] [flags]
```

### Examples

```
  # Interactive creation
  bl create-agent-app my-agent

  # With specific template
  bl create-agent-app my-agent --template template-google-adk-py

  # Non-interactive with defaults
  bl create-agent-app my-agent --template template-google-adk-py -y

  # Recommended: Use unified 'new' command instead
  bl new agent my-agent
```

### Options

```
  -h, --help              help for create-agent-app
  -t, --template string   Template to use for the agent app (skips interactive prompt)
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


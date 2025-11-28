---
title: "bl chat"
slug: bl_chat
---
## bl chat

Chat with an agent

### Synopsis

Start an interactive chat session with a deployed agent.

This command opens a terminal-based chat interface where you can send messages
to your agent and see responses in real-time. Perfect for testing agent behavior,
exploring capabilities, or debugging conversational flows.

The agent must be deployed and in DEPLOYED status. Use 'bl get agent NAME'
to check deployment status before chatting.

Local Testing:
Use --local flag to chat with a locally running agent (requires 'bl serve'
to be running in another terminal). This is useful during development.

Debug Mode:
Enable --debug to see detailed API calls, responses, and timing information.
Helpful for troubleshooting issues or understanding agent behavior.

Keyboard Controls:
- Type your message and press Enter to send
- Ctrl+C to exit chat session
- Ctrl+L to clear screen (if supported)

```
bl chat [agent-name] [flags]
```

### Examples

```
  # Chat with deployed agent
  bl chat my-agent

  # Chat with local development agent (requires 'bl serve')
  bl chat my-agent --local

  # Debug mode (shows API calls and responses)
  bl chat my-agent --debug

  # Add custom headers (for authentication, metadata, etc.)
  bl chat my-agent --header "X-User-ID: 123" --header "X-Session: abc"

  # Development workflow
  bl serve --hotreload         # Terminal 1: Run locally
  bl chat my-agent --local     # Terminal 2: Test chat
```

### Options

```
      --debug            Debug mode
      --header strings   Request headers in 'Key: Value' format. Can be specified multiple times
  -h, --help             help for chat
      --local            Run locally
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


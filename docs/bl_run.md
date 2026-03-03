---
title: "bl run"
slug: bl_run
---
## bl run

Run a resource on blaxel

### Synopsis

Execute a Blaxel resource with custom input data.

Different resource types behave differently when run:

- agent: Send a single request (non-interactive, unlike 'bl chat')
         Returns agent response for the given input

- model: Make an inference request to an AI model
         Calls the model's API endpoint with your data

- job: Start a job execution with batch input
       Processes multiple tasks defined in JSON batch file

- function/mcp: Invoke an MCP server function
                Calls a specific tool or method

Local vs Remote:
- Remote (default): Runs against deployed resources in your workspace
- Local (--local): Runs against locally served resources (requires 'bl serve')

Input Formats:
- Inline JSON with --data json-object
- From file with --file path/to/input.json

Advanced Usage:
Use --path, --method, and --params for custom HTTP requests to your resources.
This is useful for testing specific endpoints or non-standard API calls.

```
bl run resource-type resource-name [flags]
```

### Examples

```
  # Run agent with inline data
  bl run agent my-agent --data '{"inputs": "Summarize this text"}'

  # Run agent with file input
  bl run agent my-agent --file request.json

  # Run job with batch file
  bl run job my-job --file batches/process-users.json

  # Run job locally for testing (requires 'bl serve' in another terminal)
  bl run job my-job --local --file batch.json

  # Run job locally with 4 concurrent workers
  bl run job my-job --local --file batch.json --concurrent 4

  # Run model with custom endpoint
  bl run model my-model --path /v1/chat/completions --data '{"messages": [...]}'

  # Run with query parameters
  bl run agent my-agent --data '{}' --params "stream=true" --params "max_tokens=100"

  # Run with custom headers
  bl run agent my-agent --data '{}' --header "X-User-ID: 123"

  # Debug mode (see full request/response details)
  bl run agent my-agent --data '{}' --debug
```

### Options

```
  -c, --concurrent int       Number of concurrent workers for local job execution (default 1)
  -d, --data string          JSON body data for the inference request
      --debug                Debug mode
      --directory string     Directory to run the command from
  -e, --env-file strings     Environment file to load (default [.env])
  -f, --file string          Input from a file
      --header stringArray   Request headers in 'Key: Value' format. Can be specified multiple times
  -h, --help                 help for run
      --local                Run locally
      --method string        HTTP method for the inference request (default "POST")
  -o, --output string        Output format: json, yaml
      --params strings       Query params sent to the inference request
      --path string          path for the inference request
  -s, --secrets strings      Secrets to pass to the execution
      --upload-file string   This transfers the specified local file to the remote URL
```

### Options inherited from parent commands

```
      --skip-version-warning   Skip version warning
  -u, --utc                    Enable UTC timezone
  -v, --verbose                Enable verbose output
  -w, --workspace string       Specify the workspace name
```

### SEE ALSO

* [bl](bl.md)	 - Blaxel CLI is a command line tool to interact with Blaxel APIs.


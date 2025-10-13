---
title: "bl logs"
slug: bl_logs
---
## bl logs

View logs for a resource

### Synopsis

View logs for Blaxel resources.

The logs command displays logs for agents, jobs, sandboxes, and functions.
You must specify both the resource type and resource name.

Resource Types (with aliases):
- sandboxes (sandbox, sbx)
- jobs (job, j, jb)
- agents (agent, ag)
- functions (function, fn, mcp, mcps)

Time Filtering:
By default, logs from the last 1 hour are displayed.
In follow mode (--follow), the last 15 minutes are shown as context, then new logs
are continuously streamed in real-time.
You can customize this by:
- Using duration format (e.g., 3d, 1h, 10m, 24h) with --period flag
- Using explicit start/end times with --start and --end flags
- Maximum time range is 3 days

Duration units:
- d: days
- h: hours
- m: minutes
- s: seconds

Timestamps:
By default, logs are prefixed with their timestamp in local timezone.
Use --no-timestamps to hide them, or --utc to display timestamps in UTC.

Severity Filtering:
By default, all severity levels are shown. Use --severity to filter by specific levels.
Available severities: FATAL, ERROR, WARNING, INFO, DEBUG, TRACE, UNKNOWN
Use comma-separated values: --severity ERROR,FATAL

Search:
Use --search to filter logs by text content. Only logs containing the search term will be displayed.

Job-Specific Filtering:
When viewing logs for jobs, you can filter by specific task or execution:
- --task-id: Filter logs for a specific task ID
- --execution-id: Filter logs for a specific execution ID

Examples:
  # View logs for a specific sandbox (last 1 hour - default)
  bl logs sandbox my-sandbox

  # Follow logs in real-time (shows last 15 minutes, then streams new logs)
  bl logs sandbox my-sandbox --follow

  # Follow logs with more historical context
  bl logs sandbox my-sandbox --follow --period 1h

  # View logs from last 3 days
  bl logs job my-job --period 3d

  # View logs with 1 hour period (explicit)
  bl logs sandbox my-sandbox --period 1h

  # View logs for a specific time range
  bl logs agent my-agent --start 2024-01-01T00:00:00Z --end 2024-01-01T23:59:59Z

  # Hide timestamps in output
  bl logs agent my-agent --no-timestamps

  # Show timestamps in UTC
  bl logs agent my-agent --utc

  # Filter by severity
  bl logs agent my-agent --severity ERROR,FATAL

  # Search for specific text in logs
  bl logs agent my-agent --search "error"

  # Filter job logs by task ID
  bl logs job my-job --task-id task-123

  # Filter job logs by execution ID
  bl logs job my-job --execution-id exec-456

  # Combine filters
  bl logs job my-job --severity ERROR --search "timeout" --task-id task-123

  # Using aliases
  bl logs sbx my-sandbox --follow
  bl logs j my-job --period 1h
  bl logs fn my-function --follow

```
bl logs RESOURCE_TYPE RESOURCE_NAME [flags]
```

### Options

```
      --end string            End time for logs (RFC3339 format or YYYY-MM-DD)
      --execution-id string   Filter logs by execution ID (job resources only)
  -f, --follow                Follow log output (like tail -f)
  -h, --help                  help for logs
      --no-timestamps         Hide timestamps in log output
  -p, --period string         Time period to fetch logs (e.g., 3d, 1h, 10m, 24h)
      --search string         Search for logs containing specific text
      --severity string       Filter by severity levels (comma-separated): FATAL,ERROR,WARNING,INFO,DEBUG,TRACE,UNKNOWN
      --start string          Start time for logs (RFC3339 format or YYYY-MM-DD)
      --task-id string        Filter logs by task ID (job resources only)
      --utc                   Display timestamps in UTC instead of local timezone
```

### Options inherited from parent commands

```
  -o, --output string          Output format. One of: pretty,yaml,json,table
      --skip-version-warning   Skip version warning
  -v, --verbose                Enable verbose output
  -w, --workspace string       Specify the workspace name
```

### SEE ALSO

* [bl](bl.md)	 - Blaxel CLI is a command line tool to interact with Blaxel APIs.


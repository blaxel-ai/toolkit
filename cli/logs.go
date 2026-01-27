package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/monitor"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("logs", func() *cobra.Command {
		return LogsCmd()
	})
}

// normalizeResourceType converts resource type aliases to their canonical singular form
// (The monitor package will handle pluralization)
func normalizeResourceType(resourceType string) (string, error) {
	rt := strings.ToLower(resourceType)

	// Map of aliases to canonical singular forms
	aliases := map[string]string{
		// Sandboxes
		"sandbox":   "sandbox",
		"sbx":       "sandbox",
		"sandboxes": "sandbox",

		// Jobs
		"job":  "job",
		"j":    "job",
		"jb":   "job",
		"jobs": "job",

		// Agents
		"agent":  "agent",
		"ag":     "agent",
		"agents": "agent",

		// Functions
		"function":  "function",
		"fn":        "function",
		"mcp":       "function",
		"mcps":      "function",
		"functions": "function",
	}

	if canonical, ok := aliases[rt]; ok {
		return canonical, nil
	}

	return "", fmt.Errorf("invalid resource type '%s'. Valid types: sandbox/sbx, job/j, agent/ag, function/fn/mcp", resourceType)
}

// parseTimeFlag parses a time string flag value
func parseTimeFlag(timeStr string) (time.Time, error) {
	// Try RFC3339 first (has timezone)
	if t, err := time.Parse(time.RFC3339, timeStr); err == nil {
		return t, nil
	}

	// Try datetime format with timezone assumption
	if t, err := time.Parse("2006-01-02T15:04:05", timeStr); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC), nil
	}

	if t, err := time.Parse("2006-01-02 15:04:05", timeStr); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), 0, time.UTC), nil
	}

	// Date only - parse and set to noon in UTC to avoid API midnight bug
	if t, err := time.Parse("2006-01-02", timeStr); err == nil {
		return time.Date(t.Year(), t.Month(), t.Day(), 12, 0, 0, 0, time.UTC), nil
	}

	return time.Time{}, fmt.Errorf("invalid time format '%s'. Use RFC3339 format (e.g., 2006-01-02T15:04:05Z) or YYYY-MM-DD", timeStr)
}

// validateTimeRange ensures the time range doesn't exceed 3 days
func validateTimeRange(start, end time.Time) error {
	duration := end.Sub(start)
	maxDuration := 3 * 24 * time.Hour // 3 days

	if duration > maxDuration {
		return fmt.Errorf("time range exceeds maximum of 3 days (requested: %v)", duration)
	}

	if duration < 0 {
		return fmt.Errorf("start time must be before end time")
	}

	return nil
}

func LogsCmd() *cobra.Command {
	var (
		follow       bool
		period       string
		startTimeStr string
		endTimeStr   string
		noTimestamps bool
		utc          bool
		severity     string
		search       string
	)

	cmd := &cobra.Command{
		Use:               "logs RESOURCE_TYPE RESOURCE_NAME [NESTED_ARGS...]",
		Short:             "View logs for a resource",
		ValidArgsFunction: GetLogsValidArgsFunction(),
		Long: `View logs for Blaxel resources.

The logs command displays logs for agents, jobs, sandboxes, and functions.
You must specify both the resource type and resource name.

Resource Types (with aliases):
- sandboxes (sandbox, sbx)
- jobs (job, j, jb)
- agents (agent, ag)
- functions (function, fn, mcp, mcps)

Sandbox Process Logs:
For sandboxes, you can view logs for a specific process by adding the process name:
  bl logs sandbox my-sandbox my-process

Job Execution Logs:
For jobs, you can filter logs by execution ID and task ID:
  bl logs job my-job <execution-id>
  bl logs job my-job <execution-id> <task-id>

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

Examples:
  # View logs for a specific sandbox (last 1 hour - default)
  bl logs sandbox my-sandbox

  # View logs for a specific process in a sandbox
  bl logs sandbox my-sandbox my-process

  # Stream process logs in real-time
  bl logs sandbox my-sandbox my-process --follow

  # View all logs for a job
  bl logs job my-job

  # View logs for a specific job execution
  bl logs job my-job exec-abc123

  # View logs for a specific task within an execution
  bl logs job my-job exec-abc123 task-456

  # Follow job execution logs in real-time
  bl logs job my-job exec-abc123 --follow

  # Follow logs in real-time (shows last 15 minutes, then streams new logs)
  bl logs sandbox my-sandbox --follow

  # Follow logs with more historical context
  bl logs sandbox my-sandbox --follow --period 1h

  # View logs from last 3 days
  bl logs job my-job --period 3d

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

  # Using aliases
  bl logs sbx my-sandbox --follow
  bl logs j my-job --period 1h
  bl logs fn my-function --follow`,
		Args: cobra.RangeArgs(2, 4),
		Run: func(cmd *cobra.Command, args []string) {
			resourceType := args[0]
			resourceName := args[1]

			// Normalize resource type
			canonicalType, err := normalizeResourceType(resourceType)
			if err != nil {
				core.PrintError("logs", err)
				core.ExitWithError(err)
			}

			// Parse nested arguments based on resource type
			var executionID, taskID, processName string
			if len(args) >= 3 {
				switch canonicalType {
				case "sandbox":
					processName = args[2]
				case "job":
					executionID = args[2]
					if len(args) >= 4 {
						taskID = args[3]
					}
				default:
					err := fmt.Errorf("extra arguments are only valid for sandbox (process) or job (execution/task) resources")
					core.PrintError("logs", err)
					core.ExitWithError(err)
				}
			}

			// Handle sandbox process logs
			if canonicalType == "sandbox" && processName != "" {
				if follow {
					streamSandboxProcessLogs(resourceName, processName)
				} else {
					getSandboxProcessLogs(resourceName, processName)
				}
				return
			}

			// Determine time range
			var startTime, endTime time.Time

			if startTimeStr != "" && endTimeStr != "" {
				// Use explicit start and end times
				startTime, err = parseTimeFlag(startTimeStr)
				if err != nil {
					err = fmt.Errorf("invalid start time: %v", err)
					core.PrintError("logs", err)
					core.ExitWithError(err)
				}

				endTime, err = parseTimeFlag(endTimeStr)
				if err != nil {
					err = fmt.Errorf("invalid end time: %v", err)
					core.PrintError("logs", err)
					core.ExitWithError(err)
				}
			} else if period != "" {
				// Use period (e.g., "3d", "1h")
				duration, err := core.ParseDuration(period)
				if err != nil {
					core.PrintError("logs", err)
					core.ExitWithError(err)
				}

				endTime = time.Now().UTC()
				startTime = endTime.Add(-duration)
			} else if startTimeStr != "" {
				// Only start time provided
				startTime, err = parseTimeFlag(startTimeStr)
				if err != nil {
					err = fmt.Errorf("invalid start time: %v", err)
					core.PrintError("logs", err)
					core.ExitWithError(err)
				}

				if endTimeStr == "" {
					endTime = time.Now().UTC()
				}
			} else {
				// Default behavior depends on whether we're following
				endTime = time.Now().UTC()
				if follow {
					// In follow mode, default to showing only new logs from now
					startTime = endTime
				} else {
					// In normal mode, default to last 1 hour
					startTime = endTime.Add(-1 * time.Hour)
				}
			}

			// Validate time range (skip for follow mode with same start/end)
			if !follow {
				if err := validateTimeRange(startTime, endTime); err != nil {
					core.PrintError("logs", err)
					core.ExitWithError(err)
				}
			}

			workspace := core.GetWorkspace()
			if workspace == "" {
				ctx, _ := blaxel.CurrentContext()
				workspace = ctx.Workspace
			}

			if workspace == "" {
				err := fmt.Errorf("no workspace specified. Use 'bl login <workspace>' to authenticate")
				core.PrintError("logs", err)
				core.ExitWithError(err)
			}

			if follow {
				// Follow logs mode - show some context if period was specified
				if period == "" && startTimeStr == "" {
					// No period specified, show last 15 minutes of context
					startTime = endTime.Add(-15 * time.Minute)
				}
				followLogs(workspace, canonicalType, resourceName, startTime, noTimestamps, utc, severity, search, taskID, executionID)
			} else {
				// Fetch logs once
				fetchLogs(workspace, canonicalType, resourceName, startTime, endTime, noTimestamps, utc, severity, search, taskID, executionID)
			}
		},
	}

	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Follow log output (like tail -f)")
	cmd.Flags().StringVarP(&period, "period", "p", "", "Time period to fetch logs (e.g., 3d, 1h, 10m, 24h)")
	cmd.Flags().StringVar(&startTimeStr, "start", "", "Start time for logs (RFC3339 format or YYYY-MM-DD)")
	cmd.Flags().StringVar(&endTimeStr, "end", "", "End time for logs (RFC3339 format or YYYY-MM-DD)")
	cmd.Flags().BoolVar(&noTimestamps, "no-timestamps", false, "Hide timestamps in log output")
	cmd.Flags().BoolVar(&utc, "utc", false, "Display timestamps in UTC instead of local timezone")
	cmd.Flags().StringVar(&severity, "severity", "", "Filter by severity levels (comma-separated): FATAL,ERROR,WARNING,INFO,DEBUG,TRACE,UNKNOWN")
	cmd.Flags().StringVar(&search, "search", "", "Search for logs containing specific text")

	return cmd
}

// formatLogOutput formats a log entry with optional timestamp
func formatLogOutput(logEntry monitor.LogEntry, noTimestamps bool, utc bool) string {
	if noTimestamps {
		return logEntry.Message
	}

	// Parse the timestamp
	t, err := time.Parse(time.RFC3339Nano, logEntry.Timestamp)
	if err != nil {
		// If parsing fails, use the raw timestamp
		return fmt.Sprintf("[%s] %s", logEntry.Timestamp, logEntry.Message)
	}

	// Convert to local timezone unless UTC is requested
	if !utc {
		t = t.Local()
	}

	// Format as: 2006-01-02 15:04:05.000
	return fmt.Sprintf("[%s] %s", t.Format("2006-01-02 15:04:05.000"), logEntry.Message)
}

// fetchLogs fetches logs for a given time range
func fetchLogs(workspace, resourceType, resourceName string, startTime, endTime time.Time, noTimestamps bool, utc bool, severity, search, taskID, executionID string) {
	client := core.GetClient()
	fetcher := monitor.NewLogFetcher(client, workspace, resourceType, resourceName, startTime, endTime, severity, search, taskID, executionID)
	logs, err := fetcher.FetchLogs()
	if err != nil {
		core.PrintError("logs", err)
		core.ExitWithError(err)
	}

	// Check if no logs were retrieved
	if len(logs) == 0 {
		fmt.Println("No logs found for the specified time range and filters.")
		return
	}

	// Print logs with timestamps
	for _, log := range logs {
		fmt.Println(formatLogOutput(log, noTimestamps, utc))
	}
}

// followLogs follows logs in real-time
func followLogs(workspace, resourceType, resourceName string, startTime time.Time, noTimestamps bool, utc bool, severity, search, taskID, executionID string) {
	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	client := core.GetClient()
	follower := monitor.NewLogFollower(client, workspace, resourceType, resourceName, startTime, severity, search, taskID, executionID,
		func(logEntry monitor.LogEntry) {
			fmt.Println(formatLogOutput(logEntry, noTimestamps, utc))
		},
		func(err error) {
			core.PrintWarning(fmt.Sprintf("Warning: %v\n", err))
		},
		func(info string) {
			core.PrintInfo(info)
		},
	)

	follower.Start()

	// Wait for interrupt signal
	<-sigChan
	follower.Stop()
	fmt.Println("\nStopped following logs.")
}

package monitor

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
)

// BuildLogWatcher watches build logs for a resource
type BuildLogWatcher struct {
	client       *blaxel.Client
	workspace    string
	resourceType string
	resourceName string
	onLog        func(string)
	ctx          context.Context
	cancel       context.CancelFunc
	seenLogs     map[string]bool // Track logs we've already shown
	mu           sync.Mutex
	startAt      time.Time
}

// NewBuildLogWatcher creates a new build log watcher
func NewBuildLogWatcher(client *blaxel.Client, workspace, resourceType, resourceName string, onLog func(string)) *BuildLogWatcher {
	ctx, cancel := context.WithCancel(context.Background())
	return &BuildLogWatcher{
		client:       client,
		workspace:    workspace,
		resourceType: resourceType,
		resourceName: resourceName,
		onLog:        onLog,
		ctx:          ctx,
		cancel:       cancel,
		seenLogs:     make(map[string]bool),
	}
}

func pluralizeResourceType(resourceType string) string {
	rt := strings.ToLower(resourceType)
	for _, res := range core.GetResources() {
		if strings.ToLower(res.Singular) == rt {
			return res.Plural
		}
	}
	return core.Pluralize(rt)
}

// Start begins watching build logs
func (w *BuildLogWatcher) Start() {
	// Record the exact start time to avoid fetching logs before watcher begins
	w.startAt = time.Now().UTC()
	go w.watchLogs()
}

// Stop stops watching build logs
func (w *BuildLogWatcher) Stop() {
	if w.cancel != nil {
		w.cancel()
	}
}

func (w *BuildLogWatcher) watchLogs() {
	// Initial delay to allow build to start
	time.Sleep(200 * time.Millisecond)

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	failureCount := 0
	maxFailures := 5

	attempt := 0
	for {
		select {
		case <-w.ctx.Done():
			return
		case <-ticker.C:
			attempt++
			// Fetch all logs each time - we'll deduplicate locally
			logs, err := w.fetchBuildLogs(0)
			if err != nil {
				failureCount++
				if failureCount >= maxFailures {
					w.onLog(fmt.Sprintf("Error: Failed to fetch logs after %d attempts: %v", maxFailures, err))
					return
				}
				// Log error but continue trying
				if failureCount == 1 {
					w.onLog(fmt.Sprintf("Warning: Error fetching logs: %v", err))
				}
				continue
			}

			// Reset failure count on success
			failureCount = 0

			// Process new log lines
			for _, log := range logs {
				w.onLog(log)
			}
		}
	}
}

func (w *BuildLogWatcher) fetchBuildLogs(offset int) ([]string, error) {
	// Calculate time window: from watcher start time to a bit in the future
	start := w.startAt.Format("2006-01-02T15:04:05")
	end := w.startAt.Add(15 * time.Minute).Format("2006-01-02T15:04:05")

	// Build query options
	queryOpts := []option.RequestOption{
		option.WithQuery("start", start),
		option.WithQuery("end", end),
		option.WithQuery("resourceType", pluralizeResourceType(w.resourceType)),
		option.WithQuery("workloadIds", w.resourceName),
		option.WithQuery("type", "all"),
		option.WithQuery("traceId", ""),
		option.WithQuery("limit", "1000"),
		option.WithQuery("offset", fmt.Sprintf("%d", offset)),
		option.WithQuery("severity", "all,UNKNOWN,TRACE,DEBUG,INFO,WARNING,ERROR,FATAL"),
		option.WithQuery("search", ""),
		option.WithQuery("taskId", ""),
		option.WithQuery("executionId", ""),
		option.WithQuery("interval", "60"),
		option.WithQuery("workspace", w.workspace),
	}

	// Parse the response
	var response map[string]struct {
		Logs []struct {
			Timestamp string `json:"timestamp"`
			Message   string `json:"message"`
			Severity  int    `json:"severity"`
			TraceID   string `json:"trace_id"`
		} `json:"logs"`
		TotalCount int `json:"totalCount"`
	}

	// Use the SDK client which handles auth middleware automatically
	err := w.client.Get(w.ctx, "/observability/logs", nil, &response, queryOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch logs: %w", err)
	}

	_, ok := response[w.resourceName]
	if !ok {
		return nil, fmt.Errorf("resource %s not found", w.resourceName)
	}
	// Extract messages from the logs
	var messages []string
	if resourceData, ok := response[w.resourceName]; ok {
		// Return logs with deduplication check
		w.mu.Lock()
		defer w.mu.Unlock()

		for _, log := range resourceData.Logs {
			// Use timestamp + message as unique key
			key := fmt.Sprintf("%s:%s", log.Timestamp, log.Message)
			if !w.seenLogs[key] {
				w.seenLogs[key] = true
				messages = append(messages, log.Message)
			}
		}
	}

	return messages, nil
}

// LogEntry represents a single log entry with timestamp
type LogEntry struct {
	Timestamp string
	Message   string
}

// LogFetcher fetches logs for a resource with custom time ranges
type LogFetcher struct {
	client       *blaxel.Client
	workspace    string
	resourceType string
	resourceName string
	startTime    time.Time
	endTime      time.Time
	severity     string
	search       string
	taskID       string
	executionID  string
}

// NewLogFetcher creates a new log fetcher
func NewLogFetcher(client *blaxel.Client, workspace, resourceType, resourceName string, startTime, endTime time.Time, severity, search, taskID, executionID string) *LogFetcher {
	return &LogFetcher{
		client:       client,
		workspace:    workspace,
		resourceType: resourceType,
		resourceName: resourceName,
		startTime:    startTime,
		endTime:      endTime,
		severity:     severity,
		search:       search,
		taskID:       taskID,
		executionID:  executionID,
	}
}

// FetchLogs fetches logs for the configured time range
func (lf *LogFetcher) FetchLogs() ([]LogEntry, error) {
	return lf.fetchLogsFromAPI(context.Background(), 0)
}

// fetchLogsFromAPI fetches logs from the Blaxel API
func (lf *LogFetcher) fetchLogsFromAPI(ctx context.Context, offset int) ([]LogEntry, error) {
	// Format times in UTC (matching the curl format)
	start := lf.startTime.UTC().Format("2006-01-02T15:04:05")
	end := lf.endTime.UTC().Format("2006-01-02T15:04:05")

	// Set severity - use user-provided or default to all
	severityFilter := lf.severity
	if severityFilter == "" {
		severityFilter = "FATAL,ERROR,WARNING,INFO,DEBUG,TRACE,UNKNOWN"
	}

	// Build query options
	queryOpts := []option.RequestOption{
		option.WithQuery("start", start),
		option.WithQuery("end", end),
		option.WithQuery("resourceType", pluralizeResourceType(lf.resourceType)),
		option.WithQuery("workloadIds", lf.resourceName),
		option.WithQuery("type", "all"),
		option.WithQuery("traceId", ""),
		option.WithQuery("limit", "1000"),
		option.WithQuery("offset", fmt.Sprintf("%d", offset)),
		option.WithQuery("severity", severityFilter),
		option.WithQuery("search", lf.search),
		option.WithQuery("taskId", lf.taskID),
		option.WithQuery("executionId", lf.executionID),
		option.WithQuery("interval", "60"),
		option.WithQuery("workspace", lf.workspace),
	}

	// Parse the response
	var response map[string]struct {
		Logs []struct {
			Timestamp string `json:"timestamp"`
			Message   string `json:"message"`
			Severity  int    `json:"severity"`
			TraceID   string `json:"trace_id"`
		} `json:"logs"`
		TotalCount int `json:"totalCount"`
	}

	// Use the SDK client which handles auth middleware automatically
	err := lf.client.Get(ctx, "/observability/logs", nil, &response, queryOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch logs: %w", err)
	}

	// Extract log entries from the specific resource
	var logEntries []LogEntry
	if resourceData, ok := response[lf.resourceName]; ok {
		for _, log := range resourceData.Logs {
			logEntries = append(logEntries, LogEntry{
				Timestamp: log.Timestamp,
				Message:   log.Message,
			})
		}
	}

	// Reverse the order so logs are chronological (oldest first)
	// API returns newest first, but we want to display oldest first
	for i, j := 0, len(logEntries)-1; i < j; i, j = i+1, j-1 {
		logEntries[i], logEntries[j] = logEntries[j], logEntries[i]
	}

	return logEntries, nil
}

// LogFollower follows logs in real-time
type LogFollower struct {
	client        *blaxel.Client
	workspace     string
	resourceType  string
	resourceName  string
	startTime     time.Time
	severity      string
	search        string
	taskID        string
	executionID   string
	onLog         func(LogEntry)
	onError       func(error)
	onInfo        func(string)
	ctx           context.Context
	cancel        context.CancelFunc
	seenLogs      map[string]bool
	mu            sync.Mutex
	errorReported bool // Track if we've already reported an error to avoid spam
}

// NewLogFollower creates a new log follower
func NewLogFollower(client *blaxel.Client, workspace, resourceType, resourceName string, startTime time.Time, severity, search, taskID, executionID string, onLog func(LogEntry), onError func(error), onInfo func(string)) *LogFollower {
	ctx, cancel := context.WithCancel(context.Background())
	return &LogFollower{
		client:       client,
		workspace:    workspace,
		resourceType: resourceType,
		resourceName: resourceName,
		startTime:    startTime,
		severity:     severity,
		search:       search,
		taskID:       taskID,
		executionID:  executionID,
		onLog:        onLog,
		onError:      onError,
		onInfo:       onInfo,
		ctx:          ctx,
		cancel:       cancel,
		seenLogs:     make(map[string]bool),
	}
}

// Start begins following logs
func (lf *LogFollower) Start() {
	go lf.followLogs()
}

// Stop stops following logs
func (lf *LogFollower) Stop() {
	if lf.cancel != nil {
		lf.cancel()
	}
}

func (lf *LogFollower) followLogs() {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	// Initial fetch - try to find logs, going back in time if needed
	// Use exponential backoff: 1h, 2h, 4h, 8h, 16h (up to 24h max)
	now := time.Now().UTC()
	futureTime := now.Add(24 * time.Hour)
	maxLookback := 24 * time.Hour
	lookbackStep := 1 * time.Hour

	currentStartTime := lf.startTime
	foundLogs := false

	for !foundLogs {
		// Check if we've exceeded the maximum lookback
		if now.Sub(currentStartTime) > maxLookback {
			if lf.onInfo != nil {
				lf.onInfo("No logs found in the last 24 hours. Waiting for new logs...")
			}
			break
		}

		fetcher := NewLogFetcher(lf.client, lf.workspace, lf.resourceType, lf.resourceName, currentStartTime, futureTime, lf.severity, lf.search, lf.taskID, lf.executionID)
		logs, err := fetcher.FetchLogs()
		if err != nil {
			// Report error on initial fetch
			if lf.onError != nil {
				lf.onError(fmt.Errorf("failed to fetch logs: %w", err))
			}
			lf.errorReported = true
			break
		}

		if len(logs) > 0 {
			foundLogs = true
			lf.mu.Lock()
			for _, log := range logs {
				// Use timestamp + message as unique key for deduplication
				key := fmt.Sprintf("%s:%s", log.Timestamp, log.Message)
				lf.onLog(log)
				lf.seenLogs[key] = true
			}
			lf.mu.Unlock()
		} else {
			// Go back by current step, then double the step (exponential backoff)
			currentStartTime = currentStartTime.Add(-lookbackStep)
			lookbackStep *= 2
			// Cap at remaining time to max lookback
			if lookbackStep > maxLookback {
				lookbackStep = maxLookback
			}
		}
	}

	// Set start time for subsequent fetches to current time minus a buffer
	// We need a large buffer (30s) because logs have a delay in appearing in the observability system
	lastFetchTime := time.Now().UTC().Add(-60 * time.Second)

	for {
		select {
		case <-lf.ctx.Done():
			return
		case <-ticker.C:
			// Fetch logs from last fetch time to a future time
			// This ensures we get all new logs without an artificial cutoff
			currentTime := time.Now().UTC()
			futureTime := currentTime.Add(24 * time.Hour)

			fetcher := NewLogFetcher(lf.client, lf.workspace, lf.resourceType, lf.resourceName, lastFetchTime, futureTime, lf.severity, lf.search, lf.taskID, lf.executionID)
			logs, err := fetcher.FetchLogs()
			if err != nil {
				// Report error only once to avoid spam
				if !lf.errorReported && lf.onError != nil {
					lf.onError(fmt.Errorf("failed to fetch logs: %w", err))
					lf.errorReported = true
				}
				// Continue on error but don't update last fetch time
				continue
			}

			// Reset error flag on successful fetch
			lf.errorReported = false

			lf.mu.Lock()
			for _, log := range logs {
				// Use timestamp + message as unique key for deduplication
				key := fmt.Sprintf("%s:%s", log.Timestamp, log.Message)
				if !lf.seenLogs[key] {
					lf.onLog(log)
					lf.seenLogs[key] = true
				}
			}
			lf.mu.Unlock()

			// Update last fetch time - keep a 30 second overlap to catch logs at boundaries
			// This large overlap accounts for delays in logs appearing in the observability system
			lastFetchTime = currentTime.Add(-30 * time.Second)
		}
	}
}

// PluralizeResourceType converts singular resource types to plural
func PluralizeResourceType(resourceType string) string {
	return pluralizeResourceType(resourceType)
}

// StreamBuildLogs streams build logs from an HTTP response
func StreamBuildLogs(resp *http.Response, onLog func(string)) error {
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Handle Server-Sent Events format
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				return nil
			}
			onLog(data)
		} else {
			onLog(line)
		}
	}
}

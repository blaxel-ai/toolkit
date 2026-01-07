package monitor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	blaxel "github.com/stainless-sdks/blaxel-go"
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

	workspace := w.workspace

	// Build URL with proper encoding
	// Create URL with query parameters
	u, err := url.Parse(blaxel.BuildObservabilityLogsURL())
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Add query parameters
	q := u.Query()
	q.Set("start", start)
	q.Set("end", end)
	q.Set("resourceType", pluralizeResourceType(w.resourceType))
	q.Set("workloadIds", w.resourceName)
	q.Set("type", "all")
	q.Set("traceId", "")
	q.Set("limit", "1000")
	q.Set("offset", fmt.Sprintf("%d", offset))
	q.Set("severity", "all,UNKNOWN,TRACE,DEBUG,INFO,WARNING,ERROR,FATAL")
	q.Set("search", "")
	q.Set("taskId", "")
	q.Set("executionId", "")
	q.Set("interval", "60")
	q.Set("workspace", workspace)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	credentials, _ := blaxel.LoadCredentials(workspace)
	if credentials.IsValid() {
		if credentials.AccessToken != "" {
			req.Header.Set("X-Blaxel-Authorization", fmt.Sprintf("Bearer %s", credentials.AccessToken))
		}
		if credentials.APIKey != "" {
			req.Header.Set("X-Blaxel-Authorization", fmt.Sprintf("Bearer %s", credentials.APIKey))
		}
		if workspace != "" {
			req.Header.Set("X-Blaxel-Workspace", workspace)
		}
	}

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		w.onLog(fmt.Sprintf("[response] http error: %v", err))
		return nil, fmt.Errorf("failed to fetch logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
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

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
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
func NewLogFetcher(workspace, resourceType, resourceName string, startTime, endTime time.Time, severity, search, taskID, executionID string) *LogFetcher {
	return &LogFetcher{
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
	return lf.fetchLogsFromAPI(0)
}

// fetchLogsFromAPI fetches logs from the Blaxel API
func (lf *LogFetcher) fetchLogsFromAPI(offset int) ([]LogEntry, error) {
	// Create URL with query parameters
	u, err := url.Parse(blaxel.BuildObservabilityLogsURL())
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Format times in UTC (matching the curl format)
	start := lf.startTime.UTC().Format("2006-01-02T15:04:05")
	end := lf.endTime.UTC().Format("2006-01-02T15:04:05")

	// Add query parameters
	q := u.Query()
	q.Set("start", start)
	q.Set("end", end)
	q.Set("resourceType", pluralizeResourceType(lf.resourceType))
	q.Set("workloadIds", lf.resourceName)
	q.Set("type", "all")
	q.Set("traceId", "")
	q.Set("limit", "1000")
	q.Set("offset", fmt.Sprintf("%d", offset))

	// Set severity - use user-provided or default to all
	severityFilter := lf.severity
	if severityFilter == "" {
		severityFilter = "FATAL,ERROR,WARNING,INFO,DEBUG,TRACE,UNKNOWN"
	}
	q.Set("severity", severityFilter)

	// Set search filter
	q.Set("search", lf.search)

	// Set job-specific filters
	q.Set("taskId", lf.taskID)
	q.Set("executionId", lf.executionID)

	// Set interval (required by API)
	q.Set("interval", "60")

	q.Set("workspace", lf.workspace)
	u.RawQuery = q.Encode()

	// Create request
	req, err := http.NewRequestWithContext(context.Background(), "GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	credentials, _ := blaxel.LoadCredentials(lf.workspace)
	if !credentials.IsValid() {
		return nil, fmt.Errorf("no valid credentials found for workspace '%s'", lf.workspace)
	}

	// Set authentication headers directly
	if credentials.AccessToken != "" {
		req.Header.Set("X-Blaxel-Authorization", fmt.Sprintf("Bearer %s", credentials.AccessToken))
	}
	if credentials.APIKey != "" {
		req.Header.Set("X-Blaxel-Authorization", fmt.Sprintf("Bearer %s", credentials.APIKey))
	}
	if lf.workspace != "" {
		req.Header.Set("X-Blaxel-Workspace", lf.workspace)
	}

	// Make the request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch logs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(body))
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

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
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
	workspace    string
	resourceType string
	resourceName string
	startTime    time.Time
	severity     string
	search       string
	taskID       string
	executionID  string
	onLog        func(LogEntry)
	ctx          context.Context
	cancel       context.CancelFunc
	seenLogs     map[string]bool
	mu           sync.Mutex
}

// NewLogFollower creates a new log follower
func NewLogFollower(workspace, resourceType, resourceName string, startTime time.Time, severity, search, taskID, executionID string, onLog func(LogEntry)) *LogFollower {
	ctx, cancel := context.WithCancel(context.Background())
	return &LogFollower{
		workspace:    workspace,
		resourceType: resourceType,
		resourceName: resourceName,
		startTime:    startTime,
		severity:     severity,
		search:       search,
		taskID:       taskID,
		executionID:  executionID,
		onLog:        onLog,
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

	// Initial fetch - get existing logs from the specified start time
	// Use a far future end time to get all available logs
	futureTime := time.Now().UTC().Add(24 * time.Hour)
	fetcher := NewLogFetcher(lf.workspace, lf.resourceType, lf.resourceName, lf.startTime, futureTime, lf.severity, lf.search, lf.taskID, lf.executionID)
	logs, err := fetcher.FetchLogs()
	if err == nil {
		lf.mu.Lock()
		for _, log := range logs {
			// Use timestamp + message as unique key for deduplication
			key := fmt.Sprintf("%s:%s", log.Timestamp, log.Message)
			lf.onLog(log)
			lf.seenLogs[key] = true
		}
		lf.mu.Unlock()
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

			fetcher := NewLogFetcher(lf.workspace, lf.resourceType, lf.resourceName, lastFetchTime, futureTime, lf.severity, lf.search, lf.taskID, lf.executionID)
			logs, err := fetcher.FetchLogs()
			if err != nil {
				// Continue on error but don't update last fetch time
				continue
			}

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

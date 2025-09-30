package monitor

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
)

// BuildLogWatcher watches build logs for a resource
type BuildLogWatcher struct {
	client       *sdk.ClientWithResponses
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
func NewBuildLogWatcher(client *sdk.ClientWithResponses, workspace, resourceType, resourceName string, onLog func(string)) *BuildLogWatcher {
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
	baseURL := strings.TrimSuffix(os.Getenv("BL_API_URL"), "/")
	if baseURL == "" {
		baseURL = "https://api.blaxel.ai"
	}

	// Create URL with query parameters
	u, err := url.Parse(baseURL + "/v0/observability/logs")
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
	q.Set("interval", "14400")
	q.Set("workspace", workspace)
	u.RawQuery = q.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add authentication headers
	credentials := sdk.LoadCredentials(workspace)
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

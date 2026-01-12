package monitor

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPluralizeResourceType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"agent to agents", "agent", "agents"},
		{"function to functions", "function", "functions"},
		{"model to models", "model", "models"},
		{"job to jobs", "job", "jobs"},
		{"sandbox to sandboxes", "sandbox", "sandboxes"},
		{"policy to policies", "policy", "policies"},
		{"volume to volumes", "volume", "volumes"},
		{"Agent case insensitive", "Agent", "agents"},
		{"FUNCTION uppercase", "FUNCTION", "functions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PluralizeResourceType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestLogEntry(t *testing.T) {
	entry := LogEntry{
		Timestamp: "2024-01-15T10:30:00Z",
		Message:   "Test log message",
	}

	assert.Equal(t, "2024-01-15T10:30:00Z", entry.Timestamp)
	assert.Equal(t, "Test log message", entry.Message)
}

func TestNewLogFetcher(t *testing.T) {
	startTime := time.Now().Add(-1 * time.Hour)
	endTime := time.Now()

	fetcher := NewLogFetcher(nil, "test-workspace", "agent", "my-agent", startTime, endTime, "INFO", "search-term", "task-123", "exec-456")

	assert.Equal(t, "test-workspace", fetcher.workspace)
	assert.Equal(t, "agent", fetcher.resourceType)
	assert.Equal(t, "my-agent", fetcher.resourceName)
	assert.Equal(t, startTime, fetcher.startTime)
	assert.Equal(t, endTime, fetcher.endTime)
	assert.Equal(t, "INFO", fetcher.severity)
	assert.Equal(t, "search-term", fetcher.search)
	assert.Equal(t, "task-123", fetcher.taskID)
	assert.Equal(t, "exec-456", fetcher.executionID)
}

func TestNewBuildLogWatcher(t *testing.T) {
	var receivedLog string
	onLog := func(log string) {
		receivedLog = log
	}

	watcher := NewBuildLogWatcher(nil, "test-workspace", "function", "my-function", onLog)

	assert.NotNil(t, watcher)
	assert.Equal(t, "test-workspace", watcher.workspace)
	assert.Equal(t, "function", watcher.resourceType)
	assert.Equal(t, "my-function", watcher.resourceName)
	assert.NotNil(t, watcher.seenLogs)
	assert.NotNil(t, watcher.ctx)
	assert.NotNil(t, watcher.cancel)

	// Test the onLog callback
	watcher.onLog("test message")
	assert.Equal(t, "test message", receivedLog)
}

func TestBuildLogWatcherStop(t *testing.T) {
	watcher := NewBuildLogWatcher(nil, "test", "agent", "test", func(s string) {})

	// Start should set startAt
	watcher.Start()
	assert.False(t, watcher.startAt.IsZero())

	// Stop should not panic
	watcher.Stop()
}

func TestNewLogFollower(t *testing.T) {
	startTime := time.Now()
	var logEntries []LogEntry
	var errors []error
	var infoMessages []string

	onLog := func(e LogEntry) { logEntries = append(logEntries, e) }
	onError := func(e error) { errors = append(errors, e) }
	onInfo := func(s string) { infoMessages = append(infoMessages, s) }

	follower := NewLogFollower(nil, "test-workspace", "job", "my-job", startTime, "ERROR", "error", "task1", "exec1", onLog, onError, onInfo)

	assert.NotNil(t, follower)
	assert.Equal(t, "test-workspace", follower.workspace)
	assert.Equal(t, "job", follower.resourceType)
	assert.Equal(t, "my-job", follower.resourceName)
	assert.Equal(t, startTime, follower.startTime)
	assert.Equal(t, "ERROR", follower.severity)
	assert.Equal(t, "error", follower.search)
	assert.Equal(t, "task1", follower.taskID)
	assert.Equal(t, "exec1", follower.executionID)
	assert.NotNil(t, follower.seenLogs)
}

func TestLogFollowerStop(t *testing.T) {
	follower := NewLogFollower(nil, "test", "agent", "test", time.Now(), "", "", "", "", nil, nil, nil)

	// Stop should not panic even if Start wasn't called
	follower.Stop()
}

func TestStreamBuildLogs(t *testing.T) {
	t.Run("streams regular lines", func(t *testing.T) {
		body := io.NopCloser(strings.NewReader("line1\nline2\nline3\n"))
		resp := &http.Response{Body: body}

		var logs []string
		err := StreamBuildLogs(resp, func(log string) {
			logs = append(logs, log)
		})

		assert.NoError(t, err)
		assert.Equal(t, []string{"line1", "line2", "line3"}, logs)
	})

	t.Run("handles SSE format", func(t *testing.T) {
		body := io.NopCloser(strings.NewReader("data: log message 1\ndata: log message 2\n"))
		resp := &http.Response{Body: body}

		var logs []string
		err := StreamBuildLogs(resp, func(log string) {
			logs = append(logs, log)
		})

		assert.NoError(t, err)
		assert.Equal(t, []string{"log message 1", "log message 2"}, logs)
	})

	t.Run("handles [DONE] signal", func(t *testing.T) {
		body := io.NopCloser(strings.NewReader("data: log message\ndata: [DONE]\ndata: should not appear\n"))
		resp := &http.Response{Body: body}

		var logs []string
		err := StreamBuildLogs(resp, func(log string) {
			logs = append(logs, log)
		})

		assert.NoError(t, err)
		// Should only have the first message before [DONE]
		assert.Equal(t, []string{"log message"}, logs)
	})

	t.Run("skips empty lines", func(t *testing.T) {
		body := io.NopCloser(strings.NewReader("line1\n\n\nline2\n"))
		resp := &http.Response{Body: body}

		var logs []string
		err := StreamBuildLogs(resp, func(log string) {
			logs = append(logs, log)
		})

		assert.NoError(t, err)
		assert.Equal(t, []string{"line1", "line2"}, logs)
	})

	t.Run("handles empty body", func(t *testing.T) {
		body := io.NopCloser(strings.NewReader(""))
		resp := &http.Response{Body: body}

		var logs []string
		err := StreamBuildLogs(resp, func(log string) {
			logs = append(logs, log)
		})

		assert.NoError(t, err)
		assert.Empty(t, logs)
	})
}

func TestInternalPluralizeResourceType(t *testing.T) {
	// Test the internal function behavior
	result := pluralizeResourceType("agent")
	assert.Equal(t, "agents", result)

	// Test with unknown type (should use core.Pluralize fallback)
	result = pluralizeResourceType("unknown-type")
	assert.Contains(t, result, "unknown-type")
}

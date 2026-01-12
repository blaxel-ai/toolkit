package cli

import (
	"testing"
	"time"

	"github.com/blaxel-ai/toolkit/cli/monitor"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeResourceType(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    string
		expectError bool
	}{
		// Sandboxes
		{"sandbox", "sandbox", "sandbox", false},
		{"sbx", "sbx", "sandbox", false},
		{"sandboxes", "sandboxes", "sandbox", false},

		// Jobs
		{"job", "job", "job", false},
		{"j", "j", "job", false},
		{"jb", "jb", "job", false},
		{"jobs", "jobs", "job", false},

		// Agents
		{"agent", "agent", "agent", false},
		{"ag", "ag", "agent", false},
		{"agents", "agents", "agent", false},

		// Functions
		{"function", "function", "function", false},
		{"fn", "fn", "function", false},
		{"mcp", "mcp", "function", false},
		{"mcps", "mcps", "function", false},
		{"functions", "functions", "function", false},

		// Case insensitive
		{"uppercase", "SANDBOX", "sandbox", false},
		{"mixed case", "SandBox", "sandbox", false},

		// Invalid
		{"invalid type", "unknown", "", true},
		{"empty", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := normalizeResourceType(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseDuration(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    time.Duration
		expectError bool
	}{
		{"days", "3d", 3 * 24 * time.Hour, false},
		{"hours", "2h", 2 * time.Hour, false},
		{"minutes", "30m", 30 * time.Minute, false},
		{"seconds", "45s", 45 * time.Second, false},
		{"single day", "1d", 24 * time.Hour, false},
		{"large hours", "72h", 72 * time.Hour, false},

		// Invalid cases
		{"invalid format", "3x", 0, true},
		{"no number", "d", 0, true},
		{"empty", "", 0, true},
		{"missing unit", "123", 0, true},
		{"invalid chars", "abc", 0, true},
		{"negative", "-3d", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestParseTimeFlag(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expectError bool
		validate    func(t *testing.T, result time.Time)
	}{
		{
			name:        "RFC3339",
			input:       "2024-01-15T10:30:00Z",
			expectError: false,
			validate: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 15, result.Day())
				assert.Equal(t, 10, result.Hour())
			},
		},
		{
			name:        "datetime without timezone",
			input:       "2024-01-15T10:30:00",
			expectError: false,
			validate: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 15, result.Day())
			},
		},
		{
			name:        "datetime with space",
			input:       "2024-01-15 10:30:00",
			expectError: false,
			validate: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.January, result.Month())
			},
		},
		{
			name:        "date only",
			input:       "2024-01-15",
			expectError: false,
			validate: func(t *testing.T, result time.Time) {
				assert.Equal(t, 2024, result.Year())
				assert.Equal(t, time.January, result.Month())
				assert.Equal(t, 15, result.Day())
				// Should be set to noon UTC
				assert.Equal(t, 12, result.Hour())
			},
		},
		{
			name:        "invalid format",
			input:       "not-a-date",
			expectError: true,
		},
		{
			name:        "invalid year",
			input:       "20-01-15",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseTimeFlag(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, result)
				}
			}
		})
	}
}

func TestValidateTimeRange(t *testing.T) {
	tests := []struct {
		name        string
		start       time.Time
		end         time.Time
		expectError bool
	}{
		{
			name:        "valid 1 hour range",
			start:       time.Now().Add(-1 * time.Hour),
			end:         time.Now(),
			expectError: false,
		},
		{
			name:        "valid 2 day range",
			start:       time.Now().Add(-48 * time.Hour),
			end:         time.Now(),
			expectError: false,
		},
		{
			name:        "exceeds 3 days",
			start:       time.Now().Add(-100 * time.Hour),
			end:         time.Now(),
			expectError: true,
		},
		{
			name:        "start after end",
			start:       time.Now(),
			end:         time.Now().Add(-1 * time.Hour),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateTimeRange(tt.start, tt.end)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestFormatLogOutput(t *testing.T) {
	now := time.Now().UTC()
	timestamp := now.Format(time.RFC3339Nano)

	tests := []struct {
		name         string
		entry        monitor.LogEntry
		noTimestamps bool
		utc          bool
		validate     func(t *testing.T, result string)
	}{
		{
			name: "with timestamp",
			entry: monitor.LogEntry{
				Timestamp: timestamp,
				Message:   "test message",
			},
			noTimestamps: false,
			utc:          true,
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "test message")
				assert.Contains(t, result, "[")
				assert.Contains(t, result, "]")
			},
		},
		{
			name: "without timestamp",
			entry: monitor.LogEntry{
				Timestamp: timestamp,
				Message:   "test message",
			},
			noTimestamps: true,
			utc:          false,
			validate: func(t *testing.T, result string) {
				assert.Equal(t, "test message", result)
			},
		},
		{
			name: "invalid timestamp",
			entry: monitor.LogEntry{
				Timestamp: "invalid",
				Message:   "test message",
			},
			noTimestamps: false,
			utc:          true,
			validate: func(t *testing.T, result string) {
				assert.Contains(t, result, "test message")
				assert.Contains(t, result, "invalid")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLogOutput(tt.entry, tt.noTimestamps, tt.utc)
			if tt.validate != nil {
				tt.validate(t, result)
			}
		})
	}
}

func TestLogsCmd(t *testing.T) {
	cmd := LogsCmd()

	assert.Equal(t, "logs RESOURCE_TYPE RESOURCE_NAME", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags
	flag := cmd.Flags().Lookup("follow")
	assert.NotNil(t, flag)
	assert.Equal(t, "f", flag.Shorthand)

	periodFlag := cmd.Flags().Lookup("period")
	assert.NotNil(t, periodFlag)
	assert.Equal(t, "p", periodFlag.Shorthand)

	startFlag := cmd.Flags().Lookup("start")
	assert.NotNil(t, startFlag)

	endFlag := cmd.Flags().Lookup("end")
	assert.NotNil(t, endFlag)

	noTimestampsFlag := cmd.Flags().Lookup("no-timestamps")
	assert.NotNil(t, noTimestampsFlag)

	utcFlag := cmd.Flags().Lookup("utc")
	assert.NotNil(t, utcFlag)

	severityFlag := cmd.Flags().Lookup("severity")
	assert.NotNil(t, severityFlag)

	searchFlag := cmd.Flags().Lookup("search")
	assert.NotNil(t, searchFlag)

	taskIDFlag := cmd.Flags().Lookup("task-id")
	assert.NotNil(t, taskIDFlag)

	executionIDFlag := cmd.Flags().Lookup("execution-id")
	assert.NotNil(t, executionIDFlag)
}

func TestLogsCmdLongDescription(t *testing.T) {
	cmd := LogsCmd()

	// Verify long description contains key information
	assert.Contains(t, cmd.Long, "sandboxes")
	assert.Contains(t, cmd.Long, "jobs")
	assert.Contains(t, cmd.Long, "agents")
	assert.Contains(t, cmd.Long, "functions")
	assert.Contains(t, cmd.Long, "Duration units")
}

func TestLogsCmdExamples(t *testing.T) {
	cmd := LogsCmd()

	// Verify examples exist
	assert.NotEmpty(t, cmd.Long)
	assert.Contains(t, cmd.Long, "bl logs sandbox")
	assert.Contains(t, cmd.Long, "--follow")
	assert.Contains(t, cmd.Long, "--period")
}

func TestNormalizeResourceTypeAllAliases(t *testing.T) {
	// Test all documented aliases
	tests := []struct {
		input    string
		expected string
	}{
		// Sandboxes
		{"sandbox", "sandbox"},
		{"sbx", "sandbox"},
		{"sandboxes", "sandbox"},

		// Jobs
		{"job", "job"},
		{"j", "job"},
		{"jb", "job"},
		{"jobs", "job"},

		// Agents
		{"agent", "agent"},
		{"ag", "agent"},
		{"agents", "agent"},

		// Functions
		{"function", "function"},
		{"fn", "function"},
		{"mcp", "function"},
		{"mcps", "function"},
		{"functions", "function"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := normalizeResourceType(tt.input)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestParseDurationBoundaryValues(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		expected    time.Duration
		expectError bool
	}{
		{"zero days", "0d", 0, false},
		{"zero hours", "0h", 0, false},
		{"large days", "365d", 365 * 24 * time.Hour, false},
		{"large hours", "1000h", 1000 * time.Hour, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDuration(tt.input)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}
}

func TestFormatLogOutputVariants(t *testing.T) {
	now := time.Now().UTC()
	timestamp := now.Format(time.RFC3339Nano)

	tests := []struct {
		name     string
		entry    monitor.LogEntry
		noTs     bool
		utc      bool
		contains []string
	}{
		{
			name:     "normal output",
			entry:    monitor.LogEntry{Timestamp: timestamp, Message: "INFO: Server started"},
			noTs:     false,
			utc:      true,
			contains: []string{"INFO: Server started", "["},
		},
		{
			name:     "multiline message",
			entry:    monitor.LogEntry{Timestamp: timestamp, Message: "Line 1\nLine 2\nLine 3"},
			noTs:     true,
			utc:      false,
			contains: []string{"Line 1\nLine 2\nLine 3"},
		},
		{
			name:     "special characters",
			entry:    monitor.LogEntry{Timestamp: timestamp, Message: "Error: <html>&nbsp;</html>"},
			noTs:     true,
			utc:      false,
			contains: []string{"<html>", "&nbsp;"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLogOutput(tt.entry, tt.noTs, tt.utc)
			for _, expected := range tt.contains {
				assert.Contains(t, result, expected)
			}
		})
	}
}

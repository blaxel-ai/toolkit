package test

import (
	"fmt"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
)

func TestConfig(t *testing.T) {
	core.ReadConfigToml(".", true)
	config := core.GetConfig()
	fmt.Println(config.Runtime)
}

func TestParseDurationToSeconds(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
		wantErr  bool
	}{
		// Valid cases - seconds
		{"30 seconds", "30s", 30, false},
		{"1 second", "1s", 1, false},

		// Valid cases - minutes
		{"5 minutes", "5m", 300, false},
		{"1 minute", "1m", 60, false},
		{"60 minutes", "60m", 3600, false},

		// Valid cases - hours
		{"1 hour", "1h", 3600, false},
		{"2 hours", "2h", 7200, false},
		{"24 hours", "24h", 86400, false},

		// Valid cases - days
		{"1 day", "1d", 86400, false},
		{"7 days", "7d", 604800, false},

		// Valid cases - weeks
		{"1 week", "1w", 604800, false},
		{"2 weeks", "2w", 1209600, false},

		// Valid cases - plain integers (seconds)
		{"plain integer 900", "900", 900, false},
		{"plain integer 3600", "3600", 3600, false},
		{"plain integer 0", "0", 0, false},

		// Valid cases - uppercase should work
		{"uppercase 1H", "1H", 3600, false},
		{"uppercase 30M", "30M", 1800, false},

		// Valid cases - with whitespace
		{"with spaces", "  1h  ", 3600, false},

		// Invalid cases
		{"empty string", "", 0, true},
		{"invalid format", "1x", 0, true},
		{"no number", "h", 0, true},
		{"negative not supported", "-1h", 0, true},
		{"decimal not supported", "1.5h", 0, true},
		{"multiple units", "1h30m", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := core.ParseDurationToSeconds(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ParseDurationToSeconds(%q) expected error, got nil", tt.input)
				}
				return
			}

			if err != nil {
				t.Errorf("ParseDurationToSeconds(%q) unexpected error: %v", tt.input, err)
				return
			}

			if result != tt.expected {
				t.Errorf("ParseDurationToSeconds(%q) = %d, expected %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestConvertRuntimeTimeouts(t *testing.T) {
	tests := []struct {
		name     string
		runtime  map[string]interface{}
		expected int
		wantErr  bool
	}{
		{
			name:     "string timeout 1h",
			runtime:  map[string]interface{}{"timeout": "1h"},
			expected: 3600,
			wantErr:  false,
		},
		{
			name:     "string timeout 30m",
			runtime:  map[string]interface{}{"timeout": "30m"},
			expected: 1800,
			wantErr:  false,
		},
		{
			name:     "integer timeout unchanged",
			runtime:  map[string]interface{}{"timeout": 900},
			expected: 900,
			wantErr:  false,
		},
		{
			name:     "float64 timeout unchanged",
			runtime:  map[string]interface{}{"timeout": float64(900)},
			expected: 900,
			wantErr:  false,
		},
		{
			name:     "no timeout field",
			runtime:  map[string]interface{}{"memory": 4096},
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "nil runtime",
			runtime:  nil,
			expected: 0,
			wantErr:  false,
		},
		{
			name:     "invalid timeout format",
			runtime:  map[string]interface{}{"timeout": "invalid"},
			expected: 0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ConvertRuntimeTimeouts(tt.runtime)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ConvertRuntimeTimeouts() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ConvertRuntimeTimeouts() unexpected error: %v", err)
				return
			}

			if tt.runtime != nil && tt.expected != 0 {
				timeout, ok := tt.runtime["timeout"]
				if !ok {
					t.Errorf("ConvertRuntimeTimeouts() timeout field missing")
					return
				}
				var result int
				switch v := timeout.(type) {
				case int:
					result = v
				case float64:
					result = int(v)
				default:
					t.Errorf("ConvertRuntimeTimeouts() timeout has unexpected type: %T", timeout)
					return
				}
				if result != tt.expected {
					t.Errorf("ConvertRuntimeTimeouts() timeout = %d, expected %d", result, tt.expected)
				}
			}
		})
	}
}

func TestConvertTriggersTimeouts(t *testing.T) {
	tests := []struct {
		name     string
		triggers *[]map[string]interface{}
		expected []int // expected timeout values for each trigger
		wantErr  bool
	}{
		{
			name: "single trigger with string timeout",
			triggers: &[]map[string]interface{}{
				{"id": "trigger1", "type": "http-async", "timeout": "15m"},
			},
			expected: []int{900},
			wantErr:  false,
		},
		{
			name: "multiple triggers with different formats",
			triggers: &[]map[string]interface{}{
				{"id": "trigger1", "timeout": "1h"},
				{"id": "trigger2", "timeout": 300},
				{"id": "trigger3", "timeout": "5m"},
			},
			expected: []int{3600, 300, 300},
			wantErr:  false,
		},
		{
			name: "trigger with nested configuration timeout",
			triggers: &[]map[string]interface{}{
				{
					"id":   "trigger1",
					"type": "http-async",
					"configuration": map[string]interface{}{
						"timeout": "10m",
					},
				},
			},
			expected: []int{0}, // top-level timeout is 0, but configuration.timeout should be converted
			wantErr:  false,
		},
		{
			name: "trigger without timeout",
			triggers: &[]map[string]interface{}{
				{"id": "trigger1", "type": "http"},
			},
			expected: []int{0},
			wantErr:  false,
		},
		{
			name:     "nil triggers",
			triggers: nil,
			expected: nil,
			wantErr:  false,
		},
		{
			name: "invalid timeout format",
			triggers: &[]map[string]interface{}{
				{"id": "trigger1", "timeout": "invalid"},
			},
			expected: nil,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := core.ConvertTriggersTimeouts(tt.triggers)

			if tt.wantErr {
				if err == nil {
					t.Errorf("ConvertTriggersTimeouts() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("ConvertTriggersTimeouts() unexpected error: %v", err)
				return
			}

			if tt.triggers != nil && tt.expected != nil {
				for i, trigger := range *tt.triggers {
					if tt.expected[i] == 0 {
						continue // skip if no timeout expected
					}
					timeout, ok := trigger["timeout"]
					if !ok {
						t.Errorf("ConvertTriggersTimeouts() trigger[%d] timeout field missing", i)
						continue
					}
					var result int
					switch v := timeout.(type) {
					case int:
						result = v
					case float64:
						result = int(v)
					default:
						t.Errorf("ConvertTriggersTimeouts() trigger[%d] timeout has unexpected type: %T", i, timeout)
						continue
					}
					if result != tt.expected[i] {
						t.Errorf("ConvertTriggersTimeouts() trigger[%d] timeout = %d, expected %d", i, result, tt.expected[i])
					}
				}
			}
		})
	}
}

func TestConvertTriggersTimeouts_NestedConfiguration(t *testing.T) {
	triggers := &[]map[string]interface{}{
		{
			"id":   "async-trigger",
			"type": "http-async",
			"configuration": map[string]interface{}{
				"timeout": "30m",
				"path":    "/webhook",
			},
		},
	}

	err := core.ConvertTriggersTimeouts(triggers)
	if err != nil {
		t.Fatalf("ConvertTriggersTimeouts() unexpected error: %v", err)
	}

	config := (*triggers)[0]["configuration"].(map[string]interface{})
	timeout, ok := config["timeout"]
	if !ok {
		t.Fatal("ConvertTriggersTimeouts() configuration.timeout field missing")
	}

	if timeout != 1800 {
		t.Errorf("ConvertTriggersTimeouts() configuration.timeout = %v, expected 1800", timeout)
	}
}

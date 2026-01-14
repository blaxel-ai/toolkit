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

package test

import (
	"encoding/json"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig(t *testing.T) {
	core.ReadConfigToml(".", true)
	config := core.GetConfig()
	envs := (*config.Runtime)["envs"].([]map[string]interface{})
	ports := (*config.Runtime)["ports"].([]map[string]interface{})
	memory := (*config.Runtime)["memory"].(int64)
	assert.Equal(t, envs[0]["name"], "PLAYWRIGHT_BROWSERS_PATH")
	assert.Equal(t, envs[0]["value"], "/root/.cache/ms-playwright")
	assert.Equal(t, ports[0]["name"], "playwright")
	assert.Equal(t, ports[0]["target"], int64(3000))
	assert.Equal(t, ports[0]["protocol"], "tcp")
	assert.Equal(t, config.Name, "playwright-firefox")
	assert.Equal(t, config.Type, "sandbox")
	assert.Equal(t, memory, int64(8192))
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

		// Overflow protection cases - values exceeding 1 year max
		{"overflow seconds", "2147483647s", 0, true},    // int32 max
		{"overflow weeks", "35791394w", 0, true},        // would overflow int32
		{"overflow plain int", "999999999999", 0, true}, // exceeds max duration
		{"overflow minutes", "999999999m", 0, true},     // exceeds max
		{"overflow hours", "99999999h", 0, true},        // exceeds max
		{"overflow days", "9999999d", 0, true},          // exceeds max
		{"just over max weeks", "53w", 0, true},         // 53 weeks > 52 weeks max
		{"just over max days", "366d", 0, true},         // 366 days > 365 days max

		// Valid edge cases - at or near limits
		{"max valid weeks", "52w", 52 * 7 * 24 * 60 * 60, false},  // 52 weeks = 31,449,600s
		{"max valid days", "365d", 365 * 24 * 60 * 60, false},     // 365 days = 31,536,000s
		{"large but valid hours", "8760h", 8760 * 60 * 60, false}, // 8760h = 365 days
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

func TestEnvJSONSerialization(t *testing.T) {
	// Test that Env struct serializes to JSON correctly with proper field names
	env := core.Env{
		Name:  "TEST_VAR",
		Value: "test_value",
	}

	jsonData, err := json.Marshal(env)
	assert.NoError(t, err)

	// Verify the JSON has lowercase field names (from json tags)
	expectedJSON := `{"name":"TEST_VAR","value":"test_value"}`
	assert.JSONEq(t, expectedJSON, string(jsonData))

	// Verify we can unmarshal it back
	var unmarshaled core.Env
	err = json.Unmarshal(jsonData, &unmarshaled)
	assert.NoError(t, err)
	assert.Equal(t, env, unmarshaled)
}

func TestEnvSliceJSONSerialization(t *testing.T) {
	// Test that a slice of Env structs serializes correctly
	envs := []core.Env{
		{Name: "VAR1", Value: "value1"},
		{Name: "VAR2", Value: "value2"},
	}

	jsonData, err := json.Marshal(envs)
	assert.NoError(t, err)

	expectedJSON := `[{"name":"VAR1","value":"value1"},{"name":"VAR2","value":"value2"}]`
	assert.JSONEq(t, expectedJSON, string(jsonData))
}

func TestEnvInMapJSONSerialization(t *testing.T) {
	// Test that Env works correctly when embedded in a map (like runtime["envs"])
	// This is the actual use case in deploy.go
	runtime := make(map[string]interface{})
	runtime["envs"] = []core.Env{
		{Name: "API_KEY", Value: "secret123"},
		{Name: "DEBUG", Value: "true"},
	}
	runtime["memory"] = 4096

	jsonData, err := json.Marshal(runtime)
	assert.NoError(t, err)

	// Parse the JSON and verify structure
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	assert.NoError(t, err)

	// Verify envs is an array with correct field names
	envsArray := parsed["envs"].([]interface{})
	assert.Len(t, envsArray, 2)

	firstEnv := envsArray[0].(map[string]interface{})
	assert.Equal(t, "API_KEY", firstEnv["name"])
	assert.Equal(t, "secret123", firstEnv["value"])

	secondEnv := envsArray[1].(map[string]interface{})
	assert.Equal(t, "DEBUG", secondEnv["name"])
	assert.Equal(t, "true", secondEnv["value"])
}

func TestResultJSONSerialization(t *testing.T) {
	// Test that core.Result serializes correctly with all fields
	result := core.Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       "Agent",
		Metadata: map[string]interface{}{
			"name": "test-agent",
			"labels": map[string]interface{}{
				"x-blaxel-auto-generated": "true",
			},
		},
		Spec: map[string]interface{}{
			"runtime": map[string]interface{}{
				"envs": []core.Env{
					{Name: "ENV_VAR", Value: "env_value"},
				},
				"memory": 2048,
			},
		},
		Status: "deployed",
	}

	jsonData, err := json.Marshal(result)
	assert.NoError(t, err)

	// Parse and verify
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	assert.NoError(t, err)

	assert.Equal(t, "blaxel.ai/v1alpha1", parsed["apiVersion"])
	assert.Equal(t, "Agent", parsed["kind"])
	assert.Equal(t, "deployed", parsed["status"])

	// Verify metadata
	metadata := parsed["metadata"].(map[string]interface{})
	assert.Equal(t, "test-agent", metadata["name"])

	// Verify spec.runtime.envs has correct JSON structure
	spec := parsed["spec"].(map[string]interface{})
	runtime := spec["runtime"].(map[string]interface{})
	envs := runtime["envs"].([]interface{})
	assert.Len(t, envs, 1)

	env := envs[0].(map[string]interface{})
	assert.Equal(t, "ENV_VAR", env["name"])
	assert.Equal(t, "env_value", env["value"])
}

// TestConfigToAgentConversion verifies that the config correctly converts to a blaxel.Agent structure
// This simulates the flow: blaxel.toml -> Config -> Result -> JSON -> blaxel.AgentParam
func TestConfigToAgentConversion(t *testing.T) {
	// Simulate what GenerateDeployment does in deploy.go
	runtime := map[string]interface{}{
		"memory":     int64(4096),
		"generation": "mk3",
		"envs": []core.Env{
			{Name: "API_KEY", Value: "my-secret-key"},
			{Name: "DEBUG", Value: "true"},
			{Name: "DATABASE_URL", Value: "postgres://localhost/db"},
		},
	}

	triggers := []map[string]interface{}{
		{
			"type": "http",
			"configuration": map[string]interface{}{
				"path":               "/webhook",
				"authenticationType": "public",
			},
		},
	}

	// Create the Result structure (like deploy.go does)
	result := core.Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       "Agent",
		Metadata: map[string]interface{}{
			"name": "my-test-agent",
			"labels": map[string]interface{}{
				"x-blaxel-auto-generated": "true",
			},
		},
		Spec: map[string]interface{}{
			"runtime":  runtime,
			"triggers": triggers,
		},
	}

	// Marshal to JSON (like apply.go does before sending to API)
	jsonData, err := json.Marshal(result)
	require.NoError(t, err)

	// Verify we can unmarshal into blaxel.AgentParam (simulating what the SDK does)
	var agentParam blaxel.AgentParam
	err = json.Unmarshal(jsonData, &agentParam)
	require.NoError(t, err)

	// Verify the agent metadata (Name is a direct string, not Opt)
	assert.Equal(t, "my-test-agent", agentParam.Metadata.Name)

	// Verify the agent runtime (Memory is Opt[int64], Generation is AgentRuntimeGeneration)
	assert.Equal(t, int64(4096), agentParam.Spec.Runtime.Memory.Value)
	assert.Equal(t, blaxel.AgentRuntimeGenerationMk3, agentParam.Spec.Runtime.Generation)

	// Verify environment variables were correctly parsed
	require.Len(t, agentParam.Spec.Runtime.Envs, 3, "Expected 3 environment variables")

	// Check each env var (Name and Value are Opt[string])
	envMap := make(map[string]string)
	for _, env := range agentParam.Spec.Runtime.Envs {
		envMap[env.Name.Value] = env.Value.Value
	}

	assert.Equal(t, "my-secret-key", envMap["API_KEY"], "API_KEY env var should be set correctly")
	assert.Equal(t, "true", envMap["DEBUG"], "DEBUG env var should be set correctly")
	assert.Equal(t, "postgres://localhost/db", envMap["DATABASE_URL"], "DATABASE_URL env var should be set correctly")

	// Verify triggers (Type is TriggerType, not Opt)
	require.Len(t, agentParam.Spec.Triggers, 1, "Expected 1 trigger")
	assert.Equal(t, blaxel.TriggerTypeHTTP, agentParam.Spec.Triggers[0].Type)
}

// TestConfigToSandboxConversion verifies that sandbox config correctly converts to a blaxel.Sandbox structure
func TestConfigToSandboxConversion(t *testing.T) {
	// Simulate what GenerateDeployment does for sandbox type
	runtime := map[string]interface{}{
		"memory": int64(8192),
		"envs": []core.Env{
			{Name: "PLAYWRIGHT_BROWSERS_PATH", Value: "/root/.cache/ms-playwright"},
		},
		"ports": []map[string]interface{}{
			{
				"name":     "playwright",
				"target":   int64(3000),
				"protocol": "tcp",
			},
		},
	}

	volumes := []map[string]interface{}{
		{
			"name":      "data-volume",
			"mountPath": "/data",
		},
	}

	// Create the Result structure for sandbox
	result := core.Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       "Sandbox",
		Metadata: map[string]interface{}{
			"name": "my-test-sandbox",
		},
		Spec: map[string]interface{}{
			"runtime": runtime,
			"region":  "us-west-2",
			"volumes": volumes,
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(result)
	require.NoError(t, err)

	// Verify we can unmarshal into blaxel.SandboxParam
	var sandboxParam blaxel.SandboxParam
	err = json.Unmarshal(jsonData, &sandboxParam)
	require.NoError(t, err)

	// Verify sandbox metadata
	assert.Equal(t, "my-test-sandbox", sandboxParam.Metadata.Name)

	// Verify sandbox region
	assert.Equal(t, "us-west-2", sandboxParam.Spec.Region.Value)

	// Verify runtime memory
	assert.Equal(t, int64(8192), sandboxParam.Spec.Runtime.Memory.Value)

	// Verify environment variables
	require.Len(t, sandboxParam.Spec.Runtime.Envs, 1)
	assert.Equal(t, "PLAYWRIGHT_BROWSERS_PATH", sandboxParam.Spec.Runtime.Envs[0].Name.Value)
	assert.Equal(t, "/root/.cache/ms-playwright", sandboxParam.Spec.Runtime.Envs[0].Value.Value)

	// Verify volumes
	require.Len(t, sandboxParam.Spec.Volumes, 1)
	assert.Equal(t, "data-volume", sandboxParam.Spec.Volumes[0].Name.Value)
	assert.Equal(t, "/data", sandboxParam.Spec.Volumes[0].MountPath.Value)
}

// TestConfigToFunctionConversion verifies that function config correctly converts to a blaxel.Function structure
func TestConfigToFunctionConversion(t *testing.T) {
	// Simulate what GenerateDeployment does for function type
	runtime := map[string]interface{}{
		"memory": int64(2048),
		"type":   "mcp", // Functions have type = "mcp"
		"envs": []core.Env{
			{Name: "MCP_SERVER_NAME", Value: "my-mcp-server"},
		},
	}

	// Create the Result structure for function
	result := core.Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       "Function",
		Metadata: map[string]interface{}{
			"name": "my-test-function",
		},
		Spec: map[string]interface{}{
			"runtime": runtime,
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(result)
	require.NoError(t, err)

	// Verify we can unmarshal into blaxel.FunctionParam
	var functionParam blaxel.FunctionParam
	err = json.Unmarshal(jsonData, &functionParam)
	require.NoError(t, err)

	// Verify function metadata
	assert.Equal(t, "my-test-function", functionParam.Metadata.Name)

	// Verify runtime memory
	assert.Equal(t, int64(2048), functionParam.Spec.Runtime.Memory.Value)

	// Verify environment variables
	require.Len(t, functionParam.Spec.Runtime.Envs, 1)
	assert.Equal(t, "MCP_SERVER_NAME", functionParam.Spec.Runtime.Envs[0].Name.Value)
	assert.Equal(t, "my-mcp-server", functionParam.Spec.Runtime.Envs[0].Value.Value)
}

// TestConfigToJobConversion verifies that job config correctly converts to a blaxel.Job structure
func TestConfigToJobConversion(t *testing.T) {
	// Simulate what GenerateDeployment does for job type
	runtime := map[string]interface{}{
		"memory":             int64(4096),
		"maxConcurrentTasks": int64(10),
		"timeout":            int64(900),
		"maxRetries":         int64(3),
		"envs": []core.Env{
			{Name: "BATCH_SIZE", Value: "100"},
		},
	}

	triggers := []map[string]interface{}{
		{
			"type": "cron",
			"configuration": map[string]interface{}{
				"schedule": "0 * * * *", // Every hour
			},
		},
	}

	// Create the Result structure for job
	result := core.Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       "Job",
		Metadata: map[string]interface{}{
			"name": "my-test-job",
		},
		Spec: map[string]interface{}{
			"runtime":  runtime,
			"triggers": triggers,
		},
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(result)
	require.NoError(t, err)

	// Verify we can unmarshal into blaxel.JobParam
	var jobParam blaxel.JobParam
	err = json.Unmarshal(jsonData, &jobParam)
	require.NoError(t, err)

	// Verify job metadata
	assert.Equal(t, "my-test-job", jobParam.Metadata.Name)

	// Verify runtime
	assert.Equal(t, int64(4096), jobParam.Spec.Runtime.Memory.Value)
	assert.Equal(t, int64(10), jobParam.Spec.Runtime.MaxConcurrentTasks.Value)
	assert.Equal(t, int64(900), jobParam.Spec.Runtime.Timeout.Value)
	assert.Equal(t, int64(3), jobParam.Spec.Runtime.MaxRetries.Value)

	// Verify environment variables
	require.Len(t, jobParam.Spec.Runtime.Envs, 1)
	assert.Equal(t, "BATCH_SIZE", jobParam.Spec.Runtime.Envs[0].Name.Value)
	assert.Equal(t, "100", jobParam.Spec.Runtime.Envs[0].Value.Value)

	// Verify triggers (cron type with schedule in configuration)
	require.Len(t, jobParam.Spec.Triggers, 1)
	assert.Equal(t, blaxel.TriggerTypeCron, jobParam.Spec.Triggers[0].Type)
	assert.Equal(t, "0 * * * *", jobParam.Spec.Triggers[0].Configuration.Schedule.Value)
}

// TestEnvWithoutJSONTagsBehavior demonstrates what would happen without JSON tags
// This test documents the importance of having json tags on the Env struct
func TestEnvWithoutJSONTagsBehavior(t *testing.T) {
	// This is what the JSON looks like with proper json tags
	env := core.Env{
		Name:  "MY_VAR",
		Value: "my_value",
	}

	jsonData, err := json.Marshal(env)
	require.NoError(t, err)

	// Verify it produces lowercase field names that the API expects
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	// The API expects "name" and "value" (lowercase)
	// Without json tags, Go would produce "Name" and "Value" (capitalized)
	// which would cause the API to not recognize the fields
	_, hasName := parsed["name"]
	_, hasValue := parsed["value"]
	_, hasCapitalName := parsed["Name"]
	_, hasCapitalValue := parsed["Value"]

	assert.True(t, hasName, "JSON should have lowercase 'name' field")
	assert.True(t, hasValue, "JSON should have lowercase 'value' field")
	assert.False(t, hasCapitalName, "JSON should NOT have capitalized 'Name' field")
	assert.False(t, hasCapitalValue, "JSON should NOT have capitalized 'Value' field")
}

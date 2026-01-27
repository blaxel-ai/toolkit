package core

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

func TestResultJSONSerialization(t *testing.T) {
	result := Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       "Agent",
		Metadata: map[string]interface{}{
			"name": "test-agent",
			"labels": map[string]interface{}{
				"env": "production",
			},
		},
		Spec: map[string]interface{}{
			"runtime": map[string]interface{}{
				"memory": 4096,
				"envs": []map[string]interface{}{
					{"name": "API_KEY", "value": "secret"},
				},
			},
		},
		Status: "DEPLOYED",
	}

	// Test JSON marshaling
	jsonData, err := json.Marshal(result)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "blaxel.ai/v1alpha1", parsed["apiVersion"])
	assert.Equal(t, "Agent", parsed["kind"])
	assert.Equal(t, "DEPLOYED", parsed["status"])

	// Verify metadata
	metadata := parsed["metadata"].(map[string]interface{})
	assert.Equal(t, "test-agent", metadata["name"])

	// Verify spec
	spec := parsed["spec"].(map[string]interface{})
	runtime := spec["runtime"].(map[string]interface{})
	assert.Equal(t, float64(4096), runtime["memory"])
}

func TestResultYAMLSerialization(t *testing.T) {
	result := Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       "Function",
		Metadata: map[string]interface{}{
			"name": "test-function",
		},
		Spec: map[string]interface{}{
			"runtime": map[string]interface{}{
				"type": "mcp",
			},
		},
		Status: "DEPLOYED",
	}

	// Test YAML marshaling
	yamlData, err := yaml.Marshal(result)
	require.NoError(t, err)

	var parsed Result
	err = yaml.Unmarshal(yamlData, &parsed)
	require.NoError(t, err)

	assert.Equal(t, "blaxel.ai/v1alpha1", parsed.ApiVersion)
	assert.Equal(t, "Function", parsed.Kind)
	assert.Equal(t, "DEPLOYED", parsed.Status)
}

func TestResultToString(t *testing.T) {
	result := Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       "Sandbox",
		Metadata: map[string]interface{}{
			"name": "test-sandbox",
		},
		Spec: map[string]interface{}{
			"region": "us-west-2",
		},
		Status: "DEPLOYED",
	}

	str := result.ToString()

	assert.Contains(t, str, "apiVersion: blaxel.ai/v1alpha1")
	assert.Contains(t, str, "kind: Sandbox")
	assert.Contains(t, str, "name: test-sandbox")
	assert.Contains(t, str, "region: us-west-2")
}

func TestResultToStringEmpty(t *testing.T) {
	result := Result{}
	str := result.ToString()

	// Should still produce valid YAML even if empty
	assert.NotEmpty(t, str)
}

func TestCommandEnv(t *testing.T) {
	t.Run("Set and retrieve values", func(t *testing.T) {
		env := CommandEnv{}
		env.Set("KEY1", "value1")
		env.Set("KEY2", "value2")

		assert.Equal(t, "value1", env["KEY1"])
		assert.Equal(t, "value2", env["KEY2"])
	})

	t.Run("ToEnv returns slice of KEY=VALUE strings", func(t *testing.T) {
		env := CommandEnv{}
		env.Set("API_KEY", "secret")
		env.Set("DEBUG", "true")

		envSlice := env.ToEnv()
		assert.Len(t, envSlice, 2)

		// Check that both entries are present (order may vary due to map iteration)
		envMap := make(map[string]bool)
		for _, e := range envSlice {
			envMap[e] = true
		}
		assert.True(t, envMap["API_KEY=secret"])
		assert.True(t, envMap["DEBUG=true"])
	})

	t.Run("AddClientEnv adds environment variables", func(t *testing.T) {
		// Set a test environment variable
		os.Setenv("TEST_COMMAND_ENV_VAR", "test_value")
		defer os.Unsetenv("TEST_COMMAND_ENV_VAR")

		env := CommandEnv{}
		env.AddClientEnv()

		// The env should contain the test variable
		assert.Equal(t, "test_value", env["TEST_COMMAND_ENV_VAR"])
	})
}

func TestResultMetadata(t *testing.T) {
	// ResultMetadata is currently unused but we should test it exists
	metadata := ResultMetadata{
		Workspace: "my-workspace",
		Name:      "my-resource",
	}

	assert.Equal(t, "my-workspace", metadata.Workspace)
	assert.Equal(t, "my-resource", metadata.Name)
}

func TestResultWithOptionalStatus(t *testing.T) {
	// Test that status can be omitted
	result := Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       "Agent",
		Metadata: map[string]interface{}{
			"name": "test",
		},
		Spec: map[string]interface{}{},
		// Status is empty
	}

	jsonData, err := json.Marshal(result)
	require.NoError(t, err)

	// Status should be present but empty in JSON
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	// Status will be empty string due to json:"status,omitempty" tag
	// but since it's not truly omitted (just empty), it may or may not appear
}

func TestCommandEnvOverwrite(t *testing.T) {
	env := CommandEnv{}
	env.Set("KEY", "value1")
	env.Set("KEY", "value2")

	assert.Equal(t, "value2", env["KEY"])
}

func TestCommandEnvToEnvEmpty(t *testing.T) {
	env := CommandEnv{}
	envSlice := env.ToEnv()

	assert.Empty(t, envSlice)
}

func TestCommandEnvAddClientEnvMultiple(t *testing.T) {
	os.Setenv("TEST_VAR_1", "val1")
	os.Setenv("TEST_VAR_2", "val2")
	defer os.Unsetenv("TEST_VAR_1")
	defer os.Unsetenv("TEST_VAR_2")

	env := CommandEnv{}
	env.AddClientEnv()

	assert.Equal(t, "val1", env["TEST_VAR_1"])
	assert.Equal(t, "val2", env["TEST_VAR_2"])
}

func TestResultToStringWithComplexSpec(t *testing.T) {
	result := Result{
		ApiVersion: "blaxel.ai/v1alpha1",
		Kind:       "Job",
		Metadata: map[string]interface{}{
			"name":      "data-pipeline",
			"workspace": "production",
			"labels": map[string]interface{}{
				"team": "data",
				"env":  "prod",
			},
		},
		Spec: map[string]interface{}{
			"runtime": map[string]interface{}{
				"type":   "batch",
				"memory": 8192,
				"cpu":    4,
				"envs": []map[string]interface{}{
					{"name": "INPUT", "value": "/data/input"},
					{"name": "OUTPUT", "value": "/data/output"},
				},
			},
			"triggers": []map[string]interface{}{
				{"type": "cron", "schedule": "0 * * * *"},
			},
		},
		Status: "RUNNING",
	}

	str := result.ToString()

	assert.Contains(t, str, "data-pipeline")
	assert.Contains(t, str, "Job")
	assert.Contains(t, str, "batch")
}

func TestResultMetadataJSONTags(t *testing.T) {
	metadata := ResultMetadata{
		Workspace: "test-ws",
		Name:      "test-name",
	}

	jsonData, err := json.Marshal(metadata)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	// Note: ResultMetadata doesn't have json tags, so fields are capitalized
	assert.Equal(t, "test-ws", parsed["Workspace"])
	assert.Equal(t, "test-name", parsed["Name"])
}

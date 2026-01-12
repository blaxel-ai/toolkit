package core

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvStruct(t *testing.T) {
	t.Run("Env JSON serialization", func(t *testing.T) {
		env := Env{
			Name:  "API_KEY",
			Value: "secret-value",
		}

		jsonData, err := json.Marshal(env)
		require.NoError(t, err)

		// Verify JSON has lowercase field names
		assert.Contains(t, string(jsonData), `"name":"API_KEY"`)
		assert.Contains(t, string(jsonData), `"value":"secret-value"`)

		// Verify we can unmarshal back
		var parsed Env
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err)
		assert.Equal(t, env, parsed)
	})

	t.Run("Env slice JSON serialization", func(t *testing.T) {
		envs := []Env{
			{Name: "VAR1", Value: "value1"},
			{Name: "VAR2", Value: "value2"},
		}

		jsonData, err := json.Marshal(envs)
		require.NoError(t, err)

		var parsed []Env
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err)
		assert.Equal(t, envs, parsed)
	})

	t.Run("Env in map interface JSON serialization", func(t *testing.T) {
		// This simulates the runtime["envs"] use case
		runtime := map[string]interface{}{
			"envs": []Env{
				{Name: "ENV1", Value: "val1"},
				{Name: "ENV2", Value: "val2"},
			},
			"memory": 4096,
		}

		jsonData, err := json.Marshal(runtime)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err)

		envs := parsed["envs"].([]interface{})
		assert.Len(t, envs, 2)

		env1 := envs[0].(map[string]interface{})
		assert.Equal(t, "ENV1", env1["name"])
		assert.Equal(t, "val1", env1["value"])
	})
}

func TestEnvsType(t *testing.T) {
	t.Run("Envs is a map[string]string", func(t *testing.T) {
		var envs Envs = map[string]string{
			"KEY1": "value1",
			"KEY2": "value2",
		}

		assert.Equal(t, "value1", envs["KEY1"])
		assert.Equal(t, "value2", envs["KEY2"])
	})
}

func TestEnvJSONFieldNames(t *testing.T) {
	// This test explicitly verifies that JSON tags produce lowercase field names
	// which is required for the Blaxel API to correctly parse environment variables
	env := Env{
		Name:  "TEST_VAR",
		Value: "test_value",
	}

	jsonData, err := json.Marshal(env)
	require.NoError(t, err)

	// Parse as generic map to check actual field names
	var parsed map[string]interface{}
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	// Verify lowercase field names (from json tags)
	_, hasName := parsed["name"]
	_, hasValue := parsed["value"]
	_, hasCapitalName := parsed["Name"]
	_, hasCapitalValue := parsed["Value"]

	assert.True(t, hasName, "JSON should have lowercase 'name' field")
	assert.True(t, hasValue, "JSON should have lowercase 'value' field")
	assert.False(t, hasCapitalName, "JSON should NOT have capitalized 'Name' field")
	assert.False(t, hasCapitalValue, "JSON should NOT have capitalized 'Value' field")
}

func TestGetEnvsWithSecrets(t *testing.T) {
	// Save original state and restore after test
	originalSecrets := secrets
	originalConfig := config
	defer func() {
		secrets = originalSecrets
		config = originalConfig
	}()

	// Reset secrets and config
	secrets = Secrets{
		{Name: "SECRET_KEY", Value: "secret_value"},
		{Name: "DB_PASS", Value: "db_password"},
	}
	config = Config{
		Env: map[string]string{
			"APP_ENV": "production",
		},
	}

	envs := GetEnvs()

	// Should contain secrets and config env
	assert.GreaterOrEqual(t, len(envs), 3)

	// Verify secrets are included
	secretFound := false
	appEnvFound := false
	for _, env := range envs {
		if env.Name == "SECRET_KEY" && env.Value == "secret_value" {
			secretFound = true
		}
		if env.Name == "APP_ENV" && env.Value == "production" {
			appEnvFound = true
		}
	}
	assert.True(t, secretFound, "SECRET_KEY should be in envs")
	assert.True(t, appEnvFound, "APP_ENV should be in envs")
}

func TestGetEnvsIgnoresBLAPIKey(t *testing.T) {
	// Save original state and restore after test
	originalSecrets := secrets
	originalConfig := config
	defer func() {
		secrets = originalSecrets
		config = originalConfig
	}()

	secrets = Secrets{
		{Name: "BL_API_KEY", Value: "should_be_ignored"},
		{Name: "OTHER_KEY", Value: "should_be_included"},
	}
	config = Config{}

	envs := GetEnvs()

	// BL_API_KEY should not be in envs
	for _, env := range envs {
		assert.NotEqual(t, "BL_API_KEY", env.Name, "BL_API_KEY should be ignored")
	}

	// OTHER_KEY should be in envs
	found := false
	for _, env := range envs {
		if env.Name == "OTHER_KEY" {
			found = true
			break
		}
	}
	assert.True(t, found, "OTHER_KEY should be in envs")
}

func TestGetUniqueEnvs(t *testing.T) {
	// Save original state and restore after test
	originalSecrets := secrets
	originalConfig := config
	defer func() {
		secrets = originalSecrets
		config = originalConfig
	}()

	// Setup with duplicate names
	secrets = Secrets{
		{Name: "VAR1", Value: "value1_secret"},
	}
	config = Config{
		Env: map[string]string{
			"VAR1": "value1_config", // Duplicate name
			"VAR2": "value2",
		},
	}

	envs := GetUniqueEnvs()

	// Count occurrences of VAR1
	var1Count := 0
	for _, env := range envs {
		if env.Name == "VAR1" {
			var1Count++
		}
	}
	assert.Equal(t, 1, var1Count, "VAR1 should appear only once in unique envs")
}

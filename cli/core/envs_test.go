package core

import (
	"encoding/json"
	"os"
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

func TestSecretsEnvRegex(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		shouldMatch  bool
		secretName   string
		defaultValue string
	}{
		{"$secrets.KEY", "$secrets.MY_KEY", true, "MY_KEY", ""},
		{"${secrets.KEY}", "${secrets.MY_KEY}", true, "MY_KEY", ""},
		{"${ secrets.KEY }", "${ secrets.MY_KEY }", true, "MY_KEY", ""},
		{"$secrets.KEY:default", "$secrets.MY_KEY:fallback", true, "MY_KEY", "fallback"},
		{"${secrets.KEY:default}", "${secrets.MY_KEY:fallback}", true, "MY_KEY", "fallback"},
		{"${secrets.KEY:}", "${secrets.MY_KEY:}", true, "MY_KEY", ""},
		{"${secrets.KEY:complex-default}", "${secrets.BL_REGION:us-pdx-1}", true, "BL_REGION", "us-pdx-1"},
		{"plain value", "some_value", false, "", ""},
		{"$KEY", "$MY_KEY", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := secretsEnvRegex.FindStringSubmatch(tt.input)
			if !tt.shouldMatch {
				assert.Nil(t, match)
				return
			}
			require.NotNil(t, match, "expected match for %q", tt.input)
			// Extract name and default
			name := match[1]
			if name == "" {
				name = match[3]
			}
			def := match[2]
			if def == "" {
				def = match[4]
			}
			assert.Equal(t, tt.secretName, name)
			assert.Equal(t, tt.defaultValue, def)
		})
	}
}

func TestPlainEnvRegex(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		shouldMatch  bool
		varName      string
		defaultValue string
	}{
		{"$KEY", "$MY_KEY", true, "MY_KEY", ""},
		{"${KEY}", "${MY_KEY}", true, "MY_KEY", ""},
		{"${ KEY }", "${ MY_KEY }", true, "MY_KEY", ""},
		{"${KEY:default}", "${MY_KEY:fallback}", true, "MY_KEY", "fallback"},
		{"${KEY:complex-default}", "${BL_REGION:us-pdx-1}", true, "BL_REGION", "us-pdx-1"},
		{"${KEY:}", "${MY_KEY:}", true, "MY_KEY", ""},
		{"plain value", "some_value", false, "", ""},
		{"secrets pattern", "${secrets.KEY}", false, "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			match := plainEnvRegex.FindStringSubmatch(tt.input)
			if !tt.shouldMatch {
				assert.Nil(t, match)
				return
			}
			require.NotNil(t, match, "expected match for %q", tt.input)
			// Group 1: braced name, Group 2: braced default, Group 3: unbraced name
			name := match[1]
			def := match[2]
			if name == "" {
				name = match[3]
			}
			assert.Equal(t, tt.varName, name)
			assert.Equal(t, tt.defaultValue, def)
		})
	}
}

func TestGetEnvsWithDefaultValues(t *testing.T) {
	originalSecrets := secrets
	originalConfig := config
	defer func() {
		secrets = originalSecrets
		config = originalConfig
	}()

	t.Run("secrets with default value used when not set", func(t *testing.T) {
		secrets = Secrets{}
		config = Config{
			Env: map[string]string{
				"BL_REGION": "${secrets.BL_REGION:us-pdx-1}",
			},
		}
		// Make sure env var is not set
		_ = os.Unsetenv("BL_REGION")

		envs := GetEnvs()
		found := false
		for _, env := range envs {
			if env.Name == "BL_REGION" {
				assert.Equal(t, "us-pdx-1", env.Value)
				found = true
			}
		}
		assert.True(t, found, "BL_REGION should be in envs")
	})

	t.Run("secrets with default value overridden by env", func(t *testing.T) {
		secrets = Secrets{}
		config = Config{
			Env: map[string]string{
				"BL_REGION": "${secrets.BL_REGION:us-pdx-1}",
			},
		}
		t.Setenv("BL_REGION", "eu-west-1")

		envs := GetEnvs()
		for _, env := range envs {
			if env.Name == "BL_REGION" {
				assert.Equal(t, "eu-west-1", env.Value)
			}
		}
	})

	t.Run("secrets with default value overridden by loaded secret", func(t *testing.T) {
		secrets = Secrets{
			{Name: "BL_REGION", Value: "ap-east-1"},
		}
		config = Config{
			Env: map[string]string{
				"BL_REGION": "${secrets.BL_REGION:us-pdx-1}",
			},
		}
		_ = os.Unsetenv("BL_REGION")

		envs := GetEnvs()
		// The secret from secrets slice is added first, then config env resolves
		found := false
		for _, env := range envs {
			if env.Name == "BL_REGION" && env.Value == "ap-east-1" {
				found = true
			}
		}
		assert.True(t, found, "BL_REGION should resolve to loaded secret value")
	})

	t.Run("plain env with default value", func(t *testing.T) {
		secrets = Secrets{}
		config = Config{
			Env: map[string]string{
				"MY_VAR": "${MY_VAR:default-val}",
			},
		}
		_ = os.Unsetenv("MY_VAR")

		envs := GetEnvs()
		found := false
		for _, env := range envs {
			if env.Name == "MY_VAR" {
				assert.Equal(t, "default-val", env.Value)
				found = true
			}
		}
		assert.True(t, found, "MY_VAR should be in envs")
	})

	t.Run("plain env with default overridden by env", func(t *testing.T) {
		secrets = Secrets{}
		config = Config{
			Env: map[string]string{
				"MY_VAR": "${MY_VAR:default-val}",
			},
		}
		t.Setenv("MY_VAR", "from-env")

		envs := GetEnvs()
		for _, env := range envs {
			if env.Name == "MY_VAR" {
				assert.Equal(t, "from-env", env.Value)
			}
		}
	})

	t.Run("plain env without default still works", func(t *testing.T) {
		secrets = Secrets{}
		config = Config{
			Env: map[string]string{
				"PLAIN": "$PLAIN",
			},
		}
		t.Setenv("PLAIN", "set-value")

		envs := GetEnvs()
		for _, env := range envs {
			if env.Name == "PLAIN" {
				assert.Equal(t, "set-value", env.Value)
			}
		}
	})
}

func TestResolveVarValue(t *testing.T) {
	originalSecrets := secrets
	defer func() { secrets = originalSecrets }()

	t.Run("plain string unchanged", func(t *testing.T) {
		val, warn := ResolveVarValue("hello")
		assert.Equal(t, "hello", val)
		assert.Empty(t, warn)
	})

	t.Run("${secrets.KEY:default} uses default when unset", func(t *testing.T) {
		secrets = Secrets{}
		_ = os.Unsetenv("BL_REGION")
		val, warn := ResolveVarValue("${secrets.BL_REGION:us-pdx-1}")
		assert.Equal(t, "us-pdx-1", val)
		assert.Empty(t, warn)
	})

	t.Run("${secrets.KEY:default} uses env when set", func(t *testing.T) {
		secrets = Secrets{}
		t.Setenv("BL_REGION", "eu-west-1")
		val, warn := ResolveVarValue("${secrets.BL_REGION:us-pdx-1}")
		assert.Equal(t, "eu-west-1", val)
		assert.Empty(t, warn)
	})

	t.Run("${secrets.KEY:default} uses loaded secret", func(t *testing.T) {
		secrets = Secrets{{Name: "BL_REGION", Value: "ap-east-1"}}
		_ = os.Unsetenv("BL_REGION")
		val, warn := ResolveVarValue("${secrets.BL_REGION:us-pdx-1}")
		assert.Equal(t, "ap-east-1", val)
		assert.Empty(t, warn)
	})

	t.Run("${secrets.KEY} warns when unset", func(t *testing.T) {
		secrets = Secrets{}
		_ = os.Unsetenv("MISSING_SECRET")
		val, warn := ResolveVarValue("${secrets.MISSING_SECRET}")
		assert.Equal(t, "${secrets.MISSING_SECRET}", val)
		assert.Contains(t, warn, "MISSING_SECRET")
	})

	t.Run("${KEY:default} uses default when unset", func(t *testing.T) {
		_ = os.Unsetenv("MY_REGION")
		val, warn := ResolveVarValue("${MY_REGION:us-pdx-1}")
		assert.Equal(t, "us-pdx-1", val)
		assert.Empty(t, warn)
	})

	t.Run("${KEY:default} uses env when set", func(t *testing.T) {
		t.Setenv("MY_REGION", "eu-west-1")
		val, warn := ResolveVarValue("${MY_REGION:us-pdx-1}")
		assert.Equal(t, "eu-west-1", val)
		assert.Empty(t, warn)
	})

	t.Run("$KEY uses env when set", func(t *testing.T) {
		t.Setenv("SIMPLE", "val")
		val, warn := ResolveVarValue("$SIMPLE")
		assert.Equal(t, "val", val)
		assert.Empty(t, warn)
	})

	t.Run("$KEY warns when unset", func(t *testing.T) {
		_ = os.Unsetenv("UNSET_VAR")
		val, warn := ResolveVarValue("$UNSET_VAR")
		assert.Equal(t, "$UNSET_VAR", val)
		assert.Contains(t, warn, "UNSET_VAR")
	})

	t.Run("${ secrets.KEY:default } trims trailing space from default", func(t *testing.T) {
		secrets = Secrets{}
		_ = os.Unsetenv("BL_REGION")
		val, warn := ResolveVarValue("${ secrets.BL_REGION:us-pdx-1 }")
		assert.Equal(t, "us-pdx-1", val)
		assert.Empty(t, warn)
	})

	t.Run("${ KEY:default } trims trailing space from default", func(t *testing.T) {
		_ = os.Unsetenv("MY_REGION")
		val, warn := ResolveVarValue("${ MY_REGION:us-pdx-1 }")
		assert.Equal(t, "us-pdx-1", val)
		assert.Empty(t, warn)
	})
}

func TestResolveConfigVars(t *testing.T) {
	originalConfig := config
	originalSecrets := secrets
	defer func() {
		config = originalConfig
		secrets = originalSecrets
	}()

	t.Run("resolves region with secrets default", func(t *testing.T) {
		secrets = Secrets{}
		_ = os.Unsetenv("BL_REGION")
		config = Config{
			Region: "${secrets.BL_REGION:us-pdx-1}",
		}
		resolveConfigVars()
		assert.Equal(t, "us-pdx-1", config.Region)
	})

	t.Run("resolves region with env var default", func(t *testing.T) {
		secrets = Secrets{}
		_ = os.Unsetenv("BL_REGION")
		config = Config{
			Region: "${BL_REGION:us-pdx-1}",
		}
		resolveConfigVars()
		assert.Equal(t, "us-pdx-1", config.Region)
	})

	t.Run("resolves region from env overriding default", func(t *testing.T) {
		secrets = Secrets{}
		t.Setenv("BL_REGION", "eu-west-1")
		config = Config{
			Region: "${BL_REGION:us-pdx-1}",
		}
		resolveConfigVars()
		assert.Equal(t, "eu-west-1", config.Region)
	})

	t.Run("leaves plain values unchanged", func(t *testing.T) {
		secrets = Secrets{}
		config = Config{
			Region: "us-pdx-1",
			Name:   "my-agent",
			Type:   "agent",
		}
		resolveConfigVars()
		assert.Equal(t, "us-pdx-1", config.Region)
		assert.Equal(t, "my-agent", config.Name)
		assert.Equal(t, "agent", config.Type)
	})

	t.Run("resolves multiple fields", func(t *testing.T) {
		secrets = Secrets{}
		t.Setenv("AGENT_NAME", "prod-agent")
		_ = os.Unsetenv("BL_REGION")
		config = Config{
			Name:   "${AGENT_NAME}",
			Region: "${BL_REGION:us-pdx-1}",
		}
		resolveConfigVars()
		assert.Equal(t, "prod-agent", config.Name)
		assert.Equal(t, "us-pdx-1", config.Region)
	})
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

package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLookupSecret(t *testing.T) {
	// Save original secrets and restore after test
	originalSecrets := secrets
	defer func() { secrets = originalSecrets }()

	// Set up test secrets
	secrets = Secrets{
		{Name: "API_KEY", Value: "secret-api-key"},
		{Name: "DB_PASSWORD", Value: "secret-db-pass"},
		{Name: "EMPTY_VAR", Value: ""},
	}

	tests := []struct {
		name     string
		key      string
		expected string
	}{
		{"existing secret", "API_KEY", "secret-api-key"},
		{"another existing secret", "DB_PASSWORD", "secret-db-pass"},
		{"empty value secret", "EMPTY_VAR", ""},
		{"non-existent secret", "NON_EXISTENT", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := LookupSecret(tt.key)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSecrets(t *testing.T) {
	// Save original secrets and restore after test
	originalSecrets := secrets
	defer func() { secrets = originalSecrets }()

	// Set up test secrets
	secrets = Secrets{
		{Name: "VAR1", Value: "value1"},
		{Name: "VAR2", Value: "value2"},
	}

	result := GetSecrets()
	assert.Len(t, result, 2)
	assert.Equal(t, "VAR1", result[0].Name)
	assert.Equal(t, "value1", result[0].Value)
	assert.Equal(t, "VAR2", result[1].Name)
	assert.Equal(t, "value2", result[1].Value)
}

func TestLoadCommandSecrets(t *testing.T) {
	// Save original secrets and commandSecrets and restore after test
	originalSecrets := secrets
	originalCommandSecrets := commandSecrets
	defer func() {
		secrets = originalSecrets
		commandSecrets = originalCommandSecrets
	}()

	// Reset secrets
	secrets = Secrets{}

	// Set up test command secrets
	commandSecrets = []string{
		"API_KEY=my-api-key",
		"DB_PASSWORD=my-db-password",
		"COMPLEX_VALUE=value=with=equals",
	}

	loadCommandSecrets()

	assert.Len(t, secrets, 3)

	// Check each secret
	secretMap := make(map[string]string)
	for _, s := range secrets {
		secretMap[s.Name] = s.Value
	}

	assert.Equal(t, "my-api-key", secretMap["API_KEY"])
	assert.Equal(t, "my-db-password", secretMap["DB_PASSWORD"])
	assert.Equal(t, "value=with=equals", secretMap["COMPLEX_VALUE"])
}

func TestLoadCommandSecretsInvalidFormat(t *testing.T) {
	// Save original secrets and commandSecrets and restore after test
	originalSecrets := secrets
	originalCommandSecrets := commandSecrets
	defer func() {
		secrets = originalSecrets
		commandSecrets = originalCommandSecrets
	}()

	// Reset secrets
	secrets = Secrets{}

	// Set up invalid command secrets (missing =)
	commandSecrets = []string{
		"INVALID_SECRET",
		"VALID_KEY=valid_value",
	}

	// Should not panic, just skip invalid entries
	loadCommandSecrets()

	// Only valid secret should be loaded
	assert.Len(t, secrets, 1)
	assert.Equal(t, "VALID_KEY", secrets[0].Name)
	assert.Equal(t, "valid_value", secrets[0].Value)
}

func TestSecretsType(t *testing.T) {
	var s Secrets = []Env{
		{Name: "SECRET1", Value: "value1"},
		{Name: "SECRET2", Value: "value2"},
	}

	assert.Len(t, s, 2)
	assert.Equal(t, "SECRET1", s[0].Name)
	assert.Equal(t, "SECRET2", s[1].Name)
}

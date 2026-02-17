package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
)

func TestIsNewerVersion(t *testing.T) {
	tests := []struct {
		name           string
		latestVersion  string
		currentVersion string
		expected       bool
	}{
		{"newer major", "2.0.0", "1.0.0", true},
		{"newer minor", "1.1.0", "1.0.0", true},
		{"newer patch", "1.0.1", "1.0.0", true},
		{"same version", "1.0.0", "1.0.0", false},
		{"older major", "1.0.0", "2.0.0", false},
		{"older minor", "1.0.0", "1.1.0", false},
		{"older patch", "1.0.0", "1.0.1", false},
		{"complex version newer", "1.10.0", "1.9.0", true},
		{"complex version older", "1.9.0", "1.10.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isNewerVersion(tt.latestVersion, tt.currentVersion)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsNewerVersionInvalidSemver(t *testing.T) {
	// When semver parsing fails, fallback to string comparison
	t.Run("invalid current version", func(t *testing.T) {
		result := isNewerVersion("1.0.0", "invalid")
		assert.True(t, result) // Different strings
	})

	t.Run("invalid latest version", func(t *testing.T) {
		result := isNewerVersion("invalid", "1.0.0")
		assert.True(t, result) // Different strings
	})

	t.Run("both invalid but same", func(t *testing.T) {
		result := isNewerVersion("invalid", "invalid")
		assert.False(t, result) // Same strings
	})
}

func TestGetVersionCachePath(t *testing.T) {
	path := getVersionCachePath()

	// Should return a path containing ".blaxel/version"
	assert.Contains(t, path, ".blaxel")
	assert.Contains(t, path, "version")
}

func TestIsCIEnvironment(t *testing.T) {
	// Save original env vars
	originalEnvVars := map[string]string{
		"CI":               os.Getenv("CI"),
		"GITHUB_ACTIONS":   os.Getenv("GITHUB_ACTIONS"),
		"GITLAB_CI":        os.Getenv("GITLAB_CI"),
		"BUILDKITE":        os.Getenv("BUILDKITE"),
		"CIRCLECI":         os.Getenv("CIRCLECI"),
		"TRAVIS":           os.Getenv("TRAVIS"),
		"JENKINS_URL":      os.Getenv("JENKINS_URL"),
		"TEAMCITY_VERSION": os.Getenv("TEAMCITY_VERSION"),
	}

	// Restore after tests
	defer func() {
		for k, v := range originalEnvVars {
			if v == "" {
				os.Unsetenv(k)
			} else {
				os.Setenv(k, v)
			}
		}
	}()

	// Clear all CI env vars first
	clearCIEnvVars := func() {
		os.Unsetenv("CI")
		os.Unsetenv("GITHUB_ACTIONS")
		os.Unsetenv("GITLAB_CI")
		os.Unsetenv("BUILDKITE")
		os.Unsetenv("CIRCLECI")
		os.Unsetenv("TRAVIS")
		os.Unsetenv("JENKINS_URL")
		os.Unsetenv("TEAMCITY_VERSION")
	}

	t.Run("CI=true", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("CI", "true")
		assert.True(t, IsCIEnvironment())
	})

	t.Run("CI=1", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("CI", "1")
		assert.True(t, IsCIEnvironment())
	})

	t.Run("GITHUB_ACTIONS=true", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("GITHUB_ACTIONS", "true")
		assert.True(t, IsCIEnvironment())
	})

	t.Run("GITLAB_CI=true", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("GITLAB_CI", "true")
		assert.True(t, IsCIEnvironment())
	})

	t.Run("JENKINS_URL set", func(t *testing.T) {
		clearCIEnvVars()
		os.Setenv("JENKINS_URL", "http://jenkins.example.com")
		assert.True(t, IsCIEnvironment())
	})

	t.Run("no CI env vars", func(t *testing.T) {
		clearCIEnvVars()
		assert.False(t, IsCIEnvironment())
	})
}

func TestSetAndGetWorkspace(t *testing.T) {
	// Save original and restore
	original := workspace
	defer func() { workspace = original }()

	SetWorkspace("test-workspace")
	assert.Equal(t, "test-workspace", GetWorkspace())

	SetWorkspace("another-workspace")
	assert.Equal(t, "another-workspace", GetWorkspace())
}

func TestSetAndGetInteractiveMode(t *testing.T) {
	// Save original and restore
	original := interactiveMode
	defer func() { interactiveMode = original }()

	SetInteractiveMode(true)
	assert.True(t, IsInteractiveMode())

	SetInteractiveMode(false)
	assert.False(t, IsInteractiveMode())
}

func TestVersionCache(t *testing.T) {
	t.Run("struct fields", func(t *testing.T) {
		cache := versionCache{
			Version:   "1.0.0",
			LastCheck: time.Now(),
		}
		assert.Equal(t, "1.0.0", cache.Version)
		assert.False(t, cache.LastCheck.IsZero())
	})
}

func TestGetConfig(t *testing.T) {
	// Save original and restore
	original := config
	defer func() { config = original }()

	config = Config{
		Name:      "test-config",
		Type:      "agent",
		Workspace: "test-ws",
	}

	result := GetConfig()
	assert.Equal(t, "test-config", result.Name)
	assert.Equal(t, "agent", result.Type)
	assert.Equal(t, "test-ws", result.Workspace)
}

func TestSetConfigType(t *testing.T) {
	// Save original and restore
	original := config
	defer func() { config = original }()

	config = Config{}

	SetConfigType("function")
	assert.Equal(t, "function", config.Type)

	SetConfigType("agent")
	assert.Equal(t, "agent", config.Type)
}

func TestRegisterAndGetCommand(t *testing.T) {
	// Clear registry first
	originalRegistry := commandRegistry
	commandRegistry = make(map[string]func() *cobra.Command)
	defer func() { commandRegistry = originalRegistry }()

	// Test registering a command
	RegisterCommand("test-cmd", func() *cobra.Command {
		return &cobra.Command{Use: "test-cmd", Short: "Test command"}
	})

	// Test getting the registered command
	cmd := GetCommand("test-cmd")
	assert.Equal(t, "test-cmd", cmd.Use)
	assert.Equal(t, "Test command", cmd.Short)

	// Test getting a non-existent command
	notFoundCmd := GetCommand("non-existent")
	assert.Equal(t, "non-existent", notFoundCmd.Use)
	assert.Contains(t, notFoundCmd.Short, "not implemented")
}

func TestColorConstants(t *testing.T) {
	// Verify color constants are defined
	assert.Equal(t, "\033[33m", colorYellow)
	assert.Equal(t, "\033[36m", colorCyan)
	assert.Equal(t, "\033[32m", colorGreen)
	assert.Equal(t, "\033[1m", colorBold)
	assert.Equal(t, "\033[0m", colorReset)
}

func TestGetEnvFiles(t *testing.T) {
	// Save original and restore
	original := envFiles
	defer func() { envFiles = original }()

	envFiles = []string{".env", ".env.local"}

	result := GetEnvFiles()
	assert.Equal(t, []string{".env", ".env.local"}, result)
}

func TestGetCommandSecrets(t *testing.T) {
	// Save original and restore
	original := commandSecrets
	defer func() { commandSecrets = original }()

	commandSecrets = []string{"SECRET1=value1", "SECRET2=value2"}

	result := GetCommandSecrets()
	assert.Equal(t, []string{"SECRET1=value1", "SECRET2=value2"}, result)
}

func TestSetCommandSecrets(t *testing.T) {
	// Save original and restore
	original := commandSecrets
	defer func() { commandSecrets = original }()

	SetCommandSecrets([]string{"NEW_SECRET=new_value"})
	assert.Equal(t, []string{"NEW_SECRET=new_value"}, commandSecrets)
}

func TestGetVerbose(t *testing.T) {
	// Save original and restore
	original := verbose
	defer func() { verbose = original }()

	verbose = true
	assert.True(t, GetVerbose())

	verbose = false
	assert.False(t, GetVerbose())
}

func TestReadWriteVersionCache(t *testing.T) {
	// Create a temp directory for testing
	tempDir, err := os.MkdirTemp("", "version_cache_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Save original HOME/USERPROFILE and restore
	originalHome := os.Getenv("HOME")
	originalUserProfile := os.Getenv("USERPROFILE")
	defer func() {
		os.Setenv("HOME", originalHome)
		os.Setenv("USERPROFILE", originalUserProfile)
	}()
	os.Setenv("HOME", tempDir)
	os.Setenv("USERPROFILE", tempDir)

	// Create the .blaxel directory
	err = os.MkdirAll(filepath.Join(tempDir, ".blaxel"), 0755)
	assert.NoError(t, err)

	t.Run("write and read cache", func(t *testing.T) {
		testCache := versionCache{
			Version:   "1.2.3",
			LastCheck: time.Now(),
		}

		// Write cache
		err := writeVersionCache(testCache)
		assert.NoError(t, err)

		// Read cache back
		readCache, err := readVersionCache()
		assert.NoError(t, err)
		assert.Equal(t, testCache.Version, readCache.Version)
	})

	t.Run("read non-existent cache returns empty cache", func(t *testing.T) {
		// Use a new temp dir without a cache file
		newTempDir, err := os.MkdirTemp("", "no_cache_test")
		assert.NoError(t, err)
		defer os.RemoveAll(newTempDir)

		os.Setenv("HOME", newTempDir)
		os.Setenv("USERPROFILE", newTempDir)

		cache, err := readVersionCache()
		assert.NoError(t, err)
		assert.Equal(t, "", cache.Version)
	})
}

func TestGetOutputFormat(t *testing.T) {
	// Save original and restore
	original := outputFormat
	defer func() { outputFormat = original }()

	outputFormat = "json"
	assert.Equal(t, "json", GetOutputFormat())

	outputFormat = "yaml"
	assert.Equal(t, "yaml", GetOutputFormat())

	outputFormat = "table"
	assert.Equal(t, "table", GetOutputFormat())
}

func TestResetConfig(t *testing.T) {
	// Save original and restore
	original := config
	defer func() { config = original }()

	// Set a config
	config = Config{
		Name:      "test-config",
		Type:      "agent",
		Workspace: "test-ws",
	}

	// Reset config
	ResetConfig()

	// Verify it's empty
	assert.Equal(t, "", config.Name)
	assert.Equal(t, "", config.Type)
	assert.Equal(t, "", config.Workspace)
}

func TestGetVersion(t *testing.T) {
	// Save original and restore
	original := version
	defer func() { version = original }()

	version = "1.2.3"
	assert.Equal(t, "1.2.3", GetVersion())

	version = "dev"
	assert.Equal(t, "dev", GetVersion())
}

func TestGetCommit(t *testing.T) {
	// Save original and restore
	original := commit
	defer func() { commit = original }()

	commit = "abc123"
	assert.Equal(t, "abc123", GetCommit())
}

func TestGetDate(t *testing.T) {
	// Save original and restore
	original := date
	defer func() { date = original }()

	date = "2024-01-15"
	assert.Equal(t, "2024-01-15", GetDate())
}

func TestSetAndGetClient(t *testing.T) {
	// Save original and restore
	original := client
	defer func() { client = original }()

	// Test setting nil
	SetClient(nil)
	assert.Nil(t, GetClient())
}

func TestSetEnvFiles(t *testing.T) {
	// Save original and restore
	original := envFiles
	defer func() { envFiles = original }()

	setEnvFiles([]string{".env", ".env.local", ".env.production"})
	assert.Equal(t, []string{".env", ".env.local", ".env.production"}, envFiles)

	// Test empty array
	setEnvFiles([]string{})
	assert.Empty(t, envFiles)
}

func TestLoadCommandSecretsWrapper(t *testing.T) {
	// Save original and restore
	originalSecrets := commandSecrets
	defer func() { commandSecrets = originalSecrets }()

	// Test loading command secrets
	commandSecrets = nil
	LoadCommandSecrets([]string{"KEY1=value1", "KEY2=value2"})

	// Verify command secrets were set
	result := GetCommandSecrets()
	assert.Contains(t, result, "KEY1=value1")
	assert.Contains(t, result, "KEY2=value2")
}

func TestReadConfigTomlWrapper(t *testing.T) {
	// Create a temp directory with a blaxel.toml file
	tempDir, err := os.MkdirTemp("", "config_toml_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Write a test blaxel.toml file
	// Note: entrypoint uses "prod" and "dev" as TOML tags
	tomlContent := `name = "test-agent"
type = "agent"
workspace = "my-workspace"

[entrypoint]
prod = "python main.py"
dev = "python main.py --dev"
`
	err = os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644)
	assert.NoError(t, err)

	// Save original directory and config
	originalDir, err := os.Getwd()
	assert.NoError(t, err)
	originalConfig := config
	defer func() {
		os.Chdir(originalDir)
		config = originalConfig
	}()

	// Change to temp dir
	err = os.Chdir(tempDir)
	assert.NoError(t, err)

	// Reset config before reading
	ResetConfig()

	// Read the config (folder is relative to cwd)
	ReadConfigToml(".", false)

	// Verify config was read
	result := GetConfig()
	assert.Equal(t, "test-agent", result.Name)
	assert.Equal(t, "agent", result.Type)
	assert.Equal(t, "my-workspace", result.Workspace)
	assert.Equal(t, "python main.py", result.Entrypoint.Production)
	assert.Equal(t, "python main.py --dev", result.Entrypoint.Development)
}

func TestReadConfigTomlWithMissingFile(t *testing.T) {
	// Create an empty temp directory
	tempDir, err := os.MkdirTemp("", "no_config_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Save original directory and config
	originalDir, err := os.Getwd()
	assert.NoError(t, err)
	originalConfig := config
	defer func() {
		os.Chdir(originalDir)
		config = originalConfig
	}()

	// Change to temp dir
	err = os.Chdir(tempDir)
	assert.NoError(t, err)

	// Read the config - should not panic with missing file
	ReadConfigToml(".", false)
}

func TestReadSecretsWrapper(t *testing.T) {
	// Create a temp directory with a .env file
	tempDir, err := os.MkdirTemp("", "secrets_test")
	assert.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Write a test .env file
	envContent := `SECRET_KEY=secret_value
API_TOKEN=my_api_token
`
	err = os.WriteFile(filepath.Join(tempDir, ".env"), []byte(envContent), 0644)
	assert.NoError(t, err)

	// Save original directory and restore
	originalDir, err := os.Getwd()
	assert.NoError(t, err)
	originalEnvFiles := envFiles
	defer func() {
		os.Chdir(originalDir)
		envFiles = originalEnvFiles
	}()

	// Change to temp dir
	err = os.Chdir(tempDir)
	assert.NoError(t, err)

	// Read secrets (folder is relative to cwd now)
	ReadSecrets(".", []string{".env"})

	// Verify env files were set
	assert.Equal(t, []string{".env"}, GetEnvFiles())
}

func TestIsTerminalInteractive(t *testing.T) {
	// Test in non-interactive mode
	// Since we're running in a test, it's likely not an interactive terminal
	result := IsTerminalInteractive()
	// The result depends on the test environment
	// We just verify it returns a boolean
	assert.IsType(t, true, result)
}

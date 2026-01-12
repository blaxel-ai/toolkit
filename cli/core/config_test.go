package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigParsing(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "config_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("parses basic agent config", func(t *testing.T) {
		configContent := `
type = "agent"
name = "test-agent"
workspace = "my-workspace"

[entrypoint]
prod = "python main.py"
dev = "python main.py --dev"

[env]
API_KEY = "test-key"
DEBUG = "true"

[runtime]
memory = 4096
`
		configPath := filepath.Join(tempDir, "agent_config")
		require.NoError(t, os.MkdirAll(configPath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(configPath, "blaxel.toml"), []byte(configContent), 0644))

		var cfg Config
		content, err := os.ReadFile(filepath.Join(configPath, "blaxel.toml"))
		require.NoError(t, err)
		err = toml.Unmarshal(content, &cfg)
		require.NoError(t, err)

		assert.Equal(t, "agent", cfg.Type)
		assert.Equal(t, "test-agent", cfg.Name)
		assert.Equal(t, "my-workspace", cfg.Workspace)
		assert.Equal(t, "python main.py", cfg.Entrypoint.Production)
		assert.Equal(t, "python main.py --dev", cfg.Entrypoint.Development)
		assert.Equal(t, "test-key", cfg.Env["API_KEY"])
		assert.Equal(t, "true", cfg.Env["DEBUG"])
		assert.Equal(t, int64(4096), (*cfg.Runtime)["memory"])
	})

	t.Run("parses sandbox config with region", func(t *testing.T) {
		configContent := `
type = "sandbox"
name = "test-sandbox"
region = "us-west-2"

[runtime]
memory = 8192
`
		configPath := filepath.Join(tempDir, "sandbox_config")
		require.NoError(t, os.MkdirAll(configPath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(configPath, "blaxel.toml"), []byte(configContent), 0644))

		var cfg Config
		content, err := os.ReadFile(filepath.Join(configPath, "blaxel.toml"))
		require.NoError(t, err)
		err = toml.Unmarshal(content, &cfg)
		require.NoError(t, err)

		assert.Equal(t, "sandbox", cfg.Type)
		assert.Equal(t, "test-sandbox", cfg.Name)
		assert.Equal(t, "us-west-2", cfg.Region)
	})

	t.Run("parses job config with triggers", func(t *testing.T) {
		configContent := `
type = "job"
name = "test-job"

[runtime]
memory = 2048
maxConcurrentTasks = 10
timeout = 900

[[triggers]]
type = "schedule"
schedule = "0 * * * *"
`
		configPath := filepath.Join(tempDir, "job_config")
		require.NoError(t, os.MkdirAll(configPath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(configPath, "blaxel.toml"), []byte(configContent), 0644))

		var cfg Config
		content, err := os.ReadFile(filepath.Join(configPath, "blaxel.toml"))
		require.NoError(t, err)
		err = toml.Unmarshal(content, &cfg)
		require.NoError(t, err)

		assert.Equal(t, "job", cfg.Type)
		assert.Equal(t, "test-job", cfg.Name)
		require.NotNil(t, cfg.Triggers)
		assert.Len(t, *cfg.Triggers, 1)
	})

	t.Run("parses volume-template config", func(t *testing.T) {
		configContent := `
type = "volumetemplate"
name = "test-volume"
directory = "./data"
defaultSize = 1024
`
		configPath := filepath.Join(tempDir, "vt_config")
		require.NoError(t, os.MkdirAll(configPath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(configPath, "blaxel.toml"), []byte(configContent), 0644))

		var cfg Config
		content, err := os.ReadFile(filepath.Join(configPath, "blaxel.toml"))
		require.NoError(t, err)
		err = toml.Unmarshal(content, &cfg)
		require.NoError(t, err)

		assert.Equal(t, "volumetemplate", cfg.Type)
		assert.Equal(t, "test-volume", cfg.Name)
		assert.Equal(t, "./data", cfg.Directory)
		require.NotNil(t, cfg.DefaultSize)
		assert.Equal(t, 1024, *cfg.DefaultSize)
	})

	t.Run("parses config with policies", func(t *testing.T) {
		configContent := `
type = "agent"
name = "secure-agent"
policies = ["policy1", "policy2"]
`
		configPath := filepath.Join(tempDir, "policy_config")
		require.NoError(t, os.MkdirAll(configPath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(configPath, "blaxel.toml"), []byte(configContent), 0644))

		var cfg Config
		content, err := os.ReadFile(filepath.Join(configPath, "blaxel.toml"))
		require.NoError(t, err)
		err = toml.Unmarshal(content, &cfg)
		require.NoError(t, err)

		assert.Equal(t, "agent", cfg.Type)
		assert.Len(t, cfg.Policies, 2)
		assert.Contains(t, cfg.Policies, "policy1")
		assert.Contains(t, cfg.Policies, "policy2")
	})

	t.Run("parses config with multi-agent packages", func(t *testing.T) {
		configContent := `
type = "agent"
name = "multi-agent"
skipRoot = true

[agent.sub-agent-1]
path = "./agents/agent1"
port = 8001

[agent.sub-agent-2]
path = "./agents/agent2"
port = 8002
`
		configPath := filepath.Join(tempDir, "multi_config")
		require.NoError(t, os.MkdirAll(configPath, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(configPath, "blaxel.toml"), []byte(configContent), 0644))

		var cfg Config
		content, err := os.ReadFile(filepath.Join(configPath, "blaxel.toml"))
		require.NoError(t, err)
		err = toml.Unmarshal(content, &cfg)
		require.NoError(t, err)

		assert.True(t, cfg.SkipRoot)
		assert.Len(t, cfg.Agent, 2)
		assert.Equal(t, "./agents/agent1", cfg.Agent["sub-agent-1"].Path)
		assert.Equal(t, 8001, cfg.Agent["sub-agent-1"].Port)
	})
}

func TestEntrypointsStruct(t *testing.T) {
	t.Run("parses entrypoints from toml", func(t *testing.T) {
		tomlContent := `
[entrypoint]
prod = "python main.py"
dev = "python main.py --dev"
`
		var cfg struct {
			Entrypoint Entrypoints `toml:"entrypoint"`
		}
		err := toml.Unmarshal([]byte(tomlContent), &cfg)
		require.NoError(t, err)

		assert.Equal(t, "python main.py", cfg.Entrypoint.Production)
		assert.Equal(t, "python main.py --dev", cfg.Entrypoint.Development)
	})
}

func TestPackageStruct(t *testing.T) {
	t.Run("parses package from toml", func(t *testing.T) {
		tomlContent := `
[function.my-func]
path = "./functions/my-func"
port = 8080
type = "mcp"
`
		var cfg struct {
			Function map[string]Package `toml:"function"`
		}
		err := toml.Unmarshal([]byte(tomlContent), &cfg)
		require.NoError(t, err)

		pkg := cfg.Function["my-func"]
		assert.Equal(t, "./functions/my-func", pkg.Path)
		assert.Equal(t, 8080, pkg.Port)
		assert.Equal(t, "mcp", pkg.Type)
	})
}

func TestGetResources(t *testing.T) {
	resources := GetResources()
	assert.NotEmpty(t, resources)

	// Verify some expected resources exist
	resourceKinds := make(map[string]bool)
	for _, r := range resources {
		resourceKinds[r.Kind] = true
	}

	assert.True(t, resourceKinds["Agent"])
	assert.True(t, resourceKinds["Function"])
	assert.True(t, resourceKinds["Model"])
	assert.True(t, resourceKinds["Sandbox"])
	assert.True(t, resourceKinds["Job"])
	assert.True(t, resourceKinds["Volume"])
	assert.True(t, resourceKinds["VolumeTemplate"])
	assert.True(t, resourceKinds["Image"])
	assert.True(t, resourceKinds["Policy"])
}

func TestGetBlaxelTomlSample(t *testing.T) {
	sample := getBlaxelTomlSample()

	// Verify the sample contains expected sections
	assert.Contains(t, sample, "type = \"agent\"")
	assert.Contains(t, sample, "[entrypoint]")
	assert.Contains(t, sample, "[env]")
	assert.Contains(t, sample, "[runtime]")
	assert.Contains(t, sample, "memory = 4096")
}

func TestBuildBlaxelTomlWarning(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected string
	}{
		{
			name:     "entrypoint error",
			errMsg:   "type mismatch for core.Entrypoints: expected table",
			expected: "entrypoint",
		},
		{
			name:     "runtime error",
			errMsg:   "runtime: expected table",
			expected: "runtime",
		},
		{
			name:     "triggers error",
			errMsg:   "triggers: expected array",
			expected: "triggers",
		},
		{
			name:     "volumes error",
			errMsg:   "volumes: expected array",
			expected: "volumes",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildBlaxelTomlWarning(tomlError{msg: tt.errMsg})
			assert.Contains(t, result, tt.expected)
		})
	}
}

// Helper type for testing
type tomlError struct {
	msg string
}

func (e tomlError) Error() string {
	return e.msg
}

func TestResultStruct(t *testing.T) {
	t.Run("Result to JSON", func(t *testing.T) {
		result := Result{
			ApiVersion: "blaxel.ai/v1alpha1",
			Kind:       "Agent",
			Metadata: map[string]interface{}{
				"name": "test-agent",
			},
			Spec: map[string]interface{}{
				"runtime": map[string]interface{}{
					"memory": 4096,
				},
			},
			Status: "DEPLOYED",
		}

		jsonData, err := json.Marshal(result)
		require.NoError(t, err)

		var parsed map[string]interface{}
		err = json.Unmarshal(jsonData, &parsed)
		require.NoError(t, err)

		assert.Equal(t, "blaxel.ai/v1alpha1", parsed["apiVersion"])
		assert.Equal(t, "Agent", parsed["kind"])
		assert.Equal(t, "DEPLOYED", parsed["status"])
	})

	t.Run("Result ToString", func(t *testing.T) {
		result := Result{
			ApiVersion: "blaxel.ai/v1alpha1",
			Kind:       "Agent",
			Metadata: map[string]interface{}{
				"name": "test-agent",
			},
			Spec:   map[string]interface{}{},
			Status: "DEPLOYED",
		}

		yamlStr := result.ToString()
		assert.Contains(t, yamlStr, "apiVersion: blaxel.ai/v1alpha1")
		assert.Contains(t, yamlStr, "kind: Agent")
	})
}

func TestFieldStruct(t *testing.T) {
	field := Field{
		Key:     "STATUS",
		Value:   "status",
		Special: "date",
	}

	assert.Equal(t, "STATUS", field.Key)
	assert.Equal(t, "status", field.Value)
	assert.Equal(t, "date", field.Special)
}

func TestResourceConfigStruct(t *testing.T) {
	resource := Resource{
		Kind:     "Agent",
		Short:    "ag",
		Plural:   "agents",
		Singular: "agent",
		Aliases:  []string{"agt"},
		Fields: []Field{
			{Key: "NAME", Value: "name"},
		},
	}

	assert.Equal(t, "Agent", resource.Kind)
	assert.Equal(t, "ag", resource.Short)
	assert.Equal(t, "agents", resource.Plural)
	assert.Equal(t, "agent", resource.Singular)
	assert.Len(t, resource.Aliases, 1)
	assert.Len(t, resource.Fields, 1)
}

func TestGetBlaxelTomlWarning(t *testing.T) {
	// Save and restore original warning
	original := blaxelTomlWarning
	defer func() { blaxelTomlWarning = original }()

	blaxelTomlWarning = "test warning"
	assert.Equal(t, "test warning", GetBlaxelTomlWarning())
}

func TestClearBlaxelTomlWarning(t *testing.T) {
	// Save and restore original warning
	original := blaxelTomlWarning
	defer func() { blaxelTomlWarning = original }()

	blaxelTomlWarning = "test warning"
	ClearBlaxelTomlWarning()
	assert.Empty(t, blaxelTomlWarning)
}

func TestFormatBlaxelTomlWarning(t *testing.T) {
	t.Run("with field", func(t *testing.T) {
		result := formatBlaxelTomlWarning("entrypoint", "Must be a table")
		assert.Contains(t, result, "entrypoint")
		assert.Contains(t, result, "Must be a table")
		assert.Contains(t, result, "blaxel.toml Configuration Warning")
	})

	t.Run("without field", func(t *testing.T) {
		result := formatBlaxelTomlWarning("", "Generic error")
		assert.Contains(t, result, "Generic error")
		assert.Contains(t, result, "blaxel.toml Configuration Warning")
	})
}

func TestReadConfigTomlNoFile(t *testing.T) {
	// Save original config and restore
	original := config
	defer func() { config = original }()

	// Create a temp directory without blaxel.toml
	tempDir, err := os.MkdirTemp("", "config_test_nofile")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer os.Chdir(originalDir)

	// Reset config
	config = Config{}

	readConfigToml("", true)

	// Should set defaults when no file
	assert.Equal(t, []string{"all"}, config.Functions)
	assert.Equal(t, []string{"all"}, config.Models)
	assert.Equal(t, "agent", config.Type)
}

func TestReadConfigTomlWithFile(t *testing.T) {
	// Save original config and restore
	original := config
	defer func() { config = original }()

	// Create a temp directory with blaxel.toml
	tempDir, err := os.MkdirTemp("", "config_test_withfile")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	configContent := `
type = "function"
name = "my-function"
workspace = "test-workspace"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(configContent), 0644))

	// Change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer os.Chdir(originalDir)

	// Reset config
	config = Config{}

	readConfigToml("", false)

	assert.Equal(t, "function", config.Type)
	assert.Equal(t, "my-function", config.Name)
}

func TestResourceListExec(t *testing.T) {
	r := &Resource{Kind: "Agent"}
	result, err := r.ListExec()
	assert.Nil(t, result)
	assert.Nil(t, err)
}

func TestResourcePutFn(t *testing.T) {
	r := &Resource{Kind: "Agent"}
	result := r.PutFn("agent", "test-agent", nil)
	assert.Nil(t, result)
}

func TestResourcePostFn(t *testing.T) {
	r := &Resource{Kind: "Agent"}
	result := r.PostFn("agent", "test-agent", nil)
	assert.Nil(t, result)
}

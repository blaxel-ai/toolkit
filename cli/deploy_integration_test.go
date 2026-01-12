package cli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
	blaxel "github.com/stainless-sdks/blaxel-go"
	"github.com/stainless-sdks/blaxel-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockServer creates an httptest server that handles Blaxel API requests
func mockServer(t *testing.T, responses map[string]interface{}) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Mock request: %s %s", r.Method, r.URL.Path)

		// Build key from method + path
		key := r.Method + " " + r.URL.Path

		// Check exact match first
		if resp, ok := responses[key]; ok {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(resp)
			return
		}

		// Check prefix matches for parameterized routes
		for pattern, resp := range responses {
			parts := strings.SplitN(pattern, " ", 2)
			if len(parts) == 2 && r.Method == parts[0] && strings.HasPrefix(r.URL.Path, parts[1]) {
				w.Header().Set("Content-Type", "application/json")
				json.NewEncoder(w).Encode(resp)
				return
			}
		}

		// Default 404
		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
}

// setupMockClient creates and sets a mock blaxel client
func setupMockClient(t *testing.T, serverURL string) {
	client, err := blaxel.NewDefaultClient(
		option.WithBaseURL(serverURL),
		option.WithWorkspace("test-workspace"),
		option.WithAPIKey("test-api-key"),
	)
	require.NoError(t, err)
	core.SetClient(&client)
}

// TestGetResourceAgentIntegration tests getResource for agent type via mock API
func TestGetResourceAgentIntegration(t *testing.T) {
	responses := map[string]interface{}{
		"GET /agents/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-agent",
				"workspace": "test-workspace",
			},
			"spec": map[string]interface{}{
				"runtime": map[string]interface{}{
					"image": "registry.blaxel.ai/test-workspace/test-agent:abc123",
				},
			},
			"status": "DEPLOYED",
		},
	}

	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	resource, err := getResource("agent", "test-agent")
	require.NoError(t, err)
	assert.NotNil(t, resource)

	metadata := resource["metadata"].(map[string]interface{})
	assert.Equal(t, "test-agent", metadata["name"])

	spec := resource["spec"].(map[string]interface{})
	runtime := spec["runtime"].(map[string]interface{})
	assert.Contains(t, runtime["image"], "test-agent")
}

// TestGetResourceFunctionIntegration tests getResource for function type via mock API
func TestGetResourceFunctionIntegration(t *testing.T) {
	responses := map[string]interface{}{
		"GET /functions/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-function",
				"workspace": "test-workspace",
			},
			"spec": map[string]interface{}{
				"runtime": map[string]interface{}{
					"image": "registry.blaxel.ai/test-workspace/test-function:def456",
				},
			},
			"status": "DEPLOYED",
		},
	}

	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	resource, err := getResource("function", "test-function")
	require.NoError(t, err)
	assert.NotNil(t, resource)
}

// TestGetResourceJobIntegration tests getResource for job type via mock API
func TestGetResourceJobIntegration(t *testing.T) {
	responses := map[string]interface{}{
		"GET /jobs/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-job",
				"workspace": "test-workspace",
			},
			"spec": map[string]interface{}{
				"runtime": map[string]interface{}{
					"image": "registry.blaxel.ai/test-workspace/test-job:ghi789",
				},
			},
			"status": "DEPLOYED",
		},
	}

	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	resource, err := getResource("job", "test-job")
	require.NoError(t, err)
	assert.NotNil(t, resource)
}

// TestGetResourceSandboxIntegration tests getResource for sandbox type via mock API
func TestGetResourceSandboxIntegration(t *testing.T) {
	responses := map[string]interface{}{
		"GET /sandboxes/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-sandbox",
				"workspace": "test-workspace",
			},
			"spec": map[string]interface{}{
				"runtime": map[string]interface{}{
					"image": "registry.blaxel.ai/test-workspace/test-sandbox:jkl012",
				},
			},
			"status": "DEPLOYED",
		},
	}

	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	resource, err := getResource("sandbox", "test-sandbox")
	require.NoError(t, err)
	assert.NotNil(t, resource)
}

// TestGetResourceVolumeTemplateIntegration tests getResource for volume-template type via mock API
func TestGetResourceVolumeTemplateIntegration(t *testing.T) {
	responses := map[string]interface{}{
		"GET /volume_templates/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "test-vt",
				"workspace": "test-workspace",
			},
			"spec": map[string]interface{}{
				"defaultSize": 10,
			},
		},
	}

	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	resource, err := getResource("volume-template", "test-vt")
	require.NoError(t, err)
	assert.NotNil(t, resource)
}

// TestGetResourceUnknownTypeIntegration tests getResource with unknown type
func TestGetResourceUnknownTypeIntegration(t *testing.T) {
	server := mockServer(t, map[string]interface{}{})
	defer server.Close()
	setupMockClient(t, server.URL)

	_, err := getResource("unknown-type", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown resource type")
}

// TestGetResourceStatusAgentIntegration tests getResourceStatus for agent type via mock API
func TestGetResourceStatusAgentIntegration(t *testing.T) {
	responses := map[string]interface{}{
		"GET /agents/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-agent",
			},
			"status": "DEPLOYED",
		},
	}

	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	status, err := getResourceStatus("agent", "test-agent")
	require.NoError(t, err)
	assert.Equal(t, "DEPLOYED", status)
}

// TestGetResourceStatusFunctionIntegration tests getResourceStatus for function type via mock API
func TestGetResourceStatusFunctionIntegration(t *testing.T) {
	responses := map[string]interface{}{
		"GET /functions/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-function",
			},
			"status": "DEPLOYING",
		},
	}

	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	status, err := getResourceStatus("function", "test-function")
	require.NoError(t, err)
	assert.Equal(t, "DEPLOYING", status)
}

// TestGetResourceStatusJobIntegration tests getResourceStatus for job type via mock API
func TestGetResourceStatusJobIntegration(t *testing.T) {
	responses := map[string]interface{}{
		"GET /jobs/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-job",
			},
			"status": "BUILDING",
		},
	}

	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	status, err := getResourceStatus("job", "test-job")
	require.NoError(t, err)
	assert.Equal(t, "BUILDING", status)
}

// TestGetResourceStatusSandboxIntegration tests getResourceStatus for sandbox type via mock API
func TestGetResourceStatusSandboxIntegration(t *testing.T) {
	responses := map[string]interface{}{
		"GET /sandboxes/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-sandbox",
			},
			"status": "DEPLOYED",
		},
	}

	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	status, err := getResourceStatus("sandbox", "test-sandbox")
	require.NoError(t, err)
	assert.Equal(t, "DEPLOYED", status)
}

// TestGetResourceStatusVolumeTemplateIntegration tests getResourceStatus for volume-template type via mock API
func TestGetResourceStatusVolumeTemplateIntegration(t *testing.T) {
	responses := map[string]interface{}{
		"GET /volume_templates/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-vt",
			},
			// VolumeTemplate might not return status field
		},
	}

	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	status, err := getResourceStatus("volume-template", "test-vt")
	require.NoError(t, err)
	// VolumeTemplate may return UNKNOWN if no status field
	assert.True(t, status == "DEPLOYED" || status == "UNKNOWN" || status == "")
}

// TestGetResourceStatusUnknownTypeIntegration tests getResourceStatus with unknown type
func TestGetResourceStatusUnknownTypeIntegration(t *testing.T) {
	server := mockServer(t, map[string]interface{}{})
	defer server.Close()
	setupMockClient(t, server.URL)

	_, err := getResourceStatus("unknown-type", "test")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown resource type")
}

// TestGetResourceStatusNoStatusIntegration tests getResourceStatus when status is missing
func TestGetResourceStatusNoStatusIntegration(t *testing.T) {
	responses := map[string]interface{}{
		"GET /agents/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name": "test-agent",
			},
			// No status field
		},
	}

	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	status, err := getResourceStatus("agent", "test-agent")
	require.NoError(t, err)
	// When status field is missing, it may return empty string or UNKNOWN
	assert.True(t, status == "UNKNOWN" || status == "")
}

// TestGenerateDeploymentAgentIntegration tests GenerateDeployment for agent type
func TestGenerateDeploymentAgentIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml
	tomlContent := `name = "my-agent"
type = "agent"
workspace = "test-workspace"

[entrypoint]
production = "python main.py"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-agent",
		cwd:    tempDir,
	}

	result := d.GenerateDeployment(false)
	assert.Equal(t, "Agent", result.Kind)
	assert.Equal(t, "blaxel.ai/v1alpha1", result.ApiVersion)

	metadata := result.Metadata.(map[string]interface{})
	assert.Equal(t, "my-agent", metadata["name"])
}

// TestGenerateDeploymentFunctionIntegration tests GenerateDeployment for function type
func TestGenerateDeploymentFunctionIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml
	tomlContent := `name = "my-function"
type = "function"
workspace = "test-workspace"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-function",
		cwd:    tempDir,
	}

	result := d.GenerateDeployment(false)
	assert.Equal(t, "Function", result.Kind)

	// Check that runtime type is set to "mcp" for functions
	spec := result.Spec.(map[string]interface{})
	runtime := spec["runtime"].(map[string]interface{})
	assert.Equal(t, "mcp", runtime["type"])
}

// TestGenerateDeploymentJobIntegration tests GenerateDeployment for job type
func TestGenerateDeploymentJobIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml
	tomlContent := `name = "my-job"
type = "job"
workspace = "test-workspace"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-job",
		cwd:    tempDir,
	}

	result := d.GenerateDeployment(false)
	assert.Equal(t, "Job", result.Kind)
}

// TestGenerateDeploymentSandboxIntegration tests GenerateDeployment for sandbox type
func TestGenerateDeploymentSandboxIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml
	tomlContent := `name = "my-sandbox"
type = "sandbox"
workspace = "test-workspace"
region = "us-east-1"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-sandbox",
		cwd:    tempDir,
	}

	result := d.GenerateDeployment(false)
	assert.Equal(t, "Sandbox", result.Kind)

	spec := result.Spec.(map[string]interface{})
	assert.Equal(t, "us-east-1", spec["region"])
}

// TestGenerateDeploymentVolumeTemplateIntegration tests GenerateDeployment for volume-template type
func TestGenerateDeploymentVolumeTemplateIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	defaultSize := 100
	// Create blaxel.toml
	tomlContent := `name = "my-vt"
type = "volume-template"
workspace = "test-workspace"
defaultSize = 100
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-vt",
		cwd:    tempDir,
	}

	result := d.GenerateDeployment(false)
	assert.Equal(t, "VolumeTemplate", result.Kind)

	spec := result.Spec.(map[string]interface{})
	assert.Equal(t, defaultSize, spec["defaultSize"])
}

// TestGenerateDeploymentWithPoliciesIntegration tests GenerateDeployment with policies
func TestGenerateDeploymentWithPoliciesIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml
	tomlContent := `name = "my-agent"
type = "agent"
workspace = "test-workspace"
policies = ["policy1", "policy2"]
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-agent",
		cwd:    tempDir,
	}

	result := d.GenerateDeployment(false)
	spec := result.Spec.(map[string]interface{})
	policies := spec["policies"].([]string)
	assert.Contains(t, policies, "policy1")
	assert.Contains(t, policies, "policy2")
}

// TestGenerateDeploymentWithRuntimeIntegration tests GenerateDeployment with runtime config
func TestGenerateDeploymentWithRuntimeIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml
	tomlContent := `name = "my-agent"
type = "agent"
workspace = "test-workspace"

[runtime]
memory = 4096
minScale = 1
maxScale = 10
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-agent",
		cwd:    tempDir,
	}

	result := d.GenerateDeployment(false)
	spec := result.Spec.(map[string]interface{})
	runtime := spec["runtime"].(map[string]interface{})
	assert.Equal(t, int64(4096), runtime["memory"])
	assert.Equal(t, int64(1), runtime["minScale"])
	assert.Equal(t, int64(10), runtime["maxScale"])
}

// TestGenerateDeploymentSkipBuildWithImageIntegration tests GenerateDeployment with skipBuild=true
func TestGenerateDeploymentSkipBuildWithImageIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml
	tomlContent := `name = "my-agent"
type = "agent"
workspace = "test-workspace"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	// Setup mock server that returns an existing image
	responses := map[string]interface{}{
		"GET /agents/": map[string]interface{}{
			"metadata": map[string]interface{}{
				"name":      "my-agent",
				"workspace": "test-workspace",
			},
			"spec": map[string]interface{}{
				"runtime": map[string]interface{}{
					"image": "registry.blaxel.ai/test-workspace/my-agent:existing-tag",
				},
			},
			"status": "DEPLOYED",
		},
	}
	server := mockServer(t, responses)
	defer server.Close()
	setupMockClient(t, server.URL)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-agent",
		cwd:    tempDir,
	}

	result := d.GenerateDeployment(true) // skipBuild=true
	spec := result.Spec.(map[string]interface{})
	runtime := spec["runtime"].(map[string]interface{})
	assert.Equal(t, "registry.blaxel.ai/test-workspace/my-agent:existing-tag", runtime["image"])
}

// TestValidateDeploymentConfigIntegration tests the validateDeploymentConfig function
func TestValidateDeploymentConfigIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create valid blaxel.toml
	tomlContent := `name = "my-agent"
type = "agent"
workspace = "test-workspace"

[entrypoint]
production = "python main.py"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	// Create the entry file
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "main.py"), []byte("print('hello')"), 0644))

	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)
	config := core.GetConfig()

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-agent",
		cwd:    tempDir,
	}

	// Should not return warning for valid config
	warning := d.validateDeploymentConfig(config)
	assert.Empty(t, warning)
}

// TestValidateDeploymentConfigMissingEntrypointIntegration tests warning for missing entrypoint/language
func TestValidateDeploymentConfigMissingEntrypointIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml without entrypoint and without standard entry files
	tomlContent := `name = "my-agent"
type = "agent"
workspace = "test-workspace"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)
	config := core.GetConfig()

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-agent",
		cwd:    tempDir,
	}

	warning := d.validateDeploymentConfig(config)
	// Should have warning about missing language/configuration
	assert.NotEmpty(t, warning)
	// Warning should mention language detection or configuration
	assert.True(t, strings.Contains(warning, "language") || strings.Contains(warning, "Configuration") || strings.Contains(warning, "entrypoint"))
}

// TestHandleConfigWarningNonInteractive tests the handleConfigWarning behavior in non-interactive mode
func TestHandleConfigWarningNonInteractive(t *testing.T) {
	// Test with noTTY=true (should just print warning)
	// This shouldn't panic
	handleConfigWarning("Test warning message", true)
}

// TestDeploymentArchiveCreationIntegration tests creating archives
func TestDeploymentArchiveCreationIntegration(t *testing.T) {
	tempDir := t.TempDir()

	// Create test structure
	os.MkdirAll(filepath.Join(tempDir, "src"), 0755)
	os.WriteFile(filepath.Join(tempDir, "src", "main.py"), []byte("print('hello')"), 0644)
	os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte("name = \"test\""), 0644)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "test",
		cwd:    tempDir,
	}

	// Test Zip
	err := d.Zip()
	require.NoError(t, err)
	assert.NotNil(t, d.archive)
	assert.FileExists(t, d.archive.Name())

	// Verify archive is not empty
	info, err := os.Stat(d.archive.Name())
	require.NoError(t, err)
	assert.Greater(t, info.Size(), int64(0))
}

// TestDeploymentTarArchiveIntegration tests creating tar archives
func TestDeploymentTarArchiveIntegration(t *testing.T) {
	tempDir := t.TempDir()

	// Create test structure
	os.MkdirAll(filepath.Join(tempDir, "src"), 0755)
	os.WriteFile(filepath.Join(tempDir, "src", "main.py"), []byte("print('hello')"), 0644)
	os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte("name = \"test\""), 0644)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "test",
		cwd:    tempDir,
	}

	err := d.Tar()
	require.NoError(t, err)
	assert.NotNil(t, d.archive)
	assert.FileExists(t, d.archive.Name())
}

// TestDeploymentWithVolumes tests deployment with volumes configuration
func TestDeploymentWithVolumes(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml with volumes
	tomlContent := `name = "my-sandbox"
type = "sandbox"
workspace = "test-workspace"

[[volumes]]
name = "data-volume"
mountPath = "/data"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-sandbox",
		cwd:    tempDir,
	}

	result := d.GenerateDeployment(false)
	assert.Equal(t, "Sandbox", result.Kind)

	spec := result.Spec.(map[string]interface{})
	volumes := spec["volumes"]
	assert.NotNil(t, volumes)
}

// TestDeploymentWithTriggers tests deployment with triggers configuration
func TestDeploymentWithTriggers(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml with triggers
	tomlContent := `name = "my-agent"
type = "agent"
workspace = "test-workspace"

[[triggers]]
type = "http"
path = "/webhook"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "my-agent",
		cwd:    tempDir,
	}

	result := d.GenerateDeployment(false)
	assert.Equal(t, "Agent", result.Kind)

	spec := result.Spec.(map[string]interface{})
	triggers := spec["triggers"]
	assert.NotNil(t, triggers)
}

// TestDeploymentPrintZipIntegration tests PrintZip function
func TestDeploymentPrintZipIntegration(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	os.MkdirAll(filepath.Join(tempDir, "src"), 0755)
	os.WriteFile(filepath.Join(tempDir, "src", "main.py"), []byte("print('hello')"), 0644)
	os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte("name = \"test\""), 0644)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "test",
		cwd:    tempDir,
	}

	// First create the zip
	err := d.Zip()
	require.NoError(t, err)

	// Then print it
	err = d.PrintZip()
	require.NoError(t, err)
}

// TestDeploymentPrintTarIntegration tests PrintTar function
func TestDeploymentPrintTarIntegration(t *testing.T) {
	tempDir := t.TempDir()

	// Create test files
	os.MkdirAll(filepath.Join(tempDir, "src"), 0755)
	os.WriteFile(filepath.Join(tempDir, "src", "main.py"), []byte("print('hello')"), 0644)
	os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte("name = \"test\""), 0644)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "test",
		cwd:    tempDir,
	}

	// First create the tar
	err := d.Tar()
	require.NoError(t, err)

	// Then print it
	err = d.PrintTar()
	require.NoError(t, err)
}

// TestDeploymentPrintWithZipIntegration tests Print function with zip archive
func TestDeploymentPrintWithZipIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml for agent (uses zip)
	tomlContent := `name = "test-agent"
type = "agent"
workspace = "test-workspace"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.WriteFile(filepath.Join(tempDir, "main.py"), []byte("print('hello')"), 0644)
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "test-agent",
		cwd:    tempDir,
	}

	// Generate deployment first
	result := d.GenerateDeployment(false)
	d.blaxelDeployments = []core.Result{result}

	// Test Print with skipBuild=false (will create zip and print)
	err := d.Print(false)
	require.NoError(t, err)
}

// TestDeploymentPrintWithTarIntegration tests Print function with tar archive for volume-template
func TestDeploymentPrintWithTarIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml for volume-template (uses tar)
	tomlContent := `name = "test-vt"
type = "volume-template"
workspace = "test-workspace"
defaultSize = 10
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.WriteFile(filepath.Join(tempDir, "data.txt"), []byte("test data"), 0644)
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "test-vt",
		cwd:    tempDir,
	}

	// Generate deployment first
	result := d.GenerateDeployment(false)
	d.blaxelDeployments = []core.Result{result}

	// Test Print with skipBuild=false (will create tar and print)
	err := d.Print(false)
	require.NoError(t, err)
}

// TestDeploymentPrintSkipBuildIntegration tests Print function with skipBuild=true
func TestDeploymentPrintSkipBuildIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	tomlContent := `name = "test-agent"
type = "agent"
workspace = "test-workspace"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)

	d := &Deployment{
		dir:    ".blaxel",
		folder: "",
		name:   "test-agent",
		cwd:    tempDir,
	}

	result := d.GenerateDeployment(false)
	d.blaxelDeployments = []core.Result{result}

	// Test Print with skipBuild=true (should skip archive creation)
	err := d.Print(true)
	require.NoError(t, err)
}

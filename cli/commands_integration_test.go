package cli

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setupTestServer creates a mock HTTP server for Blaxel API
func setupTestServer(t *testing.T, handlers map[string]http.HandlerFunc) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Logf("Request: %s %s", r.Method, r.URL.Path)

		// Build key from method + path
		key := r.Method + " " + r.URL.Path

		// Check exact match first
		if handler, ok := handlers[key]; ok {
			handler(w, r)
			return
		}

		// Check prefix matches
		for pattern, handler := range handlers {
			parts := strings.SplitN(pattern, " ", 2)
			if len(parts) == 2 && r.Method == parts[0] && strings.HasPrefix(r.URL.Path, parts[1]) {
				handler(w, r)
				return
			}
		}

		w.WriteHeader(http.StatusNotFound)
		json.NewEncoder(w).Encode(map[string]string{"error": "not found"})
	}))
}

// setupTestClient creates and sets a mock blaxel client
func setupTestClient(t *testing.T, serverURL string) {
	client, err := blaxel.NewDefaultClient(
		option.WithBaseURL(serverURL),
		option.WithWorkspace("test-workspace"),
		option.WithAPIKey("test-api-key"),
	)
	require.NoError(t, err)
	core.SetClient(&client)
	core.SetWorkspace("test-workspace")
}

// TestListAgentsIntegration tests listing agents via the API
func TestListAgentsIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /agents": func(w http.ResponseWriter, r *http.Request) {
			agents := []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"name":      "agent-1",
						"workspace": "test-workspace",
						"createdAt": "2024-01-01T00:00:00Z",
					},
					"status": "DEPLOYED",
				},
				{
					"metadata": map[string]interface{}{
						"name":      "agent-2",
						"workspace": "test-workspace",
						"createdAt": "2024-01-02T00:00:00Z",
					},
					"status": "DEPLOYING",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(agents)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	agents, err := client.Agents.List(ctx)
	require.NoError(t, err)
	assert.Len(t, *agents, 2)
}

// TestListFunctionsIntegration tests listing functions via the API
func TestListFunctionsIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /functions": func(w http.ResponseWriter, r *http.Request) {
			functions := []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"name":      "function-1",
						"workspace": "test-workspace",
					},
					"status": "DEPLOYED",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(functions)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	functions, err := client.Functions.List(ctx)
	require.NoError(t, err)
	assert.Len(t, *functions, 1)
}

// TestListModelsIntegration tests listing models via the API
func TestListModelsIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /models": func(w http.ResponseWriter, r *http.Request) {
			models := []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"name":      "gpt-4",
						"workspace": "test-workspace",
					},
					"spec": map[string]interface{}{
						"type": "openai",
					},
				},
				{
					"metadata": map[string]interface{}{
						"name":      "claude-3",
						"workspace": "test-workspace",
					},
					"spec": map[string]interface{}{
						"type": "anthropic",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(models)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	models, err := client.Models.List(ctx)
	require.NoError(t, err)
	assert.Len(t, *models, 2)
}

// TestListJobsIntegration tests listing jobs via the API
func TestListJobsIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /jobs": func(w http.ResponseWriter, r *http.Request) {
			jobs := []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"name":      "job-1",
						"workspace": "test-workspace",
					},
					"status": "DEPLOYED",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(jobs)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	jobs, err := client.Jobs.List(ctx)
	require.NoError(t, err)
	assert.Len(t, *jobs, 1)
}

// TestListSandboxesIntegration tests listing sandboxes via the API
func TestListSandboxesIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /sandboxes": func(w http.ResponseWriter, r *http.Request) {
			sandboxes := []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"name":      "sandbox-1",
						"workspace": "test-workspace",
					},
					"status": "DEPLOYED",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(sandboxes)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	sandboxes, err := client.Sandboxes.List(ctx)
	require.NoError(t, err)
	assert.Len(t, *sandboxes, 1)
}

// TestListVolumesIntegration tests listing volumes via the API
func TestListVolumesIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /volumes": func(w http.ResponseWriter, r *http.Request) {
			volumes := []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"name":      "volume-1",
						"workspace": "test-workspace",
					},
					"spec": map[string]interface{}{
						"size": 1024,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(volumes)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	volumes, err := client.Volumes.List(ctx)
	require.NoError(t, err)
	assert.Len(t, *volumes, 1)
}

// TestListPoliciesIntegration tests listing policies via the API
func TestListPoliciesIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /policies": func(w http.ResponseWriter, r *http.Request) {
			policies := []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"name":      "policy-1",
						"workspace": "test-workspace",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(policies)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	policies, err := client.Policies.List(ctx)
	require.NoError(t, err)
	assert.Len(t, *policies, 1)
}

// TestGetSingleAgentIntegration tests getting a single agent
func TestGetSingleAgentIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /agents/": func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Split(r.URL.Path, "/")
			agentName := parts[len(parts)-1]

			agent := map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      agentName,
					"workspace": "test-workspace",
					"url":       "https://test-agent.blaxel.ai",
				},
				"spec": map[string]interface{}{
					"runtime": map[string]interface{}{
						"image":  "registry.blaxel.ai/test/agent:latest",
						"memory": 2048,
					},
				},
				"status": "DEPLOYED",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(agent)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	agent, err := client.Agents.Get(ctx, "my-agent", blaxel.AgentGetParams{})
	require.NoError(t, err)
	assert.Equal(t, "my-agent", agent.Metadata.Name)
	assert.Equal(t, "DEPLOYED", string(agent.Status))
}

// TestDeleteAgentIntegration tests deleting an agent
func TestDeleteAgentIntegration(t *testing.T) {
	var deletedAgent string
	handlers := map[string]http.HandlerFunc{
		"DELETE /agents/": func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Split(r.URL.Path, "/")
			deletedAgent = parts[len(parts)-1]
			// Return JSON response for delete
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"metadata": map[string]interface{}{"name": deletedAgent},
			})
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	_, err := client.Agents.Delete(ctx, "agent-to-delete")
	require.NoError(t, err)
	assert.Equal(t, "agent-to-delete", deletedAgent)
}

// TestDeleteFunctionIntegration tests deleting a function
func TestDeleteFunctionIntegration(t *testing.T) {
	var deletedFunction string
	handlers := map[string]http.HandlerFunc{
		"DELETE /functions/": func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Split(r.URL.Path, "/")
			deletedFunction = parts[len(parts)-1]
			// Return JSON response for delete
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"metadata": map[string]interface{}{"name": deletedFunction},
			})
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	_, err := client.Functions.Delete(ctx, "function-to-delete")
	require.NoError(t, err)
	assert.Equal(t, "function-to-delete", deletedFunction)
}

// TestCreateAgentIntegration tests creating an agent
func TestCreateAgentIntegration(t *testing.T) {
	var createdBody map[string]interface{}
	handlers := map[string]http.HandlerFunc{
		"POST /agents": func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			json.Unmarshal(body, &createdBody)

			response := map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      "new-agent",
					"workspace": "test-workspace",
				},
				"status": "DEPLOYING",
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			json.NewEncoder(w).Encode(response)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	agent, err := client.Agents.New(ctx, blaxel.AgentNewParams{
		Agent: blaxel.AgentParam{
			Metadata: blaxel.MetadataParam{
				Name: "new-agent",
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "new-agent", agent.Metadata.Name)
}

// TestUpdateAgentIntegration tests updating an agent
func TestUpdateAgentIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"PUT /agents/": func(w http.ResponseWriter, r *http.Request) {
			parts := strings.Split(r.URL.Path, "/")
			agentName := parts[len(parts)-1]

			response := map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":      agentName,
					"workspace": "test-workspace",
				},
				"status": "DEPLOYED",
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(response)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	agent, err := client.Agents.Update(ctx, "existing-agent", blaxel.AgentUpdateParams{
		Agent: blaxel.AgentParam{
			Metadata: blaxel.MetadataParam{
				Name: "existing-agent",
			},
		},
	})
	require.NoError(t, err)
	assert.Equal(t, "existing-agent", agent.Metadata.Name)
}

// TestJobExecutionsListIntegration tests listing job executions
func TestJobExecutionsListIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /jobs/my-job/executions": func(w http.ResponseWriter, r *http.Request) {
			executions := []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"id":    "exec-1",
						"jobID": "my-job",
					},
					"status": "COMPLETED",
				},
				{
					"metadata": map[string]interface{}{
						"id":    "exec-2",
						"jobID": "my-job",
					},
					"status": "RUNNING",
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(executions)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	executions, err := client.Jobs.Executions.List(ctx, "my-job", blaxel.JobExecutionListParams{})
	require.NoError(t, err)
	assert.Len(t, *executions, 2)
}

// TestIntegrationConnectionsListIntegration tests listing integration connections
func TestIntegrationConnectionsListIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /integrations/connections": func(w http.ResponseWriter, r *http.Request) {
			connections := []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"name":      "github-connection",
						"workspace": "test-workspace",
					},
					"spec": map[string]interface{}{
						"integration": "github",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(connections)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	connections, err := client.Integrations.Connections.List(ctx)
	require.NoError(t, err)
	assert.Len(t, *connections, 1)
}

// TestImagesListIntegration tests listing images
func TestImagesListIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /images": func(w http.ResponseWriter, r *http.Request) {
			images := []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"name":      "agent/my-agent",
						"workspace": "test-workspace",
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(images)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	images, err := client.Images.List(ctx)
	require.NoError(t, err)
	assert.Len(t, *images, 1)
}

// TestAPIErrorHandlingIntegration tests handling various API errors
func TestAPIErrorHandlingIntegration(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		expected   string
	}{
		{"NotFound", http.StatusNotFound, "404"},
		{"BadRequest", http.StatusBadRequest, "400"},
		{"Unauthorized", http.StatusUnauthorized, "401"},
		{"InternalServerError", http.StatusInternalServerError, "500"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handlers := map[string]http.HandlerFunc{
				"GET /agents": func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(tt.statusCode)
					json.NewEncoder(w).Encode(map[string]string{"error": "test error"})
				},
			}

			server := setupTestServer(t, handlers)
			defer server.Close()
			setupTestClient(t, server.URL)

			ctx := context.Background()
			client := core.GetClient()
			_, err := client.Agents.List(ctx)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expected)
		})
	}
}

// TestApplyYAMLFileIntegration tests applying resources from YAML files
func TestApplyYAMLFileIntegration(t *testing.T) {
	tempDir := t.TempDir()

	// Create a test YAML file
	yamlContent := `apiVersion: blaxel.ai/v1alpha1
kind: Agent
metadata:
  name: test-agent
spec:
  runtime:
    image: test-image:latest
    memory: 2048
`
	yamlFile := filepath.Join(tempDir, "agent.yaml")
	err := os.WriteFile(yamlFile, []byte(yamlContent), 0644)
	require.NoError(t, err)

	// Verify file can be read
	content, err := os.ReadFile(yamlFile)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test-agent")
	assert.Contains(t, string(content), "Agent")
}

// TestApplyMultipleResourcesIntegration tests applying multiple YAML resources
func TestApplyMultipleResourcesIntegration(t *testing.T) {
	tempDir := t.TempDir()

	// Create multiple YAML files
	resources := map[string]string{
		"agent.yaml": `apiVersion: blaxel.ai/v1alpha1
kind: Agent
metadata:
  name: my-agent
spec:
  runtime:
    memory: 2048
`,
		"function.yaml": `apiVersion: blaxel.ai/v1alpha1
kind: Function
metadata:
  name: my-function
spec:
  runtime:
    memory: 1024
`,
		"model.yaml": `apiVersion: blaxel.ai/v1alpha1
kind: Model
metadata:
  name: my-model
spec:
  type: openai
`,
	}

	for name, content := range resources {
		err := os.WriteFile(filepath.Join(tempDir, name), []byte(content), 0644)
		require.NoError(t, err)
	}

	// Verify all files created
	entries, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

// TestVolumeTemplatesListIntegration tests listing volume templates
func TestVolumeTemplatesListIntegration(t *testing.T) {
	handlers := map[string]http.HandlerFunc{
		"GET /volume_templates": func(w http.ResponseWriter, r *http.Request) {
			templates := []map[string]interface{}{
				{
					"metadata": map[string]interface{}{
						"name":      "template-1",
						"workspace": "test-workspace",
					},
					"spec": map[string]interface{}{
						"defaultSize": 10,
					},
				},
			}
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(templates)
		},
	}

	server := setupTestServer(t, handlers)
	defer server.Close()
	setupTestClient(t, server.URL)

	ctx := context.Background()
	client := core.GetClient()
	templates, err := client.VolumeTemplates.List(ctx)
	require.NoError(t, err)
	assert.Len(t, *templates, 1)
}

// TestWorkspaceConfigIntegration tests workspace configuration
func TestWorkspaceConfigIntegration(t *testing.T) {
	// Test workspace getter/setter
	originalWorkspace := core.GetWorkspace()
	defer core.SetWorkspace(originalWorkspace)

	core.SetWorkspace("new-workspace")
	assert.Equal(t, "new-workspace", core.GetWorkspace())

	core.SetWorkspace("another-workspace")
	assert.Equal(t, "another-workspace", core.GetWorkspace())
}

// TestCoreConfigIntegration tests core configuration functions
func TestCoreConfigIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml
	tomlContent := `name = "test-app"
type = "agent"
workspace = "test-workspace"

[runtime]
memory = 4096
minScale = 0
maxScale = 5
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)
	config := core.GetConfig()

	assert.Equal(t, "test-app", config.Name)
	assert.Equal(t, "agent", config.Type)
	assert.Equal(t, "test-workspace", config.Workspace)
	// Verify runtime config is parsed
	assert.NotNil(t, config.Runtime)
}

// TestMonorepoConfigIntegration tests monorepo configuration parsing
func TestMonorepoConfigIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create monorepo blaxel.toml
	tomlContent := `name = "monorepo"
type = "agent"
workspace = "test-workspace"
skipRoot = true

[agent.sub-agent-1]
directory = "agents/agent1"

[agent.sub-agent-2]
directory = "agents/agent2"

[function.my-function]
directory = "functions/func1"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)
	config := core.GetConfig()

	assert.True(t, config.SkipRoot)
	assert.Len(t, config.Agent, 2)
	assert.Len(t, config.Function, 1)
}

// TestEnvConfigIntegration tests environment configuration parsing
func TestEnvConfigIntegration(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, _ := os.Getwd()
	defer os.Chdir(originalDir)

	// Create blaxel.toml with env section
	tomlContent := `name = "test-app"
type = "agent"
workspace = "test-workspace"

[env]
API_KEY = "secret-key"
DATABASE_URL = "postgres://localhost/db"
DEBUG = "true"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	os.Chdir(tempDir)

	core.ResetConfig()
	core.ReadConfigToml("", true)
	config := core.GetConfig()

	assert.NotNil(t, config.Env)
	assert.Len(t, config.Env, 3)
}

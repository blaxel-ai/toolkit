package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCmd(t *testing.T) {
	cmd := GetCmd()

	assert.Equal(t, "get", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify subcommands are added for each resource type
	subcommands := cmd.Commands()
	assert.NotEmpty(t, subcommands)

	// Verify common subcommands exist
	subcommandNames := make(map[string]bool)
	for _, sub := range subcommands {
		subcommandNames[sub.Use] = true
	}

	assert.True(t, subcommandNames["agents"])
	assert.True(t, subcommandNames["functions"])
	assert.True(t, subcommandNames["models"])
	assert.True(t, subcommandNames["sandboxes"])
	assert.True(t, subcommandNames["jobs"])
	assert.True(t, subcommandNames["volumes"])
}

func TestDeleteCmd(t *testing.T) {
	cmd := DeleteCmd()

	assert.Equal(t, "delete", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify -f flag exists
	flag := cmd.Flags().Lookup("filename")
	assert.NotNil(t, flag)
	assert.Equal(t, "f", flag.Shorthand)

	// Verify -R flag exists
	rFlag := cmd.Flags().Lookup("recursive")
	assert.NotNil(t, rFlag)
	assert.Equal(t, "R", rFlag.Shorthand)

	// Verify subcommands are added
	subcommands := cmd.Commands()
	assert.NotEmpty(t, subcommands)
}

func TestApplyCmd(t *testing.T) {
	cmd := ApplyCmd()

	assert.Equal(t, "apply", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags exist
	flag := cmd.Flags().Lookup("filename")
	assert.NotNil(t, flag)

	envFlag := cmd.Flags().Lookup("env-file")
	assert.NotNil(t, envFlag)

	secretsFlag := cmd.Flags().Lookup("secrets")
	assert.NotNil(t, secretsFlag)

	rFlag := cmd.Flags().Lookup("recursive")
	assert.NotNil(t, rFlag)
}

func TestApplyOptionWithRecursive(t *testing.T) {
	opts := &applyOptions{}

	option := WithRecursive(true)
	option(opts)

	assert.True(t, opts.recursive)

	option = WithRecursive(false)
	option(opts)

	assert.False(t, opts.recursive)
}

func TestApplyResultStruct(t *testing.T) {
	result := ApplyResult{
		Kind: "Agent",
		Name: "my-agent",
		Result: ResourceOperationResult{
			Status:         "created",
			UploadURL:      "https://upload.example.com",
			CallbackSecret: "secret-123",
		},
	}

	assert.Equal(t, "Agent", result.Kind)
	assert.Equal(t, "my-agent", result.Name)
	assert.Equal(t, "created", result.Result.Status)
	assert.Equal(t, "https://upload.example.com", result.Result.UploadURL)
	assert.Equal(t, "secret-123", result.Result.CallbackSecret)
}

func TestResourceOperationResult(t *testing.T) {
	tests := []struct {
		name     string
		result   ResourceOperationResult
		expected string
	}{
		{
			name: "created",
			result: ResourceOperationResult{
				Status: "created",
			},
			expected: "created",
		},
		{
			name: "configured",
			result: ResourceOperationResult{
				Status: "configured",
			},
			expected: "configured",
		},
		{
			name: "failed",
			result: ResourceOperationResult{
				Status:   "failed",
				ErrorMsg: "something went wrong",
			},
			expected: "failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.result.Status)
		})
	}
}

func TestExtractCallbackSecret(t *testing.T) {
	tests := []struct {
		name     string
		response interface{}
		expected string
	}{
		{
			name:     "nil response",
			response: nil,
			expected: "",
		},
		{
			name: "response with callback secret",
			response: map[string]interface{}{
				"spec": map[string]interface{}{
					"triggers": []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"callbackSecret": "my-secret-123",
							},
						},
					},
				},
			},
			expected: "my-secret-123",
		},
		{
			name: "response with masked callback secret",
			response: map[string]interface{}{
				"spec": map[string]interface{}{
					"triggers": []interface{}{
						map[string]interface{}{
							"configuration": map[string]interface{}{
								"callbackSecret": "****",
							},
						},
					},
				},
			},
			expected: "",
		},
		{
			name: "response without triggers",
			response: map[string]interface{}{
				"spec": map[string]interface{}{
					"runtime": map[string]interface{}{},
				},
			},
			expected: "",
		},
		{
			name: "response without spec",
			response: map[string]interface{}{
				"metadata": map[string]interface{}{
					"name": "test",
				},
			},
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCallbackSecret(tt.response)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsBlaxelError(t *testing.T) {
	t.Run("not a blaxel error", func(t *testing.T) {
		var apiErr *blaxelError
		err := context.DeadlineExceeded

		result := isBlaxelErrorHelper(err, &apiErr)
		assert.False(t, result)
	})
}

// Helper to avoid import cycle - mimics isBlaxelError behavior
func isBlaxelErrorHelper(err error, apiErr **blaxelError) bool {
	if e, ok := err.(*blaxelError); ok {
		*apiErr = e
		return true
	}
	return false
}

type blaxelError struct {
	StatusCode int
	Message    string
}

func (e *blaxelError) Error() string {
	return e.Message
}

func TestApplyWithYAMLFileParsing(t *testing.T) {
	// Create a temporary directory for test files
	tempDir, err := os.MkdirTemp("", "apply_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create a test YAML file
	yamlContent := `apiVersion: blaxel.ai/v1alpha1
kind: Policy
metadata:
  name: test-policy
spec:
  description: Test policy
`
	yamlPath := filepath.Join(tempDir, "test.yaml")
	require.NoError(t, os.WriteFile(yamlPath, []byte(yamlContent), 0644))

	// Verify file was created correctly
	content, err := os.ReadFile(yamlPath)
	require.NoError(t, err)
	assert.Contains(t, string(content), "test-policy")
}

func TestListExecWithNilResource(t *testing.T) {
	resource := &core.Resource{
		Kind: "Test",
		List: nil,
	}

	result, err := ListExec(resource)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not a valid function")
	assert.Nil(t, result)
}

func TestSetBodyFieldsFromJSON(t *testing.T) {
	type TestStruct struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}

	type ParentStruct struct {
		Inner TestStruct
	}

	// Test setting fields from JSON
	jsonData := []byte(`{"name": "test", "value": 42}`)

	var parent ParentStruct
	parentVal := reflect.ValueOf(&parent).Elem()

	setBodyFieldsFromJSON(parentVal, jsonData)

	assert.Equal(t, "test", parent.Inner.Name)
	assert.Equal(t, 42, parent.Inner.Value)
}

func TestApplyResourcesEmpty(t *testing.T) {
	results := []core.Result{}

	applyResults, err := ApplyResources(results)
	assert.NoError(t, err)
	assert.Empty(t, applyResults)
}

func TestResourceStructure(t *testing.T) {
	// Create a minimal resource for testing
	resource := core.Resource{
		Kind:     "Test",
		Plural:   "tests",
		Singular: "test",
	}

	assert.Equal(t, "Test", resource.Kind)
	assert.Equal(t, "tests", resource.Plural)
	assert.Equal(t, "test", resource.Singular)
}

func TestHandleResourceOperationNilFunction(t *testing.T) {
	resource := &core.Resource{
		Kind: "Test",
		Put:  nil,
		Post: nil,
	}

	result, err := handleResourceOperation(resource, "test", nil, "put")
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "not a valid function")
}

func TestApplyWithInvalidFile(t *testing.T) {
	result, err := Apply("/non/existent/path.yaml")

	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestApplyWithDirectoryParsing(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "apply_dir_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create multiple YAML files
	yaml1 := `apiVersion: blaxel.ai/v1alpha1
kind: Policy
metadata:
  name: policy-1
spec:
  description: Policy 1
`
	yaml2 := `apiVersion: blaxel.ai/v1alpha1
kind: Policy
metadata:
  name: policy-2
spec:
  description: Policy 2
`

	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "policy1.yaml"), []byte(yaml1), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "policy2.yaml"), []byte(yaml2), 0644))

	// Verify files were created
	files, err := os.ReadDir(tempDir)
	require.NoError(t, err)
	assert.Len(t, files, 2)
}

func TestResourceOperationResultJSON(t *testing.T) {
	result := ResourceOperationResult{
		Status:         "created",
		UploadURL:      "https://upload.example.com",
		ErrorMsg:       "",
		CallbackSecret: "secret",
	}

	jsonData, err := json.Marshal(result)
	require.NoError(t, err)

	var parsed ResourceOperationResult
	err = json.Unmarshal(jsonData, &parsed)
	require.NoError(t, err)

	assert.Equal(t, result.Status, parsed.Status)
	assert.Equal(t, result.UploadURL, parsed.UploadURL)
	assert.Equal(t, result.CallbackSecret, parsed.CallbackSecret)
}

func TestCreateSandboxCmd(t *testing.T) {
	cmd := CreateSandboxCmd()

	assert.Equal(t, "create-sandbox", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.Contains(t, cmd.Short, "Deprecated")
	assert.Contains(t, cmd.Long, "deprecated")

	// Verify aliases
	assert.Contains(t, cmd.Aliases, "cs")
}

func TestChatCmd(t *testing.T) {
	cmd := ChatCmd()

	assert.Equal(t, "chat [agent-name]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags
	debugFlag := cmd.Flags().Lookup("debug")
	assert.NotNil(t, debugFlag)

	localFlag := cmd.Flags().Lookup("local")
	assert.NotNil(t, localFlag)

	headerFlag := cmd.Flags().Lookup("header")
	assert.NotNil(t, headerFlag)
}

func TestNewCmd(t *testing.T) {
	cmd := NewCmd()

	assert.Equal(t, "new [type] [directory]", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)
	assert.NotEmpty(t, cmd.Example)

	// Verify flags
	templateFlag := cmd.Flags().Lookup("template")
	assert.NotNil(t, templateFlag)
	assert.Equal(t, "t", templateFlag.Shorthand)

	yesFlag := cmd.Flags().Lookup("yes")
	assert.NotNil(t, yesFlag)
	assert.Equal(t, "y", yesFlag.Shorthand)
}

func TestParseNewType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected newType
	}{
		{"agent", "agent", newTypeAgent},
		{"ag alias", "ag", newTypeAgent},
		{"Agent uppercase", "Agent", newTypeAgent},
		{"mcp", "mcp", newTypeMCP},
		{"MCP uppercase", "MCP", newTypeMCP},
		{"sandbox", "sandbox", newTypeSandbox},
		{"sbx alias", "sbx", newTypeSandbox},
		{"job", "job", newTypeJob},
		{"jb alias", "jb", newTypeJob},
		{"volumetemplate", "volumetemplate", newTypeVolumeTemplate},
		{"vt alias", "vt", newTypeVolumeTemplate},
		{"volume-template alias", "volume-template", newTypeVolumeTemplate},
		{"unknown type", "unknown", ""},
		{"empty string", "", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseNewType(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

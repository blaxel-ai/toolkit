package auth

import (
	"context"
	"errors"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWorkspaceClient implements WorkspaceClient for testing
type mockWorkspaceClient struct {
	workspaces *[]blaxel.Workspace
	err        error
}

func (m *mockWorkspaceClient) Get(ctx context.Context, workspaceName string, opts ...option.RequestOption) (*blaxel.Workspace, error) {
	if m.err != nil {
		return nil, m.err
	}
	if m.workspaces == nil {
		return nil, errors.New("workspace list unavailable")
	}
	for _, workspace := range *m.workspaces {
		if workspace.Name == workspaceName {
			return &workspace, nil
		}
	}
	return nil, errors.New("workspace not found")
}

func (m *mockWorkspaceClient) List(ctx context.Context, opts ...option.RequestOption) (*[]blaxel.Workspace, error) {
	return m.workspaces, m.err
}

// mockClientFactory creates a mock client factory for testing
func mockClientFactory(workspaces *[]blaxel.Workspace, err error) ClientFactory {
	return func(opts ...option.RequestOption) WorkspaceClient {
		return &mockWorkspaceClient{workspaces: workspaces, err: err}
	}
}

type countingWorkspaceClient struct {
	getCalls         int
	getWorkspaceName string
	listCalls        int
	workspace        *blaxel.Workspace
	workspaces       *[]blaxel.Workspace
	err              error
}

func (c *countingWorkspaceClient) Get(ctx context.Context, workspaceName string, opts ...option.RequestOption) (*blaxel.Workspace, error) {
	c.getCalls++
	c.getWorkspaceName = workspaceName
	if c.err != nil {
		return nil, c.err
	}
	if c.workspace == nil {
		return nil, errors.New("workspace not found")
	}
	return c.workspace, nil
}

func (c *countingWorkspaceClient) List(ctx context.Context, opts ...option.RequestOption) (*[]blaxel.Workspace, error) {
	c.listCalls++
	return c.workspaces, c.err
}

// TestBuildClientOptionsEmpty tests BuildClientOptions with empty credentials
func TestBuildClientOptionsEmpty(t *testing.T) {
	opts := BuildClientOptions("", blaxel.Credentials{})
	// Should have at least the base URL option
	assert.NotEmpty(t, opts)
}

// TestBuildClientOptionsWithWorkspace tests BuildClientOptions with workspace
func TestBuildClientOptionsWithWorkspace(t *testing.T) {
	opts := BuildClientOptions("test-workspace", blaxel.Credentials{})
	// Should have base URL and workspace options
	assert.GreaterOrEqual(t, len(opts), 2)
}

// TestBuildClientOptionsWithAPIKey tests BuildClientOptions with API key
func TestBuildClientOptionsWithAPIKey(t *testing.T) {
	creds := blaxel.Credentials{APIKey: "test-key"}
	opts := BuildClientOptions("", creds)
	// Should have base URL and API key options
	assert.GreaterOrEqual(t, len(opts), 2)
}

// TestBuildClientOptionsWithAccessToken tests BuildClientOptions with access token
func TestBuildClientOptionsWithAccessToken(t *testing.T) {
	creds := blaxel.Credentials{AccessToken: "test-token"}
	opts := BuildClientOptions("", creds)
	// Should have base URL and access token options
	assert.GreaterOrEqual(t, len(opts), 2)
}

// TestBuildClientOptionsWithClientCredentials tests BuildClientOptions with client credentials
func TestBuildClientOptionsWithClientCredentials(t *testing.T) {
	creds := blaxel.Credentials{ClientCredentials: "test-creds"}
	opts := BuildClientOptions("", creds)
	// Should have base URL and client credentials options
	assert.GreaterOrEqual(t, len(opts), 2)
}

// TestBuildClientOptionsWithAll tests BuildClientOptions with all options
func TestBuildClientOptionsWithAll(t *testing.T) {
	creds := blaxel.Credentials{APIKey: "test-key"}
	opts := BuildClientOptions("test-workspace", creds)
	// Should have base URL, workspace, and API key options
	assert.GreaterOrEqual(t, len(opts), 3)
}

// TestValidateWorkspaceSuccess tests successful workspace validation
func TestValidateWorkspaceSuccess(t *testing.T) {
	workspaces := []blaxel.Workspace{
		{Name: "test-workspace"},
	}
	factory := mockClientFactory(&workspaces, nil)

	err := validateWorkspaceWithFactory("test-workspace", blaxel.Credentials{APIKey: "key"}, factory)
	require.NoError(t, err)
}

// TestValidateWorkspaceError tests workspace validation with error
func TestValidateWorkspaceError(t *testing.T) {
	factory := mockClientFactory(nil, errors.New("API error"))

	err := validateWorkspaceWithFactory("test-workspace", blaxel.Credentials{APIKey: "key"}, factory)
	require.Error(t, err)
	assert.Equal(t, "permission denied for workspace \"test-workspace\"", err.Error())
	assert.NotContains(t, err.Error(), "API error")
}

// TestValidateWorkspaceMissingWorkspace tests explicit workspace validation failure wording.
func TestValidateWorkspaceMissingWorkspace(t *testing.T) {
	workspaces := []blaxel.Workspace{{Name: "other-workspace"}}
	factory := mockClientFactory(&workspaces, nil)

	err := validateWorkspaceWithFactory("test-workspace", blaxel.Credentials{APIKey: "key"}, factory)
	require.Error(t, err)
	assert.Equal(t, "permission denied for workspace \"test-workspace\"", err.Error())
	assert.NotContains(t, err.Error(), "does not exist")
}

// TestValidateWorkspaceEmptyWorkspace tests validation with empty workspace
func TestValidateWorkspaceEmptyWorkspace(t *testing.T) {
	workspaces := []blaxel.Workspace{}
	factory := mockClientFactory(&workspaces, nil)

	err := validateWorkspaceWithFactory("", blaxel.Credentials{APIKey: "key"}, factory)
	require.NoError(t, err)
}

func TestValidateWorkspaceExplicitWorkspaceUsesGet(t *testing.T) {
	client := &countingWorkspaceClient{
		workspace: &blaxel.Workspace{Name: "test-workspace"},
	}
	factory := func(opts ...option.RequestOption) WorkspaceClient {
		return client
	}

	err := validateWorkspaceWithFactory("test-workspace", blaxel.Credentials{APIKey: "key"}, factory)
	require.NoError(t, err)
	assert.Equal(t, 1, client.getCalls)
	assert.Equal(t, "test-workspace", client.getWorkspaceName)
	assert.Equal(t, 0, client.listCalls)
}

func TestValidateWorkspaceEmptyWorkspaceUsesList(t *testing.T) {
	workspaces := []blaxel.Workspace{{Name: "test-workspace"}}
	client := &countingWorkspaceClient{
		workspaces: &workspaces,
	}
	factory := func(opts ...option.RequestOption) WorkspaceClient {
		return client
	}

	err := validateWorkspaceWithFactory("", blaxel.Credentials{APIKey: "key"}, factory)
	require.NoError(t, err)
	assert.Equal(t, 0, client.getCalls)
	assert.Equal(t, 1, client.listCalls)
}

// TestListWorkspacesSuccess tests successful workspace listing
func TestListWorkspacesSuccess(t *testing.T) {
	workspaces := []blaxel.Workspace{
		{Name: "workspace-1"},
		{Name: "workspace-2"},
	}
	factory := mockClientFactory(&workspaces, nil)

	result, err := listWorkspacesWithFactory(blaxel.Credentials{APIKey: "key"}, factory)
	require.NoError(t, err)
	assert.Len(t, result, 2)
	assert.Equal(t, "workspace-1", result[0].Name)
	assert.Equal(t, "workspace-2", result[1].Name)
}

// TestListWorkspacesError tests workspace listing with error
func TestListWorkspacesError(t *testing.T) {
	factory := mockClientFactory(nil, errors.New("API error"))

	result, err := listWorkspacesWithFactory(blaxel.Credentials{APIKey: "key"}, factory)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to retrieve workspaces")
}

// TestListWorkspacesNilResult tests workspace listing with nil result
func TestListWorkspacesNilResult(t *testing.T) {
	factory := mockClientFactory(nil, nil)

	result, err := listWorkspacesWithFactory(blaxel.Credentials{APIKey: "key"}, factory)
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "failed to retrieve workspaces for your account")
}

// TestListWorkspacesEmptyList tests workspace listing with empty list
func TestListWorkspacesEmptyList(t *testing.T) {
	workspaces := []blaxel.Workspace{}
	factory := mockClientFactory(&workspaces, nil)

	result, err := listWorkspacesWithFactory(blaxel.Credentials{APIKey: "key"}, factory)
	require.NoError(t, err)
	assert.Empty(t, result)
}

// TestListWorkspacesWithAccessToken tests listing with access token
func TestListWorkspacesWithAccessToken(t *testing.T) {
	workspaces := []blaxel.Workspace{
		{Name: "workspace-1"},
	}
	factory := mockClientFactory(&workspaces, nil)

	result, err := listWorkspacesWithFactory(blaxel.Credentials{AccessToken: "token"}, factory)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

// TestListWorkspacesWithClientCredentials tests listing with client credentials
func TestListWorkspacesWithClientCredentials(t *testing.T) {
	workspaces := []blaxel.Workspace{
		{Name: "workspace-1"},
	}
	factory := mockClientFactory(&workspaces, nil)

	result, err := listWorkspacesWithFactory(blaxel.Credentials{ClientCredentials: "creds"}, factory)
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

// TestValidateWorkspaceWithAccessToken tests validation with access token
func TestValidateWorkspaceWithAccessToken(t *testing.T) {
	workspaces := []blaxel.Workspace{{Name: "ws"}}
	factory := mockClientFactory(&workspaces, nil)

	err := validateWorkspaceWithFactory("ws", blaxel.Credentials{AccessToken: "token"}, factory)
	require.NoError(t, err)
}

// TestValidateWorkspaceWithClientCredentials tests validation with client credentials
func TestValidateWorkspaceWithClientCredentials(t *testing.T) {
	workspaces := []blaxel.Workspace{{Name: "ws"}}
	factory := mockClientFactory(&workspaces, nil)

	err := validateWorkspaceWithFactory("ws", blaxel.Credentials{ClientCredentials: "creds"}, factory)
	require.NoError(t, err)
}

// TestSetAndResetClientFactory tests the factory setter and resetter
func TestSetAndResetClientFactory(t *testing.T) {
	// Save original
	original := defaultClientFactory

	// Set a mock factory
	workspaces := []blaxel.Workspace{{Name: "test"}}
	SetClientFactory(mockClientFactory(&workspaces, nil))

	// Verify it's set (by calling the function which should work with mock)
	err := ValidateWorkspace("test", blaxel.Credentials{APIKey: "key"})
	require.NoError(t, err)

	// Reset
	ResetClientFactory()

	// Restore original for other tests
	defaultClientFactory = original
}

// TestPublicFunctions tests the public wrapper functions
func TestPublicFunctions(t *testing.T) {
	// Save original factory
	original := defaultClientFactory
	defer func() { defaultClientFactory = original }()

	workspaces := []blaxel.Workspace{
		{Name: "test-ws"},
	}
	SetClientFactory(mockClientFactory(&workspaces, nil))

	// Test ValidateWorkspace
	err := ValidateWorkspace("test-ws", blaxel.Credentials{APIKey: "key"})
	require.NoError(t, err)

	// Test ListWorkspaces
	result, err := ListWorkspaces(blaxel.Credentials{APIKey: "key"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

// TestLegacyFunctions tests backward compatibility functions
func TestLegacyFunctions(t *testing.T) {
	// Save original factory
	original := defaultClientFactory
	defer func() { defaultClientFactory = original }()

	workspaces := []blaxel.Workspace{
		{Name: "test-ws"},
	}
	SetClientFactory(mockClientFactory(&workspaces, nil))

	// Test validateWorkspace (lowercase)
	err := validateWorkspace("test-ws", blaxel.Credentials{APIKey: "key"})
	require.NoError(t, err)

	// Test listWorkspaces (lowercase)
	result, err := listWorkspaces(blaxel.Credentials{APIKey: "key"})
	require.NoError(t, err)
	assert.Len(t, result, 1)
}

// TestCredentialsIsValid tests the IsValid method on credentials
func TestCredentialsIsValid(t *testing.T) {
	tests := []struct {
		name     string
		creds    blaxel.Credentials
		expected bool
	}{
		{"Empty", blaxel.Credentials{}, false},
		{"APIKey", blaxel.Credentials{APIKey: "key"}, true},
		{"AccessToken", blaxel.Credentials{AccessToken: "token"}, true},
		{"ClientCredentials", blaxel.Credentials{ClientCredentials: "creds"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.creds.IsValid())
		})
	}
}

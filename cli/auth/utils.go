package auth

import (
	"context"
	"fmt"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
)

// WorkspaceLister interface for listing workspaces (allows mocking)
type WorkspaceLister interface {
	List(ctx context.Context, opts ...option.RequestOption) (*[]blaxel.Workspace, error)
}

// ClientFactory creates clients for workspace operations
type ClientFactory func(opts ...option.RequestOption) WorkspaceLister

// defaultClientFactory creates the real blaxel client
var defaultClientFactory ClientFactory = func(opts ...option.RequestOption) WorkspaceLister {
	client := blaxel.NewClient(opts...)
	return &client.Workspaces
}

// SetClientFactory allows setting a custom client factory for testing
func SetClientFactory(factory ClientFactory) {
	defaultClientFactory = factory
}

// ResetClientFactory resets to the default client factory
func ResetClientFactory() {
	defaultClientFactory = func(opts ...option.RequestOption) WorkspaceLister {
		client := blaxel.NewClient(opts...)
		return &client.Workspaces
	}
}

// BuildClientOptions builds request options from credentials
func BuildClientOptions(workspace string, credentials blaxel.Credentials) []option.RequestOption {
	opts := []option.RequestOption{
		option.WithBaseURL(blaxel.GetBaseURL()),
	}

	if workspace != "" {
		opts = append(opts, option.WithWorkspace(workspace))
	}

	if credentials.APIKey != "" {
		opts = append(opts, option.WithAPIKey(credentials.APIKey))
	} else if credentials.AccessToken != "" {
		opts = append(opts, option.WithAccessToken(credentials.AccessToken))
	} else if credentials.ClientCredentials != "" {
		opts = append(opts, option.WithClientCredentials(credentials.ClientCredentials))
	}

	return opts
}

// ValidateWorkspace validates workspace credentials
func ValidateWorkspace(workspace string, credentials blaxel.Credentials) error {
	return validateWorkspaceWithFactory(workspace, credentials, defaultClientFactory)
}

// validateWorkspaceWithFactory validates workspace using a custom client factory
func validateWorkspaceWithFactory(workspace string, credentials blaxel.Credentials, factory ClientFactory) error {
	opts := BuildClientOptions(workspace, credentials)
	client := factory(opts...)

	// Try to make a simple API call to validate
	_, err := client.List(context.Background())
	if err != nil {
		return fmt.Errorf("failed to validate workspace credentials: %w", err)
	}

	return nil
}

// ListWorkspaces lists all workspaces for the given credentials
func ListWorkspaces(credentials blaxel.Credentials) ([]blaxel.Workspace, error) {
	return listWorkspacesWithFactory(credentials, defaultClientFactory)
}

// listWorkspacesWithFactory lists workspaces using a custom client factory
func listWorkspacesWithFactory(credentials blaxel.Credentials, factory ClientFactory) ([]blaxel.Workspace, error) {
	opts := BuildClientOptions("", credentials)
	client := factory(opts...)

	// Try to make a simple API call to validate
	workspaces, err := client.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve workspaces: %w", err)
	}
	if workspaces == nil {
		return nil, fmt.Errorf("failed to retrieve workspaces for your account")
	}
	return *workspaces, nil
}

// Legacy function names for backward compatibility
func validateWorkspace(workspace string, credentials blaxel.Credentials) error {
	return ValidateWorkspace(workspace, credentials)
}

func listWorkspaces(credentials blaxel.Credentials) ([]blaxel.Workspace, error) {
	return ListWorkspaces(credentials)
}

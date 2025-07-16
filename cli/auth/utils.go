package auth

import (
	"context"
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
)

// Helper function to validate workspace
func validateWorkspace(workspace string, credentials sdk.Credentials) error {
	// Create a temporary client to validate the workspace
	c, err := sdk.NewClientWithCredentials(
		sdk.RunClientWithCredentials{
			ApiURL:      core.GetBaseURL(),
			RunURL:      core.GetRunURL(),
			Credentials: credentials,
			Workspace:   workspace,
		},
	)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	// Try to make a simple API call to validate
	_, err = c.ListWorkspacesWithResponse(context.Background())
	if err != nil {
		return fmt.Errorf("failed to validate workspace credentials: %w", err)
	}

	return nil
}

// Helper function to validate workspace
func listWorkspaces(credentials sdk.Credentials) ([]sdk.Workspace, error) {
	// Create a temporary client to validate the workspace
	c, err := sdk.NewClientWithCredentials(
		sdk.RunClientWithCredentials{
			ApiURL:      core.GetBaseURL(),
			RunURL:      core.GetRunURL(),
			Credentials: credentials,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %w", err)
	}

	// Try to make a simple API call to validate
	response, err := c.ListWorkspacesWithResponse(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to validate workspace credentials: %w", err)
	}
	if response.JSON200 == nil {
		return nil, fmt.Errorf("failed to retrieve workspaces for your account")
	}
	return *response.JSON200, nil
}

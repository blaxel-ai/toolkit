package auth

import (
	"context"
	"fmt"
	"os"

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
		return fmt.Errorf("failed to validate workspace: %w", err)
	}

	return nil
}

// Helper function to handle login success
//
//nolint:unused
func handleLoginSuccess(workspace string, credentials sdk.Credentials) {
	err := saveCredentials(workspace, credentials)
	if err != nil {
		fmt.Printf("Error saving credentials: %v\n", err)
		os.Exit(1)
	}
}

// Helper function to save credentials
//
//nolint:unused
func saveCredentials(workspace string, credentials sdk.Credentials) error {
	sdk.SaveCredentials(workspace, credentials)

	// Set as current context
	sdk.SetCurrentWorkspace(workspace)

	fmt.Printf("Successfully logged in to workspace %s\n", workspace)
	return nil
}

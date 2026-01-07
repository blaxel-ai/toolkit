package auth

import (
	"context"
	"fmt"

	blaxel "github.com/stainless-sdks/blaxel-go"
	"github.com/stainless-sdks/blaxel-go/option"
)

// Helper function to validate workspace
func validateWorkspace(workspace string, credentials blaxel.Credentials) error {
	// Build client options based on credentials
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

	c := blaxel.NewClient(opts...)

	// Try to make a simple API call to validate
	_, err := c.Workspaces.List(context.Background())
	if err != nil {
		return fmt.Errorf("failed to validate workspace credentials: %w", err)
	}

	return nil
}

// Helper function to list workspaces
func listWorkspaces(credentials blaxel.Credentials) ([]blaxel.Workspace, error) {
	// Build client options based on credentials
	opts := []option.RequestOption{
		option.WithBaseURL(blaxel.GetBaseURL()),
	}

	if credentials.APIKey != "" {
		opts = append(opts, option.WithAPIKey(credentials.APIKey))
	} else if credentials.AccessToken != "" {
		opts = append(opts, option.WithAccessToken(credentials.AccessToken))
	} else if credentials.ClientCredentials != "" {
		opts = append(opts, option.WithClientCredentials(credentials.ClientCredentials))
	}

	c := blaxel.NewClient(opts...)

	// Try to make a simple API call to validate
	workspaces, err := c.Workspaces.List(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve workspaces: %w", err)
	}
	if workspaces == nil {
		return nil, fmt.Errorf("failed to retrieve workspaces for your account")
	}
	return *workspaces, nil
}

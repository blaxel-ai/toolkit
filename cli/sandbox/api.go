package sandbox

import (
	"context"
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
)

// GetSandboxURL fetches the sandbox metadata from the control plane and returns its direct URL
func GetSandboxURL(ctx context.Context, workspace, sandboxName string) (string, error) {
	// Get or create the API client
	client := core.GetClient()
	if client == nil {
		// Initialize client if not already done
		credentials := sdk.LoadCredentials(workspace)
		if !credentials.IsValid() {
			return "", fmt.Errorf("no valid credentials found for workspace '%s'", workspace)
		}

		var err error
		client, err = sdk.NewClientWithCredentials(sdk.RunClientWithCredentials{
			ApiURL:      core.GetBaseURL(),
			RunURL:      core.GetRunURL(),
			Credentials: credentials,
			Workspace:   workspace,
		})
		if err != nil {
			return "", fmt.Errorf("failed to create API client: %w", err)
		}
	}

	// Get the sandbox by name
	response, err := client.GetSandboxWithResponse(ctx, sandboxName, &sdk.GetSandboxParams{})
	if err != nil {
		return "", fmt.Errorf("error getting sandbox: %w", err)
	}

	if response.StatusCode() == 404 {
		return "", fmt.Errorf("sandbox '%s' not found", sandboxName)
	}

	if response.StatusCode() != 200 {
		return "", fmt.Errorf("error getting sandbox: %s", response.Status())
	}

	// Extract the URL from metadata
	sandbox := response.JSON200
	if sandbox != nil && sandbox.Metadata != nil && sandbox.Metadata.Url != nil && *sandbox.Metadata.Url != "" {
		return *sandbox.Metadata.Url, nil
	}

	// Fallback to constructing the URL if not available in metadata
	return fmt.Sprintf("%s/%s/sandboxes/%s", core.GetRunURL(), workspace, sandboxName), nil
}

// NewSandboxClient creates a new sandbox client, fetching the URL from the control plane
func NewSandboxClient(workspace, sandboxName string) (*SandboxClient, error) {
	ctx := context.Background()

	// Get the sandbox URL from the API
	serverURL, err := GetSandboxURL(ctx, workspace, sandboxName)
	if err != nil {
		return nil, fmt.Errorf("failed to get sandbox URL: %w", err)
	}

	// Prepare authentication headers
	authHeaders := make(map[string]string)
	credentials := sdk.LoadCredentials(workspace)
	if !credentials.IsValid() {
		return nil, fmt.Errorf("no valid credentials found for workspace '%s'", workspace)
	}

	// Get the auth provider to get properly formatted headers
	authProvider := sdk.GetAuthProvider(credentials, workspace, core.GetBaseURL())
	headers, err := authProvider.GetHeaders()
	if err != nil {
		return nil, fmt.Errorf("failed to get auth headers: %w", err)
	}

	// Convert headers for sandbox use
	if apiKey, ok := headers["X-Blaxel-Authorization"]; ok {
		// For sandboxes, use standard Authorization header
		authHeaders["Authorization"] = apiKey
	} else if credentials.APIKey != "" {
		authHeaders["X-Blaxel-Api-Key"] = credentials.APIKey
	} else if credentials.AccessToken != "" {
		authHeaders["Authorization"] = "Bearer " + credentials.AccessToken
	} else if credentials.ClientCredentials != "" {
		authHeaders["Authorization"] = "Basic " + credentials.ClientCredentials
	}

	// Create the client with the direct URL
	return NewSandboxClientWithURL(workspace, sandboxName, serverURL, authHeaders)
}

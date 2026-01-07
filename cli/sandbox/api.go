package sandbox

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	blaxel "github.com/stainless-sdks/blaxel-go"
	"github.com/stainless-sdks/blaxel-go/option"
)

// GetSandboxURL fetches the sandbox metadata from the control plane and returns its direct URL
func GetSandboxURL(ctx context.Context, workspace, sandboxName string) (string, error) {
	// Get or create the API client
	client := core.GetClient()
	if client == nil {
		// Initialize client if not already done
		credentials, _ := blaxel.LoadCredentials(workspace)
		if !credentials.IsValid() {
			return "", fmt.Errorf("no valid credentials found for workspace '%s'", workspace)
		}

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
		client = &c
	}

	// Get the sandbox by name
	sandbox, err := client.Sandboxes.Get(ctx, sandboxName, blaxel.SandboxGetParams{})
	if err != nil {
		var apiErr *blaxel.Error
		if ok := isBlaxelError(err, &apiErr); ok && apiErr.StatusCode == 404 {
			return "", fmt.Errorf("sandbox '%s' not found", sandboxName)
		}
		return "", fmt.Errorf("error getting sandbox: %w", err)
	}

	// Extract the URL from metadata via JSON
	jsonData, _ := json.Marshal(sandbox)
	var sbMap map[string]interface{}
	json.Unmarshal(jsonData, &sbMap)

	if metadata, ok := sbMap["metadata"].(map[string]interface{}); ok {
		if url, ok := metadata["url"].(string); ok && url != "" {
			return url, nil
		}
	}

	// Fallback to constructing the URL if not available in metadata
	return blaxel.BuildSandboxURL(workspace, sandboxName), nil
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
	credentials, _ := blaxel.LoadCredentials(workspace)
	if !credentials.IsValid() {
		return nil, fmt.Errorf("no valid credentials found for workspace '%s'", workspace)
	}

	// Get authentication header based on credential type
	if credentials.APIKey != "" {
		authHeaders["X-Blaxel-Authorization"] = "Bearer " + credentials.APIKey
	} else if credentials.AccessToken != "" {
		authHeaders["Authorization"] = "Bearer " + credentials.AccessToken
	} else if credentials.ClientCredentials != "" {
		authHeaders["Authorization"] = "Basic " + credentials.ClientCredentials
	}

	// Create the client with the direct URL
	return NewSandboxClientWithURL(workspace, sandboxName, serverURL, authHeaders)
}

// isBlaxelError checks if an error is a blaxel API error and sets the apiErr pointer
func isBlaxelError(err error, apiErr **blaxel.Error) bool {
	if e, ok := err.(*blaxel.Error); ok {
		*apiErr = e
		return true
	}
	return false
}

package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("token", func() *cobra.Command {
		return TokenCmd()
	})
}

var authHeadersForCredentials = func(ctx context.Context, credentials blaxel.Credentials, workspace string) (map[string]string, error) {
	return credentials.AuthHeaders(ctx, workspace)
}

func TokenCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "token [workspace]",
		Short: "Retrieve authentication token for a workspace",
		Long: `Retrieve the authentication token for the specified workspace.

The token command displays the current authentication token used by the CLI
for API requests. This token is automatically managed and refreshed as needed.

Authentication Methods:
- API Key: Returns the API key
- OAuth (Browser Login): Returns the access token (refreshed if needed)
- Client Credentials: Returns the access token (refreshed if needed)

The token is retrieved from your stored credentials and will be automatically
refreshed if it's expired or about to expire.

Examples:

` + "```bash" + `
# Get token for current workspace
bl token

# Get token for specific workspace
bl token my-workspace

# Use in scripts (get just the token value)
export TOKEN=$(bl token)
` + "```" + ``,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Determine workspace
			workspace := core.GetWorkspace()
			if len(args) > 0 {
				workspace = args[0]
			}

			// If no workspace specified, use current context
			if workspace == "" {
				ctx, _ := blaxel.CurrentContext()
				workspace = ctx.Workspace
			}

			// Validate workspace
			if workspace == "" {
				err := fmt.Errorf("no workspace specified. Use 'bl login <workspace>' to authenticate")
				core.PrintError("token", err)
				core.ExitWithError(err)
			}

			// Get workspace to check if access is allowed + it refreshes the token if needed.
			client, err := blaxel.NewClientFromConfig(workspace)
			if err != nil {
				err := fmt.Errorf("failed to create client for workspace '%s': %w", workspace, err)
				core.PrintError("token", err)
				core.ExitWithError(err)
			}
			_, err = client.Workspaces.Get(context.Background(), workspace, blaxel.WorkspaceGetParams{})
			if err != nil {
				err := fmt.Errorf("failed to get workspace '%s': %w", workspace, err)
				core.PrintError("token", err)
				core.ExitWithError(err)
			}

			// Load credentials for the workspace
			credentials, err := blaxel.LoadCredentials(workspace)
			if err != nil {
				err := fmt.Errorf("failed to load credentials for workspace '%s': %w", workspace, err)
				core.PrintError("token", err)
				core.ExitWithError(err)
			}
			if !credentials.IsValid() {
				err := fmt.Errorf("no valid credentials found for workspace '%s'. Please run 'bl login %s'", workspace, workspace)
				core.PrintError("token", err)
				core.ExitWithError(err)
			}

			token, err := tokenForCredentials(context.Background(), workspace, credentials)
			if err != nil {
				err := fmt.Errorf("failed to retrieve token for workspace '%s': %w", workspace, err)
				core.PrintError("token", err)
				core.ExitWithError(err)
			}

			// Output the token
			fmt.Println(token)
		},
	}

	return cmd
}

func tokenForCredentials(ctx context.Context, workspace string, credentials blaxel.Credentials) (string, error) {
	headers, err := authHeadersForCredentials(ctx, credentials, workspace)
	if err != nil {
		return "", err
	}

	token := bearerTokenFromHeaders(headers)
	if token == "" {
		return "", fmt.Errorf("no bearer token found in credentials")
	}

	if expired, ok := jwtExpired(token, time.Now()); ok && expired && credentials.APIKey == "" {
		return "", fmt.Errorf("access token is expired and could not be refreshed. Please run 'bl login %s'", workspace)
	}

	return token, nil
}

func bearerTokenFromHeaders(headers map[string]string) string {
	for _, name := range []string{"Authorization", "X-Blaxel-Authorization"} {
		value := strings.TrimSpace(headers[name])
		if token, ok := strings.CutPrefix(value, "Bearer "); ok {
			return strings.TrimSpace(token)
		}
	}
	return ""
}

func jwtExpired(token string, now time.Time) (bool, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return false, false
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return false, false
	}

	var claims struct {
		Exp float64 `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == 0 {
		return false, false
	}

	expiresAt := time.Unix(int64(claims.Exp), 0)
	return !expiresAt.After(now), true
}

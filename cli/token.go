package cli

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("token", func() *cobra.Command {
		return TokenCmd()
	})
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
  # Get token for current workspace
  bl token

  # Get token for specific workspace
  bl token my-workspace

  # Use in scripts (get just the token value)
  export TOKEN=$(bl token)`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// Determine workspace
			workspace := core.GetWorkspace()
			if len(args) > 0 {
				workspace = args[0]
			}

			// If no workspace specified, use current context
			if workspace == "" {
				workspace = sdk.CurrentContext().Workspace
			}

			// Validate workspace
			if workspace == "" {
				err := fmt.Errorf("no workspace specified. Use 'bl login <workspace>' to authenticate")
				core.PrintError("token", err)
				core.ExitWithError(err)
			}

			// Load credentials for the workspace
			credentials := sdk.LoadCredentials(workspace)
			if !credentials.IsValid() {
				err := fmt.Errorf("no valid credentials found for workspace '%s'. Please run 'bl login %s'", workspace, workspace)
				core.PrintError("token", err)
				core.ExitWithError(err)
			}

			// Get auth provider
			authProvider := sdk.GetAuthProvider(credentials, workspace, core.GetBaseURL())

			// Get headers (which will trigger token refresh if needed)
			headers, err := authProvider.GetHeaders()
			if err != nil {
				err = fmt.Errorf("failed to retrieve token: %v", err)
				core.PrintError("token", err)
				core.ExitWithError(err)
			}

			// Extract token from headers
			token := ""
			if authHeader, ok := headers["X-Blaxel-Authorization"]; ok {
				// Remove "Bearer " prefix if present
				if len(authHeader) > 7 && authHeader[:7] == "Bearer " {
					token = authHeader[7:]
				} else {
					token = authHeader
				}
			}

			if token == "" {
				err := fmt.Errorf("no token found in authentication headers")
				core.PrintError("token", err)
				core.ExitWithError(err)
			}

			// Output the token
			fmt.Println(token)
		},
	}

	return cmd
}

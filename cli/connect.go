package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/connect"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func init() {
	core.RegisterCommand("connect", func() *cobra.Command {
		return ConnectCmd()
	})
}

func ConnectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect into your sandbox resources",
		Long:  "Connect into your sandbox resources with interactive interfaces",
	}

	cmd.AddCommand(ConnectSandboxCmd())
	return cmd
}

func ConnectSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "sandbox [sandbox-name]",
		Aliases: []string{"sb", "sbx"},
		Short:   "Connect to a sandbox environment",
		Long: `Connect to a sandbox environment with an interactive terminal session.

This command opens a direct terminal connection to your sandbox, similar to SSH.
The terminal supports full ANSI colors, cursor movement, and interactive applications.

Press Ctrl+D to disconnect from the sandbox.

Examples:
  bl connect sandbox my-sandbox
  bl connect sb my-sandbox
  bl connect sbx production-env`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sandboxName := args[0]

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			// Check if stdin is a terminal
			if !term.IsTerminal(int(os.Stdin.Fd())) {
				err := fmt.Errorf("this command requires an interactive terminal")
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			// Get the current workspace
			currentContext := sdk.CurrentContext()
			workspace := currentContext.Workspace
			if workspace == "" {
				err := fmt.Errorf("no workspace found in current context. Please run 'bl login' first")
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			// Load credentials
			credentials := sdk.LoadCredentials(workspace)
			if !credentials.IsValid() {
				err := fmt.Errorf("no valid credentials found. Please run 'bl login' first")
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			// Refresh token if needed before connecting to websocket
			apiURL := core.GetBaseURL()
			authProvider := sdk.GetAuthProvider(credentials, workspace, apiURL)

			// Try to refresh token based on auth provider type
			var token string
			switch p := authProvider.(type) {
			case *sdk.BearerToken:
				if err := p.RefreshIfNeeded(); err != nil {
					core.PrintError("Connect", fmt.Errorf("failed to refresh token: %w", err))
					core.ExitWithError(err)
				}
				token = p.GetCredentials().AccessToken
			case *sdk.ClientCredentials:
				if err := p.RefreshIfNeeded(); err != nil {
					core.PrintError("Connect", fmt.Errorf("failed to refresh token: %w", err))
					core.ExitWithError(err)
				}
				token = p.GetCredentials().AccessToken
			default:
				// Fallback for API key or other providers
				token = credentials.AccessToken
				if token == "" {
					token = credentials.APIKey
				}
			}

			if token == "" {
				err := fmt.Errorf("no access token or API key found. Please run 'bl login' first")
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			// Get the sandbox to retrieve its URL
			client := core.GetClient()
			response, err := client.GetSandboxWithResponse(ctx, sandboxName, &sdk.GetSandboxParams{})
			if err != nil {
				err = fmt.Errorf("error getting sandbox: %w", err)
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			if response.StatusCode() == 404 {
				err := fmt.Errorf("sandbox '%s' not found", sandboxName)
				core.PrintError("Connect", err)

				// List available sandboxes
				listResponse, listErr := client.ListSandboxesWithResponse(ctx)
				if listErr == nil && listResponse.StatusCode() == 200 && listResponse.JSON200 != nil {
					sandboxes := *listResponse.JSON200
					if len(sandboxes) > 0 {
						names := make([]string, 0, len(sandboxes))
						for _, sb := range sandboxes {
							names = append(names, sb.Metadata.Name)
						}
						if len(names) > 0 {
							core.Print(fmt.Sprintf("Available sandboxes: %s\n", strings.Join(names, ", ")))
						}
					}
				}
				core.Print(fmt.Sprintf("Create a new sandbox here: %s/%s/global-agentic-network/sandboxes\n", core.GetAppURL(), workspace))
				core.ExitWithError(err)
			}

			if response.StatusCode() != 200 {
				err := fmt.Errorf("error getting sandbox: %s", response.Status())
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

		// Get sandbox URL
		sandboxURL := ""
		if response.JSON200 != nil && response.JSON200.Metadata.Url != nil {
			sandboxURL = *response.JSON200.Metadata.Url
		}
		if sandboxURL == "" {
			sandboxURL = fmt.Sprintf("%s/%s/sandboxes/%s", core.GetRunURL(), workspace, sandboxName)
		}

			// Clear the terminal before connecting
			fmt.Print("\033[2J\033[H")

			// Print connection info
			core.Print(fmt.Sprintf("Connecting to sandbox '%s'...\n", sandboxName))
			core.Print("Press Ctrl+D to disconnect\n\n")

			// Create and run terminal client
			terminalClient, err := connect.NewTerminalClient(sandboxURL, token)
			if err != nil {
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}
			defer terminalClient.Close()

			// Run the terminal session (blocks until exit)
			if err := terminalClient.Run(ctx); err != nil {
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			core.Print("\nDisconnected from sandbox.\n")
		},
	}

	return cmd
}

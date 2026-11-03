package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/connect"
	"github.com/blaxel-ai/toolkit/cli/core"
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
		Short: "Open an interactive terminal session to a sandbox",
		Long:  "Open an interactive terminal session to a sandbox",
	}

	cmd.AddCommand(ConnectSandboxCmd())
	return cmd
}

func ConnectSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "sandbox [sandbox-name]",
		Aliases:           []string{"sb", "sbx"},
		Short:             "Connect to a sandbox environment",
		ValidArgsFunction: GetConnectSandboxValidArgsFunction(),
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
				err := core.MarkExpectedError(
					fmt.Errorf("this command requires an interactive terminal"),
					core.CLIErrorUsage,
				)
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			workspace, token, err := sandboxAuthentication(ctx)
			if err != nil {
				err = core.MarkExpectedError(err, core.CLIErrorAuthentication)
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			// Get the sandbox to retrieve its URL
			client := core.GetClient()
			sbx, err := client.Sandboxes.Get(ctx, sandboxName, blaxel.SandboxGetParams{})
			if err != nil {
				var apiErr *blaxel.Error
				if isBlaxelError(err, &apiErr) && apiErr.StatusCode == 404 {
					err := core.MarkExpectedError(
						fmt.Errorf("sandbox '%s' not found", sandboxName),
						core.CLIErrorNotFound,
					)
					core.PrintError("Connect", err)

					// List available sandboxes
					sandboxes, listErr := client.Sandboxes.List(ctx, blaxel.SandboxListParams{})
					if listErr == nil && sandboxes != nil && len(sandboxes.Data) > 0 {
						names := make([]string, 0, len(sandboxes.Data))
						for _, sb := range sandboxes.Data {
							if sb.Metadata.Name != "" {
								names = append(names, sb.Metadata.Name)
							}
						}
						if len(names) > 0 {
							core.Print(fmt.Sprintf("Available sandboxes: %s\n", strings.Join(names, ", ")))
						}
					}
					core.Print(fmt.Sprintf("Create a new sandbox here: %s/%s/global-agentic-network/sandboxes\n", blaxel.GetAppURL(), workspace))
					core.ExitWithError(err)
				}
				err = fmt.Errorf("error getting sandbox: %w", err)
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			// Build the terminal URL
			sandboxURL := sbx.Metadata.URL
			if sandboxURL == "" {
				sandboxURL = blaxel.BuildSandboxURL(workspace, sandboxName)
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

var loadStoredCredentials = blaxel.LoadCredentials

// workspaceCredentials is the stored login for the workspace or, like the API
// client behind bl get and bl apply, BL_API_KEY or BL_CLIENT_CREDENTIALS when
// the workspace has none.
func workspaceCredentials(workspace string) (blaxel.Credentials, error) {
	credentials, err := loadStoredCredentials(workspace)
	if err != nil {
		return blaxel.Credentials{}, err
	}
	if !credentials.IsValid() {
		credentials = environmentCredentials(os.Getenv)
	}
	return credentials, nil
}

// sandboxAuthentication is the selected workspace (-w, BL_WORKSPACE, or the
// current context) and a bearer token for its sandboxes, for bl connect and
// bl drive.
func sandboxAuthentication(ctx context.Context) (string, string, error) {
	workspace := core.GetWorkspace()
	if workspace == "" {
		return "", "", fmt.Errorf("no workspace selected. Please run 'bl login' first")
	}
	credentials, err := workspaceCredentials(workspace)
	if err != nil {
		return "", "", fmt.Errorf("failed to load credentials for workspace %q: %w", workspace, err)
	}
	if !credentials.IsValid() {
		return "", "", fmt.Errorf("no valid credentials found. Please run 'bl login %s', or set BL_API_KEY", workspace)
	}
	token, err := tokenForCredentials(ctx, workspace, credentials)
	if err != nil {
		return "", "", err
	}
	return workspace, token, nil
}

// environmentCredentials are the credentials set in BL_API_KEY, else
// BL_CLIENT_CREDENTIALS, or none.
func environmentCredentials(getenv func(string) string) blaxel.Credentials {
	if key := strings.TrimSpace(getenv("BL_API_KEY")); key != "" {
		return blaxel.Credentials{APIKey: key}
	}
	if clientCredentials := strings.TrimSpace(getenv("BL_CLIENT_CREDENTIALS")); clientCredentials != "" {
		return blaxel.Credentials{ClientCredentials: clientCredentials}
	}
	return blaxel.Credentials{}
}

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/sandbox"
	"github.com/blaxel-ai/toolkit/sdk"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
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
	var debug bool
	var url string

	cmd := &cobra.Command{
		Use:     "sandbox [sandbox-name]",
		Aliases: []string{"sb", "sbx"},
		Short:   "Connect to a sandbox environment",
		Long: `Connect to a sandbox environment using an interactive shell interface.

This command provides a terminal-like interface for:
- Executing commands in the sandbox
- Browsing files and directories
- Managing the sandbox environment

The shell connects to your sandbox via MCP (Model Control Protocol) over WebSocket.

Limitations:
- Interactive commands (vim, nano, less, top) are not supported
- Long-running commands may experience timeouts or interruptions
- Use non-interactive alternatives (cat, echo, ps) instead

Keyboard Shortcuts:
- Enter: Execute command
- ↑/↓: Navigate command history
- Ctrl+L: Clear screen
- Ctrl+C: Exit sandbox shell

Examples:
  bl connect sandbox my-sandbox
  bl connect sb my-sandbox
  bl connect sandbox production-env
  bl connect sandbox my-sandbox --url wss://custom.domain.com/sandbox/my-sandbox`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sandboxName := args[0]

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			// Get the current workspace from SDK context
			currentContext := sdk.CurrentContext()
			workspace := currentContext.Workspace
			if workspace == "" {
				core.PrintError("Connect", fmt.Errorf("no workspace found in current context. Please run 'bl login' first"))
				os.Exit(1)
			}

			if url == "" {
				client := core.GetClient()
				// First, try to get the specific sandbox directly (more efficient)
				response, err := client.GetSandboxWithResponse(ctx, sandboxName, &sdk.GetSandboxParams{})
				if err != nil {
					core.PrintError("Connect", fmt.Errorf("error getting sandbox: %w", err))
					os.Exit(1)
				}

				if response.StatusCode() == 200 {
					// Sandbox found, get the URL from metadata
					if response.JSON200.Metadata.Url != nil && *response.JSON200.Metadata.Url != "" {
						url = *response.JSON200.Metadata.Url
					}
				} else if response.StatusCode() == 404 {
					// Sandbox not found, provide helpful error message with available sandboxes
					core.PrintError("Connect", fmt.Errorf("sandbox '%s' not found", sandboxName))

					// Now list sandboxes to show available options
					listResponse, listErr := client.ListSandboxesWithResponse(ctx)
					if listErr == nil && listResponse.StatusCode() == 200 && listResponse.JSON200 != nil {
						sandboxes := *listResponse.JSON200
						if len(sandboxes) > 0 {
							names := make([]string, 0, len(sandboxes))
							for _, sandbox := range sandboxes {
								if sandbox.Metadata != nil && sandbox.Metadata.Name != nil {
									names = append(names, *sandbox.Metadata.Name)
								}
							}
							if len(names) > 0 {
								core.Print(fmt.Sprintf("Available sandboxes: %s\n", strings.Join(names, ", ")))
							}
						}
					}
					core.Print(fmt.Sprintf("Create a new sandbox here: https://app.blaxel.ai/%s/global-agentic-network/sandboxes\n", workspace))
					os.Exit(1)
				} else {
					// Other error
					core.PrintError("Connect", fmt.Errorf("error getting sandbox: %s", response.Status()))
					os.Exit(1)
				}
			}
			// Prepare authentication headers based on available credentials
			authHeaders := make(map[string]string)
			// Load credentials for the workspace
			credentials := sdk.LoadCredentials(workspace)
			if !credentials.IsValid() {
				core.PrintError("Connect", fmt.Errorf("no valid credentials found. Please run 'bl login' first"))
				os.Exit(1)
			}
			if credentials.APIKey != "" {
				authHeaders["X-Blaxel-Api-Key"] = credentials.APIKey
			} else if credentials.AccessToken != "" {
				authHeaders["Authorization"] = "Bearer " + credentials.AccessToken
			} else if credentials.ClientCredentials != "" {
				authHeaders["Authorization"] = "Basic " + credentials.ClientCredentials
			}

			// Use default URL if none provided
			if url == "" {
				url = fmt.Sprintf("%s/%s/sandboxes/%s", core.GetRunURL(), workspace, sandboxName)
			}

			// Create the MCP-based sandbox shell with custom URL
			shell, err := sandbox.NewSandboxShellWithURL(ctx, workspace, sandboxName, url, authHeaders)
			if err != nil {
				core.PrintError("Connect", fmt.Errorf("failed to connect to sandbox: %w", err))
				os.Exit(1)
			}

			// Initialize and run the Bubble Tea program
			p := tea.NewProgram(shell, tea.WithAltScreen(), tea.WithMouseCellMotion())

			if debug {
				core.Print("Debug: Starting shell interface...")
			}

			core.SetInteractiveMode(true)
			if _, err := p.Run(); err != nil {
				core.SetInteractiveMode(false)
				core.PrintError("Connect", fmt.Errorf("failed to run sandbox connection: %w", err))
				os.Exit(1)
			}
			core.SetInteractiveMode(false)
		},
	}

	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode")
	cmd.Flags().StringVar(&url, "url", "", "Custom WebSocket URL for MCP connection (defaults to wss://run.blaxel.ai/$WORKSPACE/sandboxes/$SANDBOX_NAME)")
	return cmd
}

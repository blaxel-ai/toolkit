package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/sandbox"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	blaxel "github.com/stainless-sdks/blaxel-go"
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
			currentContext, _ := blaxel.CurrentContext()
			workspace := currentContext.Workspace
			if workspace == "" {
				err := fmt.Errorf("no workspace found in current context. Please run 'bl login' first")
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			if url == "" {
				client := core.GetClient()
				// First, try to get the specific sandbox directly (more efficient)
				sbx, err := client.Sandboxes.Get(ctx, sandboxName, blaxel.SandboxGetParams{})
				if err != nil {
					// Check if it's a 404 error
					var apiErr *blaxel.Error
					if isBlaxelError(err, &apiErr) && apiErr.StatusCode == 404 {
						// Sandbox not found, provide helpful error message with available sandboxes
						err := fmt.Errorf("sandbox '%s' not found", sandboxName)
						core.PrintError("Connect", err)

						// Now list sandboxes to show available options
						sandboxes, listErr := client.Sandboxes.List(ctx)
						if listErr == nil && sandboxes != nil {
							if len(*sandboxes) > 0 {
								names := make([]string, 0, len(*sandboxes))
								for _, sb := range *sandboxes {
									// Get name from metadata via JSON
									jsonData, _ := json.Marshal(sb)
									var sbMap map[string]interface{}
									json.Unmarshal(jsonData, &sbMap)
									if metadata, ok := sbMap["metadata"].(map[string]interface{}); ok {
										if name, ok := metadata["name"].(string); ok {
											names = append(names, name)
										}
									}
								}
								if len(names) > 0 {
									core.Print(fmt.Sprintf("Available sandboxes: %s\n", strings.Join(names, ", ")))
								}
							}
						}
						core.Print(fmt.Sprintf("Create a new sandbox here: %s/%s/global-agentic-network/sandboxes\n", blaxel.GetAppURL(), workspace))
						core.ExitWithError(err)
					}
					err = fmt.Errorf("error getting sandbox: %w", err)
					core.PrintError("Connect", err)
					core.ExitWithError(err)
				}

				// Get URL from metadata via JSON
				jsonData, _ := json.Marshal(sbx)
				var sbMap map[string]interface{}
				json.Unmarshal(jsonData, &sbMap)
				if metadata, ok := sbMap["metadata"].(map[string]interface{}); ok {
					if sbURL, ok := metadata["url"].(string); ok && sbURL != "" {
						url = sbURL
					}
				}
			}
			// Prepare authentication headers based on available credentials
			authHeaders := make(map[string]string)
			// Load credentials for the workspace
			credentials, _ := blaxel.LoadCredentials(workspace)
			if !credentials.IsValid() {
				err := fmt.Errorf("no valid credentials found. Please run 'bl login' first")
				core.PrintError("Connect", err)
				core.ExitWithError(err)
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
				url = blaxel.BuildSandboxURL(workspace, sandboxName)
			}

			// Create the MCP-based sandbox shell with custom URL
			shell, err := sandbox.NewSandboxShellWithURL(ctx, workspace, sandboxName, url, authHeaders)
			if err != nil {
				err = fmt.Errorf("failed to connect to sandbox: %w", err)
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			// Initialize and run the Bubble Tea program
			p := tea.NewProgram(shell, tea.WithAltScreen(), tea.WithMouseCellMotion())

			if debug {
				core.Print("Debug: Starting shell interface...")
			}

			core.SetInteractiveMode(true)
			if _, err := p.Run(); err != nil {
				core.SetInteractiveMode(false)
				err = fmt.Errorf("failed to run sandbox connection: %w", err)
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}
			core.SetInteractiveMode(false)
		},
	}

	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode")
	cmd.Flags().StringVar(&url, "url", "", "Custom WebSocket URL for MCP connection (defaults to wss://run.blaxel.ai/$WORKSPACE/sandboxes/$SANDBOX_NAME)")
	return cmd
}

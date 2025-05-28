package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/blaxel-ai/toolkit/cli/sandbox"
	"github.com/blaxel-ai/toolkit/sdk"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func (r *Operations) ExploreCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "peek",
		Short: "Peek into your sandbox resources",
		Long:  "Peek into your sandbox resources with interactive interfaces",
	}

	cmd.AddCommand(r.ExploreSandboxCmd())
	return cmd
}

func (r *Operations) ExploreSandboxCmd() *cobra.Command {
	var debug bool
	var url string

	cmd := &cobra.Command{
		Use:   "sandbox [sandbox-name]",
		Short: "Explore a sandbox environment",
		Long: `Explore and interact with a sandbox environment using an interactive shell interface.

This command provides a terminal-like interface for:
- Executing commands in the sandbox
- Browsing files and directories
- Managing the sandbox environment

The shell connects to your sandbox via MCP (Model Control Protocol) over WebSocket.

Examples:
  bl peek sandbox my-sandbox
  bl peek sandbox production-env
  bl peek sandbox my-sandbox --url wss://custom.domain.com/sandbox/my-sandbox`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sandboxName := args[0]

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			r.ExploreSandbox(ctx, sandboxName, debug, url)
		},
	}

	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode")
	cmd.Flags().StringVar(&url, "url", "", "Custom WebSocket URL for MCP connection (defaults to wss://run.blaxel.ai/$WORKSPACE/sandboxes/$SANDBOX_NAME)")
	return cmd
}

func (r *Operations) ExploreSandbox(ctx context.Context, sandboxName string, debug bool, url string) {
	// Get the current workspace from SDK context
	currentContext := sdk.CurrentContext()
	workspace := currentContext.Workspace
	if workspace == "" {
		fmt.Println("Error: No workspace found in current context. Please run 'bl auth login' first.")
		os.Exit(1)
	}

	// Load credentials for the workspace
	credentials := sdk.LoadCredentials(workspace)
	if !credentials.IsValid() {
		fmt.Println("Error: No valid credentials found. Please run 'bl auth login' first.")
		os.Exit(1)
	}

	// Prepare authentication headers based on available credentials
	authHeaders := make(map[string]string)

	if credentials.APIKey != "" {
		authHeaders["X-Blaxel-Api-Key"] = credentials.APIKey
	} else if credentials.AccessToken != "" {
		authHeaders["Authorization"] = "Bearer " + credentials.AccessToken
	} else if credentials.ClientCredentials != "" {
		authHeaders["Authorization"] = "Basic " + credentials.ClientCredentials
	}

	// Use default URL if none provided
	if url == "" {
		url = fmt.Sprintf("wss://run.blaxel.ai/%s/sandboxes/%s", workspace, sandboxName)
	}

	if debug {
		fmt.Printf("Debug: Connecting to sandbox '%s' in workspace '%s'\n", sandboxName, workspace)
		fmt.Printf("Debug: WebSocket URL: %s\n", url)
		fmt.Printf("Debug: Authentication method: ")
		if credentials.APIKey != "" {
			fmt.Println("API Key")
		} else if credentials.AccessToken != "" {
			fmt.Println("Access Token")
		} else if credentials.ClientCredentials != "" {
			fmt.Println("Client Credentials")
		}
	}

	// Create the MCP-based sandbox shell with custom URL
	shell := sandbox.NewSandboxShellWithURL(ctx, workspace, sandboxName, url, authHeaders)

	// Initialize and run the Bubble Tea program
	p := tea.NewProgram(shell, tea.WithAltScreen(), tea.WithMouseCellMotion())

	if debug {
		fmt.Println("Debug: Starting shell interface...")
	}

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running sandbox peek: %v\n", err)
		os.Exit(1)
	}
}

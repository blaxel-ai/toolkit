package cli

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/sandbox"
	"github.com/blaxel-ai/toolkit/sdk"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func (r *Operations) ConnectCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect into your sandbox resources",
		Long:  "Connect into your sandbox resources with interactive interfaces",
	}

	cmd.AddCommand(r.ConnectSandboxCmd())
	return cmd
}

func (r *Operations) ConnectSandboxCmd() *cobra.Command {
	var debug bool
	var url string

	cmd := &cobra.Command{
		Use:   "sandbox [sandbox-name]",
		Short: "Connect to a sandbox environment",
		Long: `Connect to a sandbox environment using an interactive shell interface.

This command provides a terminal-like interface for:
- Executing commands in the sandbox
- Browsing files and directories
- Managing the sandbox environment

The shell connects to your sandbox via MCP (Model Control Protocol) over WebSocket.

Examples:
  bl connect sandbox my-sandbox
  bl connect sandbox production-env
  bl connect sandbox my-sandbox --url wss://custom.domain.com/sandbox/my-sandbox`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			sandboxName := args[0]

			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			r.ConnectSandbox(ctx, sandboxName, debug, url)
		},
	}

	cmd.Flags().BoolVar(&debug, "debug", false, "Enable debug mode")
	cmd.Flags().StringVar(&url, "url", "", "Custom WebSocket URL for MCP connection (defaults to wss://run.blaxel.ai/$WORKSPACE/sandboxes/$SANDBOX_NAME)")
	return cmd
}

func (r *Operations) ConnectSandbox(ctx context.Context, sandboxName string, debug bool, url string) {
	// Get the current workspace from SDK context
	currentContext := sdk.CurrentContext()
	workspace := currentContext.Workspace
	if workspace == "" {
		fmt.Println("Error: No workspace found in current context. Please run 'bl auth login' first.")
		os.Exit(1)
	}

	if url == "" {
		response, err := client.ListSandboxesWithResponse(ctx)
		if err != nil {
			fmt.Printf("Error listing sandboxes: %v", err)
			os.Exit(1)
		}
		if response.StatusCode() != 200 {
			fmt.Printf("Error listing sandboxes: %s", response.Status())
			os.Exit(1)
		}
		found := false
		sandboxes := response.JSON200
		names := []string{}
		for _, sandbox := range *sandboxes {
			if *sandbox.Metadata.Name == sandboxName {
				found = true
				break
			}
			names = append(names, *sandbox.Metadata.Name)
		}
		if !found {
			fmt.Printf("\033[31mSandbox %s not found.\033[0m\n", sandboxName)
			if len(names) > 0 {
				fmt.Printf("Here is a list of available sandboxes: %s\n", strings.Join(names, ", "))
				fmt.Printf("Or you can create a new Sandbox here: https://app.blaxel.ai/%s/global-agentic-network/sandboxes\n", workspace)
			} else {
				fmt.Printf("You can create a Sandbox here: https://app.blaxel.ai/%s/global-agentic-network/sandboxes\n", workspace)
			}
			os.Exit(1)
		}
	}
	// Prepare authentication headers based on available credentials
	authHeaders := make(map[string]string)
	// Load credentials for the workspace
	credentials := sdk.LoadCredentials(workspace)
	if !credentials.IsValid() {
		fmt.Println("Error: No valid credentials found. Please run 'bl auth login' first.")
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
		fmt.Printf("Error running sandbox connect: %v\n", err)
		os.Exit(1)
	}
}

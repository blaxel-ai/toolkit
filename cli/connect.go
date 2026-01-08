package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
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
	cmd := &cobra.Command{
		Use:     "sandbox [sandbox-name]",
		Aliases: []string{"sb", "sbx"},
		Short:   "Connect to a sandbox environment",
		Long: `Connect to a sandbox environment by opening a terminal in your browser.

This command opens a web-based terminal interface for your sandbox.

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

			// Get the current workspace
			currentContext, _ := blaxel.CurrentContext()
			workspace := currentContext.Workspace
			if workspace == "" {
				err := fmt.Errorf("no workspace found in current context. Please run 'bl login' first")
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			// Load credentials
			credentials, _ := blaxel.LoadCredentials(workspace)
			if !credentials.IsValid() {
				err := fmt.Errorf("no valid credentials found. Please run 'bl login' first")
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			// Get the access token
			token := credentials.AccessToken
			if token == "" {
				token = credentials.APIKey
			}
			if token == "" {
				err := fmt.Errorf("no access token or API key found. Please run 'bl login' first")
				core.PrintError("Connect", err)
				core.ExitWithError(err)
			}

			// Get the sandbox to retrieve its URL
			client := core.GetClient()
			sbx, err := client.Sandboxes.Get(ctx, sandboxName, blaxel.SandboxGetParams{})
			if err != nil {
				var apiErr *blaxel.Error
				if isBlaxelError(err, &apiErr) && apiErr.StatusCode == 404 {
					err := fmt.Errorf("sandbox '%s' not found", sandboxName)
					core.PrintError("Connect", err)

					// List available sandboxes
					sandboxes, listErr := client.Sandboxes.List(ctx)
					if listErr == nil && sandboxes != nil && len(*sandboxes) > 0 {
						names := make([]string, 0, len(*sandboxes))
						for _, sb := range *sandboxes {
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
			terminalURL := fmt.Sprintf("%s/terminal?token=%s", sandboxURL, token)

			// Open in browser
			core.Print(fmt.Sprintf("Opening terminal for sandbox '%s' in browser...\n", sandboxName))
			if err := openBrowser(terminalURL); err != nil {
				// If browser fails, print the URL for manual access
				core.Print(fmt.Sprintf("Could not open browser automatically. Please open this URL:\n%s\n", terminalURL))
			}
		},
	}

	return cmd
}

// openBrowser opens the specified URL in the default browser
func openBrowser(url string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		return fmt.Errorf("unsupported platform")
	}

	return cmd.Start()
}

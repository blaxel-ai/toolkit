package cli

import (
	"os"

	"github.com/blaxel-ai/toolkit/cli/auth"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func init() {
	// Auto-register this command
	core.RegisterCommand("login", func() *cobra.Command {
		return LoginCmd()
	})
}

func LoginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login [workspace]",
		Short: "Login to Blaxel",
		Long: `Authenticate with Blaxel to access your workspace.

A workspace is your organization's isolated environment in Blaxel that contains
all your resources (agents, jobs, sandboxes, models, etc.). You must login before
using most Blaxel CLI commands.

Authentication Methods:
1. Browser OAuth (default) - Interactive login via web browser
2. API Key - For automation and scripts (set BL_API_KEY environment variable)
3. Client Credentials - For CI/CD pipelines (set BL_CLIENT_CREDENTIALS)

The CLI automatically detects which authentication method to use:
- If BL_CLIENT_CREDENTIALS is set, uses client credentials
- If BL_API_KEY is set, uses API key authentication
- Otherwise, shows interactive menu to choose browser or API key login

Credentials are stored securely in your system's credential store and persist
across sessions. Use 'bl logout' to remove stored credentials.

Examples:
  # Interactive login (shows menu to choose method)
  bl login my-workspace

  # Login without workspace (will prompt for workspace)
  bl login

  # API key authentication (non-interactive)
  export BL_API_KEY=your-api-key
  bl login my-workspace

  # Client credentials for CI/CD
  export BL_CLIENT_CREDENTIALS=your-credentials
  bl login my-workspace

After logging in, all commands will use this workspace by default.
Override with --workspace flag: bl get agents --workspace other-workspace`,
		Args: cobra.MaximumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			workspace := "" // Default workspace
			if len(args) > 0 {
				workspace = args[0]
			}
			if workspace == "" {
				auth.LoginDevice(workspace)
				return
			}

			// Check for environment variables first
			if os.Getenv("BL_CLIENT_CREDENTIALS") != "" {
				auth.LoginClientCredentials(workspace, os.Getenv("BL_CLIENT_CREDENTIALS"))
				return
			}

			if os.Getenv("BL_API_KEY") != "" {
				auth.LoginApiKey(workspace)
				return
			}

			// Show interactive menu
			showLoginMenu(workspace)
		},
	}
	return cmd
}

func showLoginMenu(workspace string) {
	var selectedMethod string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose a login method").
				Description("Select how you want to authenticate with Blaxel").
				Options(
					huh.NewOption("Login with your browser", "browser"),
					huh.NewOption("Login with API key", "apikey"),
				).
				Value(&selectedMethod),
		),
	)

	form.WithTheme(core.GetHuhTheme())

	err := form.Run()
	if err != nil {
		core.PrintError("Login", err)
		os.Exit(1)
	}

	switch selectedMethod {
	case "browser":
		auth.LoginDevice(workspace)
	case "apikey":
		auth.LoginApiKey(workspace)
	}
}

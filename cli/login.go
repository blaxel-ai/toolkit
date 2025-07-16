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
		Args:  cobra.MaximumNArgs(1),
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

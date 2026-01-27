package cli

import (
	"fmt"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func init() {
	// Auto-register this command
	core.RegisterCommand("logout", func() *cobra.Command {
		return LogoutCmd()
	})
}

// ClearCredentials clears credentials for a workspace
func clearCredentials(workspaceName string) error {
	config, err := blaxel.LoadConfig()
	if err != nil {
		return err
	}

	// Find and remove the workspace
	for i, ws := range config.Workspaces {
		if ws.Name == workspaceName {
			config.Workspaces = append(config.Workspaces[:i], config.Workspaces[i+1:]...)
			break
		}
	}

	// Clear current workspace if it was the one being removed
	if config.Context.Workspace == workspaceName {
		config.Context.Workspace = ""
	}

	return blaxel.WriteConfig(config)
}

func LogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout [workspace]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Logout from Blaxel",
		Long: `Remove stored credentials for a workspace.

This command clears local authentication tokens and credentials from your
system's credential store. Your deployed resources (agents, jobs, sandboxes)
continue running and are not affected by logout.

If you have multiple workspaces authenticated, you can logout from:
- A specific workspace by providing its name
- Any workspace interactively by running 'bl logout' without arguments

After logging out, you'll need to run 'bl login WORKSPACE' again to
authenticate before using other commands for that workspace.

Note: Logout is a local operation only. It does not:
- Stop running agents or jobs
- Delete any deployed resources
- Revoke tokens on the server (they will expire naturally)
- Affect other authenticated workspaces

Examples:
  # Logout from current workspace (interactive selection)
  bl logout

  # Logout from specific workspace
  bl logout my-workspace

  # Login again after logout
  bl login my-workspace`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				cfg, _ := blaxel.LoadConfig()
				workspaces := make([]string, 0, len(cfg.Workspaces))
				for _, ws := range cfg.Workspaces {
					workspaces = append(workspaces, ws.Name)
				}
				if len(workspaces) == 0 {
					err := fmt.Errorf("no authenticated workspaces found")
					core.PrintError("Logout", err)
					core.ExitWithError(err)
				}

				var selectedWorkspace string
				if len(workspaces) > 1 {
					// Create options for huh form
					options := make([]huh.Option[string], 0, len(workspaces))
					for _, ws := range workspaces {
						options = append(options, huh.NewOption(ws, ws))
					}

					// Create huh form for workspace selection
					form := huh.NewForm(
						huh.NewGroup(
							huh.NewSelect[string]().
								Title("Choose a workspace to logout from").
								Description("Select the workspace you want to logout from").
								Options(options...).
								Value(&selectedWorkspace),
						),
					)

					form.WithTheme(core.GetHuhTheme())

					err := form.Run()
					if err != nil {
						err = fmt.Errorf("error selecting workspace: %w", err)
						core.PrintError("Logout", err)
						core.ExitWithError(err)
					}
				} else {
					selectedWorkspace = workspaces[0]
				}

				clearCredentials(selectedWorkspace)
				core.PrintSuccess(fmt.Sprintf("Logged out from workspace %s", selectedWorkspace))
			} else {
				clearCredentials(args[0])
				core.PrintSuccess(fmt.Sprintf("Logged out from workspace %s", args[0]))
			}
		},
	}
}

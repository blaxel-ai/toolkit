package cli

import (
	"fmt"
	"os"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func init() {
	// Auto-register this command
	core.RegisterCommand("logout", func() *cobra.Command {
		return LogoutCmd()
	})
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

After logging out, you'll need to run 'bl login <workspace>' again to
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
				workspaces := sdk.ListWorkspaces()
				if len(workspaces) == 0 {
					core.PrintError("Logout", fmt.Errorf("no authenticated workspaces found"))
					os.Exit(1)
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
						core.PrintError("Logout", fmt.Errorf("error selecting workspace: %w", err))
						os.Exit(1)
					}
				} else {
					selectedWorkspace = workspaces[0]
				}

				sdk.ClearCredentials(selectedWorkspace)
				core.PrintSuccess(fmt.Sprintf("Logged out from workspace %s", selectedWorkspace))
			} else {
				sdk.ClearCredentials(args[0])
				core.PrintSuccess(fmt.Sprintf("Logged out from workspace %s", args[0]))
			}
		},
	}
}

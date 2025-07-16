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

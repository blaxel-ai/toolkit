package cli

import (
	"fmt"
	"os"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
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
				fmt.Println("Error: Enter a workspace")
				os.Exit(1)
			} else {
				sdk.ClearCredentials(args[0])
			}
		},
	}
}

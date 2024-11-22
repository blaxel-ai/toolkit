package cli

import (
	"fmt"
	"os"

	"github.com/beamlit/toolkit/sdk"
	"github.com/spf13/cobra"
)

func (r *Operations) LogoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout [workspace]",
		Args:  cobra.MaximumNArgs(1),
		Short: "Logout from Beamlit",
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

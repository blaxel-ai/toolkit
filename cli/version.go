package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func (r *Operations) VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Get the version of the CLI",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("Blaxel CLI: %s\n", version)
			fmt.Printf("Git commit: %s\n", commit)
			fmt.Printf("Built at: %s\n", date)
		},
	}
}

package cli

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	// Auto-register this command
	core.RegisterCommand("version", func() *cobra.Command {
		return VersionCmd()
	})
}

func VersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			version := core.GetVersion()
			commit := core.GetCommit()
			date := core.GetDate()

			if version == "" {
				version = "dev"
			}
			if commit == "" {
				commit = "unknown"
			}
			if date == "" {
				date = "unknown"
			}

			fmt.Printf("Blaxel CLI version: %s\n", version)
			fmt.Printf("Commit: %s\n", commit)
			fmt.Printf("Date: %s\n", date)
		},
	}
}

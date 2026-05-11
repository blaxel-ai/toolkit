package cli

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("create-job", func() *cobra.Command {
		return CreateJobCmd()
	})
}

// CreateJobCmd returns a deprecated command that redirects users to `bl new job`.
func CreateJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "create-job",
		Short:  "Deprecated: use 'bl new job' instead",
		Long:   "This command has been deprecated. Please use 'bl new job' instead.",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			err := fmt.Errorf("this command has been deprecated. Please use 'bl new job' instead")
			core.PrintError("create-job", err)
			core.ExitWithError(err)
		},
	}
	return cmd
}

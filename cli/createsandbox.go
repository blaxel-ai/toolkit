package cli

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("create-sandbox", func() *cobra.Command {
		return CreateSandboxCmd()
	})
}

// CreateSandboxCmd returns a deprecated command that redirects users to `bl new sandbox`.
func CreateSandboxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create-sandbox",
		Aliases: []string{"cs"},
		Short:   "Deprecated: use 'bl new sandbox' instead",
		Long:    "This command has been deprecated. Please use 'bl new sandbox' instead.",
		Hidden:  true,
		Run: func(cmd *cobra.Command, args []string) {
			core.PrintError("create-sandbox", fmt.Errorf("this command has been deprecated. Please use 'bl new sandbox' instead"))
		},
	}
	return cmd
}

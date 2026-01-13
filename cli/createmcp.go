package cli

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("create-mcp", func() *cobra.Command {
		return CreateMCPCmd()
	})
}

// CreateMCPCmd returns a deprecated command that redirects users to `bl new mcp`.
func CreateMCPCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create-mcp",
		Short: "Deprecated: use 'bl new mcp' instead",
		Long:  "This command has been deprecated. Please use 'bl new mcp' instead.",
		Run: func(cmd *cobra.Command, args []string) {
			core.PrintError("create-mcp", fmt.Errorf("this command has been deprecated. Please use 'bl new mcp' instead"))
		},
	}
	return cmd
}

package cli

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("create-agent-app", func() *cobra.Command {
		return CreateAgentAppCmd()
	})
}

// CreateAgentAppCmd returns a deprecated command that redirects users to `bl new agent`.
func CreateAgentAppCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:    "create-agent-app",
		Short:  "Deprecated: use 'bl new agent' instead",
		Long:   "This command has been deprecated. Please use 'bl new agent' instead.",
		Hidden: true,
		Run: func(cmd *cobra.Command, args []string) {
			core.PrintError("create-agent-app", fmt.Errorf("this command has been deprecated. Please use 'bl new agent' instead"))
		},
	}
	return cmd
}

package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func (r *Operations) ChatCmd() *cobra.Command {
	var debug bool
	var local bool

	cmd := &cobra.Command{
		Use:     "chat [agent-name]",
		Args:    cobra.ExactArgs(1),
		Short:   "Chat with an agent",
		Example: `bl chat my-agent`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Chatting with", args[0])
			if len(args) == 0 {
				fmt.Println("Error: Agent name is required")
				os.Exit(1)
			}

			resourceType := "agent"
			resourceName := args[0]

			err := client.Chat(context.Background(), workspace, environment, resourceType, resourceName)
			if err != nil {
				fmt.Println("Error: Failed to chat", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().BoolVar(&debug, "debug", false, "Debug mode")
	cmd.Flags().BoolVar(&local, "local", false, "Run locally")
	return cmd
}

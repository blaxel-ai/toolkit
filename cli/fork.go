package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("fork", func() *cobra.Command {
		return ForkCmd()
	})
}

type forkRequest struct {
	Name    string       `json:"name"`
	Type    string       `json:"type,omitempty"`
	Traffic *int         `json:"traffic,omitempty"`
	Port    *int         `json:"port,omitempty"`
	Memory  *int         `json:"memory,omitempty"`
	Spec    *forkAppSpec `json:"spec,omitempty"`
}

type forkAppSpec struct {
	Enabled   bool              `json:"enabled"`
	Revisions []forkAppRevision `json:"revisions"`
}

type forkAppRevision struct {
	Traffic int `json:"traffic,omitempty"`
	Port    int `json:"port,omitempty"`
	Memory  int `json:"memory,omitempty"`
}

func ForkCmd() *cobra.Command {
	var forkType string
	var traffic int
	var port int
	var memory int

	cmd := &cobra.Command{
		Use:   "fork <source-sandbox> <target-name>",
		Short: "Fork a sandbox into a new sandbox or application",
		Long:  "Create a new sandbox or application by forking an existing sandbox.",
		Example: `  # Fork a sandbox into a new sandbox
  bl fork my-sandbox my-sandbox-fork

  # Fork a sandbox into an application
  bl fork my-sandbox my-app --type application

  # Fork with canary traffic percentage and port
  bl fork my-sandbox my-app --type application --traffic 20 --port 8080

  # Fork with custom memory
  bl fork my-sandbox my-sandbox-fork --memory 4096`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			sourceName := args[0]
			targetName := args[1]

			client := core.GetClient()
			ctx := context.Background()

			reqBody := forkRequest{
				Name: targetName,
				Type: forkType,
			}

			if forkType == "application" {
				revision := forkAppRevision{}
				if cmd.Flags().Changed("traffic") {
					revision.Traffic = traffic
				}
				if cmd.Flags().Changed("port") {
					revision.Port = port
				}
				if cmd.Flags().Changed("memory") {
					revision.Memory = memory
				}
				reqBody.Spec = &forkAppSpec{
					Enabled:   true,
					Revisions: []forkAppRevision{revision},
				}
			} else {
				if cmd.Flags().Changed("traffic") {
					reqBody.Traffic = &traffic
				}
				if cmd.Flags().Changed("port") {
					reqBody.Port = &port
				}
				if cmd.Flags().Changed("memory") {
					reqBody.Memory = &memory
				}
			}

			if strings.Contains(sourceName, "/") || strings.Contains(sourceName, "..") {
				core.PrintError("Fork", fmt.Errorf("invalid sandbox name: %s", sourceName))
				core.ExitWithError(fmt.Errorf("invalid sandbox name"))
			}
			path := fmt.Sprintf("sandboxes/%s/fork", sourceName)
			var result json.RawMessage
			err := client.Post(ctx, path, reqBody, &result)
			if err != nil {
				core.PrintError("Fork", fmt.Errorf("failed to fork sandbox %s: %w", sourceName, err))
				core.ExitWithError(err)
			}

			fmt.Printf("Successfully forked sandbox %q into %s %q\n", sourceName, forkType, targetName)
		},
	}

	cmd.Flags().StringVar(&forkType, "type", "sandbox", "Target resource type (sandbox or application)")
	cmd.Flags().IntVar(&traffic, "traffic", 0, "Canary traffic percentage for the new revision")
	cmd.Flags().IntVar(&port, "port", 0, "Port to expose")
	cmd.Flags().IntVar(&memory, "memory", 0, "Memory in MB (inherits from source if not specified)")

	return cmd
}

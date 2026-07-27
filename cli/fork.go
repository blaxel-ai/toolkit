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
	TargetName string `json:"target_name"`
	Type       string `json:"type,omitempty"`
	Traffic    *int   `json:"traffic,omitempty"`
	Port       *int   `json:"port,omitempty"`
	Memory     *int   `json:"memory,omitempty"`
}

func buildForkRequest(targetName, targetType string, traffic, port, memory *int) forkRequest {
	return forkRequest{
		TargetName: targetName,
		Type:       targetType,
		Traffic:    traffic,
		Port:       port,
		Memory:     memory,
	}
}

// parseForkArg parses a "type/name" argument. Accepted type prefixes:
//   - sbx, sandbox → "sandbox"
//   - app, application → "application"
//
// If no prefix is given, the raw string is returned with an empty type.
func parseForkArg(arg string) (resourceType, name string, err error) {
	parts := strings.SplitN(arg, "/", 2)
	if len(parts) == 1 {
		return "", parts[0], nil
	}
	name = parts[1]
	if name == "" {
		return "", "", fmt.Errorf("missing name after '/' in %q", arg)
	}
	if strings.Contains(name, "/") {
		return "", "", fmt.Errorf("resource name must not contain '/': %q", name)
	}
	switch strings.ToLower(parts[0]) {
	case "sbx", "sandbox":
		return "sandbox", name, nil
	case "app", "application":
		return "application", name, nil
	default:
		return "", "", fmt.Errorf("unknown resource type %q in %q (use sbx or app)", parts[0], arg)
	}
}

func ForkCmd() *cobra.Command {
	var traffic int
	var port int
	var memory int

	cmd := &cobra.Command{
		Use:   "fork <source> <target>",
		Short: "Fork a sandbox into a new sandbox or application",
		Long: `Create a new sandbox or application by forking an existing sandbox.

Arguments use the type/name format:
  sbx/name or sandbox/name  — sandbox resource
  app/name or application/name — application resource

If the source has no type prefix, it defaults to sandbox.`,
		Example: `  # Fork a sandbox into a new sandbox
  bl fork sbx/my-sandbox sbx/my-sandbox-fork

  # Fork a sandbox into an application
  bl fork sbx/my-sandbox app/my-app

  # Fork with canary traffic and port
  bl fork sbx/my-sandbox app/my-app --traffic 20 --port 8080

  # Fork with custom memory
  bl fork sbx/my-sandbox sbx/my-fork --memory 4096

  # Short form (source defaults to sandbox)
  bl fork my-sandbox app/my-app`,
		Args: cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			sourceType, sourceName, err := parseForkArg(args[0])
			if err != nil {
				core.PrintError("Fork", err)
				core.ExitWithError(err)
			}
			if sourceType == "" {
				sourceType = "sandbox"
			}
			if sourceType != "sandbox" {
				err := fmt.Errorf("source must be a sandbox (got %s)", sourceType)
				core.PrintError("Fork", err)
				core.ExitWithError(err)
			}

			targetType, targetName, err := parseForkArg(args[1])
			if err != nil {
				core.PrintError("Fork", err)
				core.ExitWithError(err)
			}
			if targetType == "" {
				targetType = "sandbox"
			}

			if strings.Contains(sourceName, "..") {
				core.PrintError("Fork", fmt.Errorf("invalid sandbox name: %s", sourceName))
				core.ExitWithError(fmt.Errorf("invalid sandbox name"))
			}

			client := core.GetClient()
			ctx := context.Background()

			var trafficParam, portParam, memoryParam *int
			if cmd.Flags().Changed("traffic") {
				trafficParam = &traffic
			}
			if cmd.Flags().Changed("port") {
				portParam = &port
			}
			if cmd.Flags().Changed("memory") {
				memoryParam = &memory
			}
			reqBody := buildForkRequest(targetName, targetType, trafficParam, portParam, memoryParam)

			path := fmt.Sprintf("sandboxes/%s/fork", sourceName)
			var result json.RawMessage
			err = client.Post(ctx, path, reqBody, &result)
			if err != nil {
				core.PrintError("Fork", fmt.Errorf("failed to fork sandbox %s: %w", sourceName, err))
				core.ExitWithError(err)
			}

			fmt.Printf("Successfully forked sandbox %q into %s %q\n", sourceName, targetType, targetName)
		},
	}

	cmd.Flags().IntVar(&traffic, "traffic", 0, "Canary traffic percentage for the new revision")
	cmd.Flags().IntVar(&port, "port", 0, "Port to expose")
	cmd.Flags().IntVar(&memory, "memory", 0, "Memory in MB (inherits from source if not specified)")

	return cmd
}

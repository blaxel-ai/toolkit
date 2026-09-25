package cli

import (
	"context"
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

// forkRequest is the body of POST /sandboxes/{name}/fork. The field names are
// the API's SandboxForkRequest, which is camelCase: a snake_case key does not
// reach it at all, since Go's JSON decoding matches names case-insensitively
// but not across an underscore, so the server would read an empty target and
// reject the call.
type forkRequest struct {
	TargetName string `json:"targetName"`
	TargetType string `json:"targetType,omitempty"`
	Traffic    *int   `json:"traffic,omitempty"`
	Port       *int   `json:"port,omitempty"`
	// SnapshotID forks from a snapshot taken earlier instead of from the
	// source's live state. Empty means fork the live state.
	SnapshotID string `json:"snapshotId,omitempty"`
}

// forkResponse is the API's SandboxForkResponse. SnapshotID is set only when
// the fork went through a snapshot — an explicit --snapshot-id, or a fork into
// an application — and is empty for a direct sandbox fork.
type forkResponse struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	SnapshotID string `json:"snapshotId"`
}

func buildForkRequest(targetName, targetType string, traffic, port *int, snapshotID string) forkRequest {
	return forkRequest{
		TargetName: targetName,
		TargetType: targetType,
		Traffic:    traffic,
		Port:       port,
		SnapshotID: snapshotID,
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
	var snapshotID string

	cmd := &cobra.Command{
		Use:   "fork <source> <target>",
		Short: "Fork a sandbox into a new sandbox or application",
		Long: `Create a new sandbox or application by forking an existing sandbox.

Forking into a sandbox copies the source's live state — its filesystem,
running processes and memory — straight into the new sandbox. No snapshot is
created, and none is left behind on the source. Forking a running source
pauses it for the moment its memory is captured.

Pass --snapshot-id to fork from a snapshot you took earlier instead of from
the source's live state. Forking into an application always goes through a
snapshot, which the application's revision references so you can re-activate
that revision later.

A fork inherits the source's image, memory and environment; only its
environment can be changed, and only through the API.

Arguments use the type/name format:
  sbx/name or sandbox/name  — sandbox resource
  app/name or application/name — application resource

If the source has no type prefix, it defaults to sandbox.`,
		Example: `  # Fork a sandbox into a new sandbox, copying its live state
  bl fork sbx/my-sandbox sbx/my-sandbox-fork

  # Fork from a snapshot taken earlier instead of the live state
  bl fork sbx/my-sandbox sbx/my-sandbox-fork --snapshot-id snap_abc123

  # Fork a sandbox into an application
  bl fork sbx/my-sandbox app/my-app

  # Fork with canary traffic and port
  bl fork sbx/my-sandbox app/my-app --traffic 20 --port 8080

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

			// A fork resumes the source's own machine state, so its memory has
			// to match the source's exactly — the runtime refuses a fork whose
			// resources differ. There is no memory field on the fork API to
			// carry this, and honouring the flag is not possible, so say so
			// rather than accepting it and quietly forking at another size.
			if cmd.Flags().Changed("memory") {
				err := fmt.Errorf("--memory cannot be set on a fork: a fork inherits the source sandbox's memory (%d MB requested)", memory)
				core.PrintError("Fork", err)
				core.ExitWithError(err)
			}

			var trafficParam, portParam *int
			if cmd.Flags().Changed("traffic") {
				trafficParam = &traffic
			}
			if cmd.Flags().Changed("port") {
				portParam = &port
			}
			reqBody := buildForkRequest(targetName, targetType, trafficParam, portParam, snapshotID)

			path := fmt.Sprintf("sandboxes/%s/fork", sourceName)
			var result forkResponse
			err = client.Post(ctx, path, reqBody, &result)
			if err != nil {
				core.PrintError("Fork", fmt.Errorf("failed to fork sandbox %s: %w", sourceName, err))
				core.ExitWithError(err)
			}

			createdName := result.Name
			if createdName == "" {
				createdName = targetName
			}
			fmt.Printf("Successfully forked sandbox %q into %s %q\n", sourceName, targetType, createdName)
			// Only worth reporting when one exists: a direct sandbox fork
			// leaves no snapshot behind and returns an empty id.
			if result.SnapshotID != "" {
				fmt.Printf("Forked from snapshot %q\n", result.SnapshotID)
			}
		},
	}

	cmd.Flags().IntVar(&traffic, "traffic", 0, "Canary traffic percentage for the new revision")
	cmd.Flags().IntVar(&port, "port", 0, "Port to expose")
	cmd.Flags().IntVar(&memory, "memory", 0, "Deprecated: a fork always inherits the source sandbox's memory")
	cmd.Flags().StringVar(&snapshotID, "snapshot-id", "", "Fork from this snapshot instead of the source's live state")

	return cmd
}

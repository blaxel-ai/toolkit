package cli

import (
	"context"
	"fmt"
	"os"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("workspace", func() *cobra.Command {
		return ListOrSetWorkspacesCmd()
	})
}

func ListOrSetWorkspacesCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "workspaces [workspace]",
		Aliases: []string{"ws", "workspace"},
		Short:   "List all workspaces with the current workspace highlighted, set optionally a new current workspace",
		Long: `List and manage Blaxel workspaces.

A workspace is an isolated environment within Blaxel that contains your
resources (agents, jobs, models, sandboxes, etc.). Workspaces provide:

- Isolation between projects or environments (dev/staging/prod)
- Separate billing and resource quotas
- Team collaboration boundaries
- Independent access control and permissions

The current workspace (marked with *) determines where commands operate.
All commands like 'bl deploy', 'bl get', 'bl run' use the current workspace
unless you override with the --workspace flag.

To switch workspaces, provide the workspace name as an argument.
To list all authenticated workspaces, run without arguments.`,
		Example: `  # List all authenticated workspaces
  bl workspaces

  # Switch to different workspace
  bl workspaces production

  # Use specific workspace for one command (doesn't switch current)
  bl get agents --workspace staging

  # Common multi-workspace workflow
  bl workspaces dev        # Switch to dev
  bl deploy                # Deploy to dev
  bl workspaces prod       # Switch to prod
  bl deploy                # Deploy to prod`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				if len(args) > 1 {
					sdk.SetCurrentWorkspace(args[0])
				} else {
					sdk.SetCurrentWorkspace(args[0])
				}
			}

			workspaces := sdk.ListWorkspaces()
			currentWorkspace := sdk.CurrentContext().Workspace

			// En-têtes avec largeurs fixes
			fmt.Printf("%-30s %-20s\n", "NAME", "CURRENT")

			// Afficher chaque workspace avec les mêmes largeurs fixes
			for _, workspace := range workspaces {
				current := " "
				if workspace == currentWorkspace {
					current = "*"
				}
				fmt.Printf("%-30s %-20s\n", workspace, current)
			}
		},
	}
}

func CheckWorkspaceAccess(workspaceName string, credentials sdk.Credentials) (sdk.Workspace, error) {
	c, err := sdk.NewClientWithCredentials(
		sdk.RunClientWithCredentials{
			ApiURL:      core.GetBaseURL(),
			RunURL:      core.GetRunURL(),
			Credentials: credentials,
			Workspace:   workspaceName,
		},
	)
	if err != nil {
		return sdk.Workspace{}, err
	}
	response, err := c.GetWorkspaceWithResponse(context.Background(), workspaceName)
	if err != nil {
		return sdk.Workspace{}, err
	}
	if response.StatusCode() >= 400 {
		fmt.Println(core.ErrorHandler(response.HTTPResponse.Request, "workspace", workspaceName, string(response.Body)))
		os.Exit(1)
	}
	return *response.JSON200, nil
}

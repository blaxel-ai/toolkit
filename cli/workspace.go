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

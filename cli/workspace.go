package cli

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/beamlit/toolkit/sdk"
	"github.com/spf13/cobra"
)

func (r *Operations) ListOrSetWorkspacesCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "workspaces [workspace]",
		Aliases: []string{"ws", "workspace"},
		Short:   "List all workspaces with the current workspace highlighted, set optionally a new current workspace",
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) > 0 {
				sdk.SetCurrentWorkspace(args[0])
			}

			workspaces := sdk.ListWorkspaces()
			currentWorkspace := sdk.CurrentWorkspace()

			// Afficher les en-têtes du tableau
			fmt.Printf("%-20s %s\n", "NAME", "CURRENT")

			// Afficher chaque workspace
			for _, workspace := range workspaces {
				current := " "
				if workspace == currentWorkspace {
					current = "*"
				}
				fmt.Printf("%-20s %s\n", workspace, current)
			}
		},
	}
}

func CheckWorkspaceAccess(workspaceName string, credentials sdk.Credentials) error {
	c, err := sdk.NewClientWithCredentials(
		sdk.RunClientWithCredentials{
			ApiURL:      BASE_URL,
			RunURL:      RUN_URL,
			Credentials: credentials,
			Workspace:   workspace,
		},
	)
	if err != nil {
		return err
	}
	response, err := c.GetWorkspace(context.Background(), workspaceName)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if response.StatusCode >= 400 {
		ErrorHandler(buf.String())
		os.Exit(1)
	}
	return nil
}

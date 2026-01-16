package cli

import (
	"context"
	"fmt"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("workspace", func() *cobra.Command {
		return ListOrSetWorkspacesCmd()
	})
}

func ListOrSetWorkspacesCmd() *cobra.Command {
	var current bool

	cmd := &cobra.Command{
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

  # Get only the current workspace name
  bl workspaces --current

  # Common multi-workspace workflow
  bl workspaces dev        # Switch to dev
  bl deploy                # Deploy to dev
  bl workspaces prod       # Switch to prod
  bl deploy                # Deploy to prod`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, _ := blaxel.CurrentContext()
			currentWorkspace := ctx.Workspace

			// If --current flag is set, only print the current workspace name
			if current {
				fmt.Println(currentWorkspace)
				return
			}

			// If workspace name is provided, set it as current and return
			if len(args) > 0 {
				workspaceName := args[0]
				blaxel.SetCurrentWorkspace(workspaceName)
				fmt.Printf("Current workspace set to %s.\n", workspaceName)
				return
			}

			// Otherwise, list all workspaces
			cfg, _ := blaxel.LoadConfig()
			workspaces := make([]string, 0, len(cfg.Workspaces))
			for _, ws := range cfg.Workspaces {
				workspaces = append(workspaces, ws.Name)
			}

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

	cmd.Flags().BoolVar(&current, "current", false, "Display only the current workspace name")

	return cmd
}

func CheckWorkspaceAccess(workspaceName string, credentials blaxel.Credentials) (blaxel.Workspace, error) {
	// Build client options based on credentials
	opts := []option.RequestOption{
		option.WithBaseURL(blaxel.GetBaseURL()),
	}

	if workspaceName != "" {
		opts = append(opts, option.WithWorkspace(workspaceName))
	}

	if credentials.APIKey != "" {
		opts = append(opts, option.WithAPIKey(credentials.APIKey))
	} else if credentials.AccessToken != "" {
		opts = append(opts, option.WithAccessToken(credentials.AccessToken))
	} else if credentials.ClientCredentials != "" {
		opts = append(opts, option.WithClientCredentials(credentials.ClientCredentials))
	}

	c := blaxel.NewClient(opts...)
	workspace, err := c.Workspaces.Get(context.Background(), workspaceName)
	if err != nil {
		return blaxel.Workspace{}, err
	}
	return *workspace, nil
}

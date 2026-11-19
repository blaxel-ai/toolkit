package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

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
		Use:               "workspaces [workspace]",
		Aliases:           []string{"ws", "workspace"},
		Short:             "List workspaces or switch the current workspace",
		ValidArgsFunction: GetWorkspaceValidArgsFunction(),
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

To switch workspaces, provide the workspace name as an argument. It must be
a workspace you are logged in to (run 'bl login' to add one), or one that
BL_API_KEY / BL_CLIENT_CREDENTIALS can access; an unknown name is rejected
and the current workspace stays as it was.
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
				cfg, err := blaxel.LoadConfig()
				if err == nil {
					err = validateWorkspaceSwitch(workspaceName, args[1:], cfg)
				}
				if err != nil && envCredentialsReach(workspaceName) {
					err = nil
				}
				if err != nil {
					err = core.MarkExpectedError(err, core.CLIErrorNotFound)
					core.PrintError("Workspace", err)
					core.ExitWithError(err)
				}
				if err := blaxel.SetCurrentWorkspace(workspaceName); err != nil {
					core.PrintError("Workspace", fmt.Errorf("failed to set workspace: %w", err))
					core.ExitWithError(err)
				}
				fmt.Printf("Current workspace set to %s.\n", workspaceName)
				return
			}

			// Otherwise, list all workspaces
			cfg, _ := blaxel.LoadConfig()
			workspaces := make([]string, 0, len(cfg.Workspaces))
			for _, ws := range cfg.Workspaces {
				workspaces = append(workspaces, ws.Name)
			}

			// Headers with fixed widths
			fmt.Printf("%-30s %-20s\n", "NAME", "CURRENT")

			// Display each workspace with the same fixed widths
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

// switchVerbs are words people (and agents) commonly put before the workspace
// name, as in "bl workspaces switch prod". The command takes the name directly.
var switchVerbs = map[string]bool{"switch": true, "use": true, "set": true, "select": true}

// validateWorkspaceSwitch refuses a workspace that is not in the local config:
// switching to it would leave every later command without credentials.
func validateWorkspaceSwitch(workspaceName string, extraArgs []string, cfg blaxel.Config) error {
	names := make([]string, 0, len(cfg.Workspaces))
	for _, ws := range cfg.Workspaces {
		if ws.Name == workspaceName {
			return nil
		}
		names = append(names, ws.Name)
	}

	var msg strings.Builder
	fmt.Fprintf(&msg, "workspace '%s' is not one you are logged in to, so the current workspace was not changed.", workspaceName)
	if switchVerbs[workspaceName] && len(extraArgs) > 0 {
		fmt.Fprintf(&msg, " To switch, pass the name directly: 'bl workspaces %s'.", extraArgs[0])
	} else if len(names) == 0 {
		fmt.Fprintf(&msg, " You are not logged in to any workspace yet: run 'bl login %s'.", workspaceName)
	} else {
		fmt.Fprintf(&msg, " Logged-in workspaces: %s. Run 'bl workspaces' to list them, or 'bl login %s' to add it.",
			strings.Join(names, ", "), workspaceName)
	}
	return errors.New(msg.String())
}

// checkWorkspaceAccess is a variable so tests can stub the API call.
var checkWorkspaceAccess = CheckWorkspaceAccess

// envCredentialsReach reports whether BL_API_KEY or BL_CLIENT_CREDENTIALS can
// read the workspace. Commands use those without a saved login, so a workspace
// they can reach is a valid switch target even when it is not in the config.
func envCredentialsReach(workspaceName string) bool {
	creds := blaxel.Credentials{APIKey: os.Getenv("BL_API_KEY"), ClientCredentials: os.Getenv("BL_CLIENT_CREDENTIALS")}
	if creds.APIKey == "" && creds.ClientCredentials == "" {
		return false
	}
	_, err := checkWorkspaceAccess(workspaceName, creds)
	return err == nil
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
	workspace, err := c.Workspaces.Get(context.Background(), workspaceName, blaxel.WorkspaceGetParams{})
	if err != nil {
		return blaxel.Workspace{}, err
	}
	return *workspace, nil
}

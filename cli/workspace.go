package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
	"golang.org/x/term"
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

	cmd.AddCommand(WorkspaceHipaaCmd())

	return cmd
}

// workspaceHipaaResponse mirrors the parts of the controlplane Workspace JSON
// response that this command cares about. The sdk-go Workspace struct is
// generated from an older spec and does not expose hipaaUnsafe directly.
type workspaceHipaaResponse struct {
	HipaaUnsafe bool   `json:"hipaaUnsafe"`
	Name        string `json:"name"`
}

// WorkspaceHipaaCmd is the parent of `bl workspaces hipaa ...`.
func WorkspaceHipaaCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hipaa",
		Short: "Manage non-HIPAA-compliant deploy consent for the workspace",
		Long: `Manage the workspace-level consent to deploy in non-HIPAA-compliant
regions/products (workspace.hipaaUnsafe).

By default, deploying agents, sandboxes, functions or jobs to a region
that is not HIPAA-compliant is blocked. There are two ways to unblock
such a deploy:
  - Standing consent: set workspace.hipaaUnsafe=true with 'accept'. All
    future non-HIPAA-compliant deploys from this workspace will be
    allowed.
  - Per-deploy consent: pass '--accept-not-hipaa' to 'bl deploy' /
    'bl apply' for a single non-HIPAA-compliant deploy.

Use 'bl workspaces hipaa accept' to grant standing consent,
'bl workspaces hipaa decline' to revoke it, and
'bl workspaces hipaa status' to inspect the current state.`,
	}

	cmd.AddCommand(WorkspaceHipaaAcceptCmd())
	cmd.AddCommand(WorkspaceHipaaDeclineCmd())
	cmd.AddCommand(WorkspaceHipaaStatusCmd())
	return cmd
}

// WorkspaceHipaaAcceptCmd flips workspace.hipaaUnsafe to true.
func WorkspaceHipaaAcceptCmd() *cobra.Command {
	var assumeYes bool

	cmd := &cobra.Command{
		Use:   "accept",
		Short: "Allow non-HIPAA-compliant deploys for the current workspace",
		Long: `Grant standing consent for the current workspace to deploy in
regions/products that are NOT HIPAA-compliant (workspace.hipaaUnsafe=true).

By accepting, a workspace admin acknowledges that:
  - Future deploys from this workspace MAY target regions and products
    that are not HIPAA-compliant, with no further acknowledgement required.
  - Protected health information must not be processed in those deploys.

The '--workspace' global flag, if set, targets a different workspace. Only
workspace admins can change this setting.`,
		Example: `  # Allow non-HIPAA-compliant deploys for the current workspace (prompts)
  bl workspaces hipaa accept

  # Accept without an interactive prompt (useful in CI)
  bl workspaces hipaa accept --yes

  # Accept for a workspace other than the current one
  bl workspaces hipaa accept --workspace prod -y`,
		Run: func(cmd *cobra.Command, args []string) {
			runWorkspaceHipaaUpdate(cmd.Context(), true, assumeYes)
		},
	}

	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "Skip the interactive confirmation prompt")
	return cmd
}

// WorkspaceHipaaDeclineCmd flips workspace.hipaaUnsafe back to false.
func WorkspaceHipaaDeclineCmd() *cobra.Command {
	var assumeYes bool

	cmd := &cobra.Command{
		Use:   "decline",
		Short: "Block non-HIPAA-compliant deploys for the current workspace",
		Long: `Revoke the workspace's standing consent to deploy in non-HIPAA-
compliant regions/products (workspace.hipaaUnsafe=false).

After declining, non-HIPAA-compliant deploys will be rejected unless the
individual deploy is acknowledged with '--accept-not-hipaa'. Existing
resources are not affected. Only workspace admins can change this setting.`,
		Run: func(cmd *cobra.Command, args []string) {
			runWorkspaceHipaaUpdate(cmd.Context(), false, assumeYes)
		},
	}

	cmd.Flags().BoolVarP(&assumeYes, "yes", "y", false, "Skip the interactive confirmation prompt")
	return cmd
}

// WorkspaceHipaaStatusCmd prints the current value of workspace.hipaaUnsafe.
func WorkspaceHipaaStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show whether non-HIPAA-compliant deploys are allowed in the workspace",
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			workspaceName := resolveWorkspaceName()
			ws, err := fetchWorkspaceHipaa(ctx, workspaceName)
			if err != nil {
				core.PrintError("Workspace", err)
				core.ExitWithError(err)
			}
			state := "blocked (workspace.hipaaUnsafe=false)"
			if ws.HipaaUnsafe {
				state = "allowed (workspace.hipaaUnsafe=true)"
			}
			fmt.Printf("Workspace %s: non-HIPAA-compliant deploys %s\n", workspaceName, state)
		},
	}
}

func runWorkspaceHipaaUpdate(ctx context.Context, hipaaUnsafe bool, assumeYes bool) {
	if ctx == nil {
		ctx = context.Background()
	}
	workspaceName := resolveWorkspaceName()

	// Run the cheap client-availability check before any interactive prompt
	// so an unauthenticated user is told to `bl login` immediately instead
	// of after a successful y/N dialog.
	client := core.GetClient()
	if client == nil {
		err := fmt.Errorf("no API client available. Please run 'bl login' first")
		core.PrintError("Workspace", err)
		core.ExitWithError(err)
	}

	if !assumeYes && !confirmHipaaChange(workspaceName, hipaaUnsafe) {
		core.Print("Aborted.\n")
		return
	}

	body := map[string]bool{"hipaaUnsafe": hipaaUnsafe}
	var res workspaceHipaaResponse
	path := fmt.Sprintf("workspaces/%s/hipaa", workspaceName)
	if err := client.Put(ctx, path, body, &res); err != nil {
		msg := extractErrorMessage(err)
		core.PrintError("Workspace", fmt.Errorf("failed to update HIPAA-unsafe consent: %s", msg))
		core.ExitWithError(err)
	}

	if hipaaUnsafe {
		fmt.Printf("Workspace %s: non-HIPAA-compliant deploys are now allowed.\n", workspaceName)
	} else {
		fmt.Printf("Workspace %s: non-HIPAA-compliant deploys are now blocked.\n", workspaceName)
	}
}

// fetchWorkspaceHipaa loads the workspace through the generic client so we can
// read fields (hipaaUnsafe) that the older sdk-go Workspace struct does not
// expose as typed members.
func fetchWorkspaceHipaa(ctx context.Context, workspaceName string) (workspaceHipaaResponse, error) {
	client := core.GetClient()
	if client == nil {
		return workspaceHipaaResponse{}, fmt.Errorf("no API client available. Please run 'bl login' first")
	}
	var res workspaceHipaaResponse
	path := fmt.Sprintf("workspaces/%s", workspaceName)
	if err := client.Get(ctx, path, nil, &res); err != nil {
		return workspaceHipaaResponse{}, fmt.Errorf("%s", extractErrorMessage(err))
	}
	if res.Name == "" {
		res.Name = workspaceName
	}
	return res, nil
}

// resolveWorkspaceName returns the workspace targeted by the command — the
// --workspace override if provided, otherwise the current workspace from
// the local config.
func resolveWorkspaceName() string {
	if ws := core.GetWorkspace(); ws != "" {
		return ws
	}
	ctx, _ := blaxel.CurrentContext()
	if ctx.Workspace == "" {
		err := fmt.Errorf("no workspace selected. Run 'bl login' or pass --workspace")
		core.PrintError("Workspace", err)
		core.ExitWithError(err)
	}
	return ctx.Workspace
}

// confirmHipaaChange shows the change and asks for y/N when stdin is a TTY.
// Non-TTY callers (CI, pipelines) must pass --yes explicitly.
func confirmHipaaChange(workspaceName string, hipaaUnsafe bool) bool {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintln(os.Stderr, "Refusing to change HIPAA-unsafe consent without --yes (stdin is not a terminal).")
		return false
	}
	action := "ALLOW non-HIPAA-compliant deploys"
	if !hipaaUnsafe {
		action = "BLOCK non-HIPAA-compliant deploys"
	}
	fmt.Printf("About to %s for workspace '%s'. Continue? [y/N]: ", action, workspaceName)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	answer := strings.TrimSpace(strings.ToLower(line))
	return answer == "y" || answer == "yes"
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

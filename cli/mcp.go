package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/mcpbridge"
	"github.com/spf13/cobra"
)

func init() { core.RegisterCommand("mcp", MCPCmd) }

func MCPCmd() *cobra.Command {
	return &cobra.Command{
		Use: "mcp", Short: "Serve the Blaxel MCP server to a coding agent, signed in with your bl login",
		Long: `Serve the hosted Blaxel MCP tools over stdio using your existing bl login.

Run bl login before starting your agent. bl setup configures local MCP targets;
no token is stored in agent configurations and no separate MCP OAuth is needed.
Without a usable login, the connection still initializes, with an empty tool
list and instructions to run bl login, then restart or reconnect the agent.

The default workspace is pinned when the bridge starts resolving credentials:
the current bl workspace, --workspace or BL_WORKSPACE. A tool's workspace
argument can intentionally select another authorized workspace. This pin is
not a restriction on the user's authority.

Tokens refresh in memory only. bl logout removes local credentials, affecting
future requests, but does not revoke refresh grants or work already in flight.
BL_API_KEY and BL_CLIENT_CREDENTIALS override stored credentials and are not
removed by bl logout. Environment inheritance varies between agent clients.

The HTTPS origin comes from the stored workspace environment (prod or dev),
not inherited BL_API_URL or BL_ENV. Redirects are refused for both MCP requests
and token exchanges.`,
		Example: `  claude mcp add --scope user blaxel -- bl mcp

  {"mcpServers": {"blaxel": {"command": "/absolute/path/to/bl", "args": ["mcp"]}}}`,
		Args: cobra.NoArgs, SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			workspace, _ := explicitWorkspaceFlag(cmd)
			if workspace == "" {
				workspace = strings.TrimSpace(os.Getenv("BL_WORKSPACE"))
			}
			if os.Getenv("BL_API_URL") != "" || os.Getenv("BL_ENV") != "" {
				_, _ = fmt.Fprintln(os.Stderr, "bl mcp: ignoring inherited BL_API_URL/BL_ENV; using the stored workspace environment")
			}
			if os.Getenv("BL_API_KEY") != "" {
				_, _ = fmt.Fprintln(os.Stderr, "bl mcp: using BL_API_KEY (bl logout does not remove it)")
			} else if os.Getenv("BL_CLIENT_CREDENTIALS") != "" {
				_, _ = fmt.Fprintln(os.Stderr, "bl mcp: using BL_CLIENT_CREDENTIALS (bl logout does not remove it)")
			}
			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			if updater, err := defaultSkillsUpdater(core.GetVersion()); err == nil {
				go updater.run(ctx, skillsUpdateInterval)
			}
			return mcpbridge.Serve(ctx, workspace, os.Stdin, os.Stdout)
		},
	}
}

package cli

import (
	"github.com/blaxel-ai/toolkit/cli/agentsetup"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("skills", SkillsCmd)
}

func SkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "skills", Short: "Manage Blaxel skills for coding agents",
		Long: `Manage the Blaxel agent skills installed for your coding agents.

Once skills are installed, ordinary bl commands and bl mcp check for a newer
verified skills bundle at most every six hours. Turn this off with
bl skills autoupdate off, or with BL_INSTALL_SKILLS=false; CI is skipped.

To uninstall, run bl skills autoupdate off so automatic updates and CLI
upgrades stop reinstalling them, then delete the blaxel-* folders in
~/.agents/skills and the matching links in each coding agent's skills folder
(for example ~/.claude/skills). The update state stays in ~/.blaxel/skills.`,
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error { return nil },
	}
	cmd.AddCommand(&cobra.Command{
		Use: "install", Short: "Install or refresh Blaxel skills for coding agents",
		Long:         "Install the latest Blaxel agent skills globally from github.com/blaxel-ai/agent-skills.\nSkills go to ~/.agents/skills and to the coding agents detected on this machine\n(Claude Code, Codex, Cursor, ...). Nothing else needs to be installed first.\nThis explicit command runs even when automatic installation is disabled with\nBL_INSTALL_SKILLS=false or in CI.\nExisting externally managed skill links are kept, not refreshed. Same-name\nskills in nested folders are reused automatically. Conflicting copies are\nbacked up outside the agents' skills folders and replaced with links.",
		Args:         cobra.NoArgs,
		RunE:         func(_ *cobra.Command, _ []string) error { return agentsetup.InstallSkillsOnce() },
		SilenceUsage: true, SilenceErrors: true,
	})
	cmd.AddCommand(&cobra.Command{Use: "status", Short: "Show skills revisions, update checks, skips and failures", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return agentsetup.WriteSkillsUpdateStatus(cmd.OutOrStdout())
		}})
	cmd.AddCommand(&cobra.Command{Use: "update", Short: "Refresh skills from the compatible verified release bundle",
		Long: "Check the trusted Blaxel skills manifest and install its compatible checksum-verified bundle.\nModified folders, manager links and plugin copies are preserved. This command\nignores automatic update opt-outs. Restart your coding agent to load changes.\nUntil the first bundle is published, use bl skills install for initial installation.",
		Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
			return agentsetup.UpdateSkills(cmd.Context(), cmd.OutOrStdout())
		}})
	cmd.AddCommand(&cobra.Command{Use: "autoupdate [on|off]", Short: "Save this machine's automatic skills update preference",
		Args: cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs), ValidArgs: []string{"on", "off"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return agentsetup.SetSkillsAutoUpdate(cmd.Context(), args[0] == "on")
		}})
	return cmd
}

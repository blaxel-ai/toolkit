package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"slices"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/agentsetup"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("uninstall", UninstallCmd)
}

type uninstallOptions struct {
	yes, dryRun, skipSkills, skipMCP bool
	// interactive is true in a terminal, where uninstall can ask first.
	interactive bool
	home        string
	env         func(string) string
	mcp         agentsetup.MCPEnv
	in          io.Reader
	out         io.Writer
	cli         agentsetup.CLIInstall
}

// errUninstallIncomplete exits with status 1 after uninstall has said why:
// nothing was confirmed, or something could not be removed.
var errUninstallIncomplete = errors.New("uninstall incomplete")

func UninstallCmd() *cobra.Command {
	options := uninstallOptions{}
	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the skills and MCP servers bl setup added to coding agents",
		Long: `Undo bl setup: remove the Blaxel agent skills and MCP servers that bl setup,
bl skills and bl mcp added to the coding agents on this machine, and turn off
their automatic updates, so CLI upgrades and the install scripts do not add
them back.

Only what Blaxel wrote and nobody changed since is removed. MCP server entries
you configured or edited yourself, skills you edited, and skills managed by
another tool (a link in ~/.agents/skills) are kept, and listed. A configuration
file that setup created and that is empty afterwards is deleted.

In a terminal, uninstall lists what it removes and asks first; --yes removes it
without asking. Without a terminal and without --yes, it only lists it.
--dry-run lists it and changes nothing.

Uninstall never deletes the running bl. It ends with the commands that remove
the CLI for the way it was installed (Homebrew, the install script, or
install.ps1 on Windows). If you keep bl, bl setup adds the skills and MCP
servers back and turns their automatic updates on again.`,
		Example: `  # See what would be removed, without changing anything
  bl uninstall --dry-run

  # Remove the skills and MCP servers without asking
  bl uninstall --yes

  # Remove only the MCP servers
  bl uninstall --skip-skills`,
		Args:         cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		// Uninstall needs no login, workspace or project settings.
		PersistentPreRunE: func(*cobra.Command, []string) error { return nil },
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			executable, _ := os.Executable()
			options.home, options.env = home, os.Getenv
			options.mcp = agentsetup.NewMCPEnv(home)
			options.interactive = core.IsTerminalInteractive()
			options.in, options.out = os.Stdin, cmd.OutOrStdout()
			options.cli = agentsetup.CLIInstall{Executable: executable, Home: home, GOOS: runtime.GOOS, Env: os.Getenv, Sudo: needsSudoForPath}
			err = runUninstall(cmd.Context(), options)
			if errors.Is(err, errUninstallIncomplete) {
				core.Exit(1)
			}
			return err
		},
	}
	cmd.Flags().BoolVarP(&options.yes, "yes", "y", false, "Remove without asking")
	cmd.Flags().BoolVar(&options.dryRun, "dry-run", false, "List what would be removed and change nothing")
	cmd.Flags().BoolVar(&options.skipSkills, "skip-skills", false, "Keep the Blaxel agent skills and automatic skills updates")
	cmd.Flags().BoolVar(&options.skipMCP, "skip-mcp", false, "Keep the Blaxel MCP servers")
	return cmd
}

func runUninstall(ctx context.Context, options uninstallOptions) error {
	plan, err := agentsetup.PlanUninstall(ctx, agentsetup.UninstallOptions{
		Home: options.home, Env: options.env, MCP: options.mcp, SkipSkills: options.skipSkills, SkipMCP: options.skipMCP,
	})
	if err != nil {
		return err
	}
	out := options.out
	if len(plan.Remove) == 0 {
		_, _ = fmt.Fprintln(out, "Nothing to remove: no Blaxel skills or MCP servers from bl setup were found.")
		printUninstallKept(out, options.home, plan.Keep)
		printCLIRemoval(out, options)
		return nil
	}
	mcp := slices.ContainsFunc(plan.Remove, func(item agentsetup.UninstallItem) bool {
		return item.Group == agentsetup.UninstallGroupMCP || item.Group == agentsetup.UninstallGroupMCPUpdates
	})
	_, _ = fmt.Fprintln(out, "bl uninstall removes:")
	printUninstallItems(out, options.home, plan.Remove)
	printUninstallKept(out, options.home, plan.Keep)
	switch {
	case options.dryRun:
		_, _ = fmt.Fprintln(out, "\nDry run: nothing was removed. Run bl uninstall to remove these.")
		printCLIRemoval(out, options)
		return nil
	case options.yes:
	case !options.interactive:
		_, _ = fmt.Fprintln(out, "\nNothing was removed. To remove these, run: bl uninstall --yes")
		return errUninstallIncomplete
	default:
		_, _ = fmt.Fprint(out, "\nRemove these? [y/N] ")
		answer, _ := bufio.NewReader(options.in).ReadString('\n')
		if answer = strings.ToLower(strings.TrimSpace(answer)); answer != "y" && answer != "yes" {
			_, _ = fmt.Fprintln(out, "Nothing was removed.")
			return nil
		}
	}
	items, err := plan.Run(ctx)
	if err != nil {
		return err
	}
	var removed, failed []agentsetup.UninstallItem
	for _, item := range items {
		if item.Err != nil {
			failed = append(failed, item)
		} else {
			removed = append(removed, item)
		}
	}
	if len(removed) > 0 {
		_, _ = fmt.Fprintln(out, "\nRemoved:")
		printUninstallItems(out, options.home, removed)
	}
	if len(failed) > 0 {
		_, _ = fmt.Fprintln(out, "\nNot removed:")
		for _, item := range failed {
			_, _ = fmt.Fprintf(out, "  %s  %s\n", displayHomePath(options.home, item.Path), item.Err)
		}
	}
	if len(removed) > 0 {
		_, _ = fmt.Fprintln(out, "\nRestart your coding agents so they stop loading them.")
	}
	if plan.AutoUpdateOff {
		_, _ = fmt.Fprintln(out, "Automatic skills updates are off; bl setup turns them back on.")
	}
	if mcp {
		_, _ = fmt.Fprintln(out, "Upgrades no longer add the MCP servers; run bl setup to add them back.")
	}
	printCLIRemoval(out, options)
	if len(failed) > 0 {
		return errUninstallIncomplete
	}
	return nil
}

// printUninstallItems lists items under their group, one line per path.
func printUninstallItems(out io.Writer, home string, items []agentsetup.UninstallItem) {
	group := ""
	for _, item := range items {
		if item.Group != group {
			group = item.Group
			_, _ = fmt.Fprintf(out, "  %s\n", group)
		}
		line := "    " + displayHomePath(home, item.Path)
		if item.Detail != "" {
			line += "  " + item.Detail
		}
		_, _ = fmt.Fprintln(out, line)
	}
}

func printUninstallKept(out io.Writer, home string, kept []agentsetup.UninstallItem) {
	if len(kept) == 0 {
		return
	}
	_, _ = fmt.Fprintln(out, "\nKept:")
	for _, item := range kept {
		_, _ = fmt.Fprintf(out, "  %s  %s\n", displayHomePath(home, item.Path), item.Detail)
	}
}

// printCLIRemoval tells how to remove bl itself, which is still running.
func printCLIRemoval(out io.Writer, options uninstallOptions) {
	steps := agentsetup.CLIRemovalSteps(options.cli)
	_, _ = fmt.Fprintln(out, "\nTo remove the bl CLI as well:")
	for _, step := range steps {
		switch {
		case step.Command == "":
			_, _ = fmt.Fprintf(out, "  %s\n", step.Note)
		case step.Note == "":
			_, _ = fmt.Fprintf(out, "  %s\n", step.Command)
		default:
			_, _ = fmt.Fprintf(out, "  %s  # %s\n", step.Command, step.Note)
		}
	}
}

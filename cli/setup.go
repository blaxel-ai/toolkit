package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/auth"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/ui"
	"github.com/spf13/cobra"
)

const (
	// mcpInstallEnv=false stops setup from adding the Blaxel MCP servers by default.
	mcpInstallEnv = "BL_INSTALL_MCP"
	// loginInstallEnv=false stops setup from logging in by default.
	loginInstallEnv = "BL_INSTALL_LOGIN"
	// trackingInstallEnv=false turns the anonymous error reports off.
	trackingInstallEnv = "BL_INSTALL_TRACKING"
	// The installers describe what they already did, for the setup screens.
	installerShellEnv  = "BL_INSTALLER_SHELL"
	installerReloadEnv = "BL_INSTALLER_RELOAD"
	setupDocsURL       = "https://docs.blaxel.ai/skills-mcp"
)

func init() {
	core.RegisterCommand("setup", SetupCmd)
}

type setupOptions struct {
	yes                            bool
	agents                         []string
	skipSkills, skipMCP, skipLogin bool
	// interactive is true in a terminal, where setup can log in with the browser.
	interactive                     bool
	workspace                       string
	home                            string
	env                             func(string) string
	out                             *os.File
	subtitle                        string
	mcp                             mcpEnv
	resourceServer, documentsServer mcpServer
	installSkills                   func(context.Context, []skillsAgent) (skillsInstallResult, error)
	login                           func(ctx context.Context, c *ui.Control, workspace string) (string, error)
	loginState                      func(workspace string) string
	trackingConfigured              func() bool
	trackingEnabled                 func() bool
	setTracking                     func(bool)
}

func SetupCmd() *cobra.Command {
	options := setupOptions{}
	refreshCheck := false
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up Blaxel for your coding agents and log in",
		Long: `Set up everything Blaxel needs on this machine, then log in.

When logging in without a specified workspace, if your account has no
workspaces, setup opens the Console so you can create or join one, and waits
up to five minutes for it to become available.

For the coding agents found on this machine (Claude Code, Codex, Cursor, ...),
setup installs the Blaxel agent skills and adds two MCP servers: blaxel, to
manage your workspace resources, and blaxel-docs, to search the Blaxel
documentation. It then logs you in to Blaxel in your browser. The agents use
that login through bl mcp, so they need no sign-in of their own.

In a terminal, setup shows everything it found, selected, and installs it
when you press Enter; --yes installs it without showing the plan. Without a
terminal, setup installs the same defaults and skips the browser login, so
run bl login afterwards. Anonymous error reports are on unless you turn them
off (or set DO_NOT_TRACK=1).

Setup only adds what is missing, and it is safe to run again after
installing another agent. MCP server entries you configured yourself are left
unchanged; the hosted blaxel server added by earlier versions is switched to
bl mcp. A blaxel server that the Blaxel plugin provides is left to the plugin.`,
		Example: `  # See what setup found, then press Enter to install
  bl setup

  # Install the defaults without showing the plan
  bl setup --yes

  # Set up only Claude Code and Codex, without logging in
  bl setup --agent claude-code,codex --skip-login`,
		Args:         cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if refreshCheck || strings.EqualFold(strings.TrimSpace(os.Getenv(setupRefreshEnv)), "true") {
				return nil
			}
			if root := cmd.Root(); root != cmd && root.PersistentPreRunE != nil {
				return root.PersistentPreRunE(cmd, args)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if refreshCheck {
				_, err := fmt.Fprintln(cmd.OutOrStdout(), setupRefreshCapability)
				return err
			}
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			options.home = home
			options.env = os.Getenv
			if strings.EqualFold(strings.TrimSpace(os.Getenv(setupRefreshEnv)), "true") {
				refreshSetup(options)
				if executable, err := os.Executable(); err == nil {
					setupHomebrewRefresh(executable, func() {})
				}
				return nil
			}
			options.out = os.Stdout
			options.interactive = core.IsTerminalInteractive()
			options.workspace, _ = explicitWorkspaceFlag(cmd)
			if options.workspace == "" {
				options.workspace = strings.TrimSpace(os.Getenv("BL_WORKSPACE"))
			}
			options.subtitle = setupSubtitle()
			options.installSkills = installSkillsFor
			options.login = setupDeviceLogin
			options.loginState = setupLoginState
			options.trackingConfigured = blaxel.IsTrackingConfigured
			options.trackingEnabled = blaxel.IsTrackingEnabled
			options.setTracking = blaxel.SetTracking
			options.mcp = newMCPEnv(home)
			bl, pathErr := blCommandPath(os.Executable)
			if pathErr != nil {
				return pathErr
			}
			options.resourceServer, options.documentsServer = resourceMCPServer(bl), docsMCPServer()
			err = runSetup(cmd.Context(), options)
			var problems setupProblems
			if errors.As(err, &problems) {
				core.Exit(1)
			}
			return err
		},
	}
	cmd.Flags().BoolVarP(&options.yes, "yes", "y", false, "Install the defaults without showing the plan")
	cmd.Flags().StringSliceVar(&options.agents, "agent", nil, "Coding agents to set up instead of the detected ones (for example claude-code,codex)")
	cmd.Flags().BoolVar(&options.skipSkills, "skip-skills", false, "Do not install the Blaxel agent skills")
	cmd.Flags().BoolVar(&options.skipMCP, "skip-mcp", false, "Do not add the Blaxel MCP servers")
	cmd.Flags().BoolVar(&options.skipLogin, "skip-login", false, "Do not log in to Blaxel")
	cmd.Flags().BoolVar(&refreshCheck, "refresh-check", false, "Check the internal refresh contract")
	_ = cmd.Flags().MarkHidden("refresh-check")
	_ = cmd.RegisterFlagCompletionFunc("agent", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		ids := make([]string, 0, len(skillsAgents))
		for _, agent := range setupAgents() {
			ids = append(ids, agent.id+"\t"+agent.name)
		}
		return ids, cobra.ShellCompDirectiveNoFileComp
	})
	return cmd
}

func setupSubtitle() string {
	system := map[string]string{"darwin": "macOS", "linux": "Linux", "windows": "Windows"}[runtime.GOOS]
	if system == "" {
		system = runtime.GOOS
	}
	arch := map[string]string{"amd64": "x64", "386": "x86"}[runtime.GOARCH]
	if arch == "" {
		arch = runtime.GOARCH
	}
	return fmt.Sprintf("v%s · %s %s", strings.TrimPrefix(core.GetVersion(), "v"), system, arch)
}

// setupLoginState describes existing authentication, or returns "" when the
// user still needs to log in.
func setupLoginState(workspace string) string {
	if workspace != "" && (os.Getenv("BL_API_KEY") != "" || os.Getenv("BL_CLIENT_CREDENTIALS") != "") {
		return workspace + " (from the environment)"
	}
	if workspace == "" {
		config, _ := blaxel.LoadConfig()
		workspace = config.Context.Workspace
	}
	if workspace == "" {
		return ""
	}
	if credentials, err := blaxel.LoadCredentials(workspace); err == nil && credentials.IsValid() {
		return workspace
	}
	return ""
}

// setupDeviceLogin logs in with the browser from the setup screens.
func setupDeviceLogin(ctx context.Context, c *ui.Control, workspace string) (string, error) {
	c.Progress("opening your browser")
	login, opened, err := auth.StartDeviceLogin(ctx)
	if err != nil {
		return "", err
	}
	if opened {
		c.Note("Confirm the login in your browser. Not open? " + login.VerificationURIComplete)
	} else {
		c.Note("Open this page to log in: " + login.VerificationURIComplete)
	}
	c.Progress("waiting for you in the browser")
	creds, err := auth.WaitForDeviceLogin(ctx, login.DeviceCode)
	if err != nil {
		return "", err
	}
	if workspace == "" {
		c.Progress("checking your workspaces")
		names, err := auth.WaitForLoginWorkspaces(ctx, creds, func(note string) {
			c.Progress("waiting for you to create or join a workspace")
			c.Note(note)
		})
		if err != nil {
			return "", err
		}
		workspace = names[0]
		if len(names) > 1 {
			index, err := c.Choose("Choose a workspace", names)
			if err != nil {
				return "", err
			}
			workspace = names[index]
		}
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	c.Progress("saving your login")
	return workspace, auth.SaveDeviceLogin(workspace, creds)
}

func envDisabled(env func(string) string, key string) bool {
	return strings.EqualFold(strings.TrimSpace(env(key)), "false")
}

// setupPlan holds what setup found on this machine.
type setupPlan struct {
	agents   []skillsAgent
	status   map[string]agentStatus
	skills   []string // Blaxel skills installed before
	loggedIn string
}

// agentStatus is what an agent already has from an earlier setup.
type agentStatus struct {
	skills   bool
	noSkills bool     // the agent takes MCP servers only (Claude Desktop)
	mcp      bool     // the agent takes MCP servers
	has      []string // Blaxel MCP servers it already has
	missing  []string // Blaxel MCP servers it lacks or has in an outdated form
	outdated []string // of missing, those an earlier setup wrote
	plugin   bool     // the Blaxel plugin supplies the blaxel server
}

// label names a server the agent has, and says when the plugin supplies it.
func (s agentStatus) label(name string) string {
	if name == "blaxel" && s.plugin {
		return mcpLabel(name) + pluginLabelSuffix
	}
	return mcpLabel(name)
}

// complete reports whether the agent has everything setup offers.
func (s agentStatus) complete(skills, mcp bool) bool {
	return (!skills || s.skills || s.noSkills) && (!mcp || !s.mcp || len(s.missing) == 0)
}

// fresh reports whether an earlier setup left nothing in the agent.
func (s agentStatus) fresh() bool {
	return (s.noSkills || !s.skills) && len(s.has) == 0 && len(s.outdated) == 0
}

func newSetupPlan(options setupOptions) (setupPlan, error) {
	plan := setupPlan{}
	if len(options.agents) > 0 {
		for _, id := range options.agents {
			id = strings.TrimSpace(id)
			agent, ok := findSkillsAgent(id)
			if !ok {
				return plan, fmt.Errorf("unknown agent %q; choose from: %s", id, strings.Join(skillsAgentIDs(), ", "))
			}
			if !slices.ContainsFunc(plan.agents, func(a skillsAgent) bool { return a.id == id }) {
				plan.agents = append(plan.agents, agent)
			}
		}
	} else {
		plan.agents = detectedSetupAgents(options.home, options.env)
	}
	plan.loggedIn = options.loginState(options.workspace)
	plan.skills = installedBlaxelSkills(options.home, options.env)
	plan.status = map[string]agentStatus{}
	for _, agent := range plan.agents {
		status := agentStatus{noSkills: isMCPOnlyAgent(agent.id)}
		if !status.noSkills {
			status.skills = skillsInstalledFor(agent, options.home, options.env, plan.skills)
		}
		if target, ok := mcpTargets[agent.id]; ok {
			status.mcp = true
			plugin := target.pluginServes(options.mcp)
			status.plugin = plugin
			for _, server := range []mcpServer{options.resourceServer, options.documentsServer} {
				if !target.takes(server) {
					continue
				}
				state := classifyMCPEntry(options.mcp, server, target.entry(options.mcp, server.name))
				switch {
				case server.plugin && plugin, state == mcpEntryCurrent, state == mcpEntryCustom:
					status.has = append(status.has, server.name)
				case state == mcpEntryOutdated:
					status.missing = append(status.missing, server.name)
					status.outdated = append(status.outdated, server.name)
				default:
					status.missing = append(status.missing, server.name)
				}
			}
		}
		plan.status[agent.id] = status
	}
	return plan, nil
}

func skillsAgentIDs() []string {
	ids := make([]string, 0, len(skillsAgents))
	for _, agent := range setupAgents() {
		ids = append(ids, agent.id)
	}
	return ids
}

func ciEnvironment(env func(string) string) bool {
	for _, name := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL", "BUILDKITE"} {
		if env(name) != "" {
			return true
		}
	}
	return false
}

// setupOffers reports whether setup offers the skills and the MCP servers.
func setupOffers(options setupOptions) (skills, mcp bool) {
	return !options.skipSkills && !envDisabled(options.env, skillsInstallEnv), !options.skipMCP && !envDisabled(options.env, mcpInstallEnv)
}

// setupItems is the plan the user sees. Everything new is selected, and what
// an earlier setup already did is shown as done.
func setupItems(options setupOptions, plan setupPlan) []*ui.Item {
	offerSkills, offerMCP := setupOffers(options)
	var agentItems []*ui.Item
	toSetUp, firstRun := 0, len(plan.skills) == 0
	needs := map[string]int{}
	present := map[string]int{}
	for _, agent := range plan.agents {
		status := plan.status[agent.id]
		firstRun = firstRun && status.fresh()
		capabilities := "skills"
		switch {
		case status.noSkills:
			capabilities = "MCP"
		case status.mcp && offerMCP:
			capabilities = "skills · MCP"
		}
		if status.complete(offerSkills, offerMCP) {
			agentItems = append(agentItems, &ui.Item{ID: "agent:" + agent.id, Label: agent.name, Done: "set up · " + capabilities})
			for _, name := range status.has {
				present[name]++
			}
			continue
		}
		toSetUp++
		detail := capabilities
		if !status.fresh() {
			var add, update []string
			if offerSkills && !status.skills && !status.noSkills {
				add = append(add, "skills")
			}
			if offerMCP {
				var adds []string
				for _, name := range status.missing {
					if slices.Contains(status.outdated, name) {
						update = append(update, mcpLabel(name))
					} else {
						adds = append(adds, mcpLabel(name))
					}
				}
				if len(adds) == 2 {
					adds = []string{"MCP"}
				}
				add = append(add, adds...)
			}
			var parts []string
			if len(add) > 0 {
				parts = append(parts, "add "+strings.Join(add, ", "))
			}
			if len(update) > 0 {
				// The hosted server from before bl mcp: no more sign-ins.
				parts = append(parts, "switch "+strings.Join(update, ", ")+" to your bl login")
			}
			detail = strings.Join(parts, " · ")
		}
		for _, name := range status.missing {
			needs[name]++
		}
		for _, name := range status.has {
			present[name]++
		}
		agentItems = append(agentItems, &ui.Item{ID: "agent:" + agent.id, Label: agent.name, Detail: detail, On: true})
	}
	group := fmt.Sprintf("Coding agents · %d found", len(plan.agents))
	switch {
	case !firstRun && toSetUp > 0:
		group = fmt.Sprintf("Coding agents · %d to set up", toSetUp)
	case !firstRun && len(plan.agents) > 0:
		group = "Coding agents · all set up"
	}
	for _, item := range agentItems {
		item.Group = group
	}
	items := agentItems
	if offerSkills {
		item := &ui.Item{ID: "skills", Group: "Blaxel", Label: "Agent skills", Detail: "teach your agents bl and the SDKs", On: true}
		switch {
		case len(plan.skills) > 0:
			item.Detail, item.Update = "update to the latest", true
		case len(plan.agents) == 0:
			item.Detail = "to ~/.agents/skills, read by most agents"
		}
		items = append(items, item)
	}
	if offerMCP {
		for _, server := range []struct{ id, label, detail, name string }{
			{"mcp", "Blaxel MCP", "your workspace, from your agents", options.resourceServer.name},
			{"docs", "Docs MCP", "search docs.blaxel.ai", options.documentsServer.name},
		} {
			switch {
			case needs[server.name] > 0:
				items = append(items, &ui.Item{ID: server.id, Group: "Blaxel", Label: server.label, Detail: server.detail, On: true})
			case present[server.name] > 0:
				items = append(items, &ui.Item{ID: server.id, Group: "Blaxel", Label: server.label,
					Done: fmt.Sprintf("in %d %s", present[server.name], plural(present[server.name], "agent", "agents"))})
			}
		}
	}
	if shell := strings.TrimSpace(options.env(installerShellEnv)); shell != "" {
		items = append(items, &ui.Item{ID: "shell", Group: "This machine", Label: "Shell", Done: shell})
	}
	switch {
	case plan.loggedIn != "":
		items = append(items, &ui.Item{ID: "account", Group: "This machine", Label: "Logged in", Done: plan.loggedIn})
	case options.interactive && !options.skipLogin && !envDisabled(options.env, loginInstallEnv):
		items = append(items, &ui.Item{ID: "login", Group: "This machine", Label: "Log in", Detail: "opens your browser", On: true})
	}
	explicit := strings.EqualFold(strings.TrimSpace(options.env(trackingInstallEnv)), "true")
	// DO_NOT_TRACK, set either way, is already a choice, as in sdk-go.
	doNotTrack := strings.TrimSpace(options.env("DO_NOT_TRACK")) != ""
	if !doNotTrack && (explicit || !ciEnvironment(options.env)) {
		enabled := !envDisabled(options.env, trackingInstallEnv)
		if strings.TrimSpace(options.env(trackingInstallEnv)) == "" && options.trackingConfigured() {
			enabled = options.trackingEnabled()
		}
		items = append(items, &ui.Item{ID: "tracking", Group: "This machine", Label: "Error reports", Detail: "anonymous, helps us fix bugs faster", On: enabled})
	}
	return items
}

// setupOutcome collects what the tasks did, for the final screen.
type setupOutcome struct {
	mu        sync.Mutex
	skills    []string
	preserved []string
	repaired  []string
	backups   []string
	mcp       map[string]mcpAgentResult
	workspace string
	tracking  *bool
}

func runSetup(ctx context.Context, options setupOptions) error {
	if ctx == nil {
		ctx = context.Background()
	}
	plan, err := newSetupPlan(options)
	if err != nil {
		return err
	}
	outcome := &setupOutcome{mcp: map[string]mcpAgentResult{}}
	setup := &ui.Setup{
		Subtitle: options.subtitle,
		Items:    setupItems(options, plan),
		Tasks:    func(chosen map[string]bool) []ui.Task { return setupTasks(options, plan, chosen, outcome) },
	}
	var chosen map[string]bool
	setup.Summary = func(results map[string]ui.Result) ui.Summary {
		chosen = map[string]bool{}
		for _, item := range setup.Items {
			chosen[item.ID] = item.On || item.Done != ""
		}
		return setupSummary(options, plan, chosen, results, outcome)
	}
	summary, err := setup.Run(ctx, ui.Options{Out: options.out, Interactive: options.interactive && !options.yes, Yes: options.interactive && options.yes})
	if errors.Is(err, ui.ErrCancelled) {
		_, _ = fmt.Fprintln(options.out, "  Nothing was changed. Run bl setup to start again.")
		return nil
	}
	if err != nil {
		return err
	}
	if summary.Problems > 0 {
		return setupProblems(summary.Problems)
	}
	return nil
}

// setupProblems is returned when some steps failed. The summary already
// shows them, so the command only exits with an error status.
type setupProblems int

func (n setupProblems) Error() string {
	return fmt.Sprintf("setup finished with %d %s", int(n), plural(int(n), "problem", "problems"))
}

// setupError describes a failed step briefly.
func setupError(err error) string {
	if errors.Is(err, ui.ErrSkipped) {
		return "skipped"
	}
	if errors.Is(err, context.Canceled) {
		return "stopped"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timed out"
	}
	return err.Error()
}

func chosenAgents(plan setupPlan, chosen map[string]bool) []skillsAgent {
	var agents []skillsAgent
	for _, agent := range plan.agents {
		if chosen["agent:"+agent.id] {
			agents = append(agents, agent)
		}
	}
	return agents
}

// setupTasks turns the chosen items into work. Skills and each agent's MCP
// servers install in parallel; the browser login comes last.
func setupTasks(options setupOptions, plan setupPlan, chosen map[string]bool, outcome *setupOutcome) []ui.Task {
	agents := chosenAgents(plan, chosen)
	// Updating the skills also refreshes the agents that already have them:
	// where links are unavailable (Windows), their folders are copies.
	var skillsAgents []skillsAgent
	for _, agent := range plan.agents {
		if (chosen["agent:"+agent.id] || plan.status[agent.id].skills) && !plan.status[agent.id].noSkills {
			skillsAgents = append(skillsAgents, agent)
		}
	}
	var tasks []ui.Task
	if chosen["skills"] {
		tasks = append(tasks, ui.Task{ID: "skills", Label: "Agent skills", Run: func(ctx context.Context, c *ui.Control) (string, error) {
			c.Progress("downloading from GitHub")
			ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
			defer cancel()
			result, err := options.installSkills(ctx, skillsAgents)
			if err != nil {
				return "", err
			}
			outcome.mu.Lock()
			outcome.skills = append(append([]string(nil), result.skills...), result.preserved...)
			outcome.preserved = result.preserved
			outcome.repaired = result.repaired
			outcome.backups = result.backups
			outcome.mu.Unlock()
			return skillsInstallDetail(result), nil
		}})
	}
	var servers []mcpServer
	if chosen["mcp"] {
		servers = append(servers, options.resourceServer)
	}
	if chosen["docs"] {
		servers = append(servers, options.documentsServer)
	}
	for _, agent := range agents {
		target, ok := mcpTargets[agent.id]
		if !ok || !slices.ContainsFunc(servers, target.takes) {
			continue
		}
		tasks = append(tasks, ui.Task{ID: "mcp:" + agent.id, Label: agent.name, Group: "MCP servers", Run: func(ctx context.Context, c *ui.Control) (string, error) {
			c.Progress("adding the MCP servers")
			result := configureAgentMCP(ctx, options.mcp, target, servers)
			// Kept on failure too: a server added before it still needs its sign-in.
			outcome.mu.Lock()
			outcome.mcp[agent.id] = result
			outcome.mu.Unlock()
			if result.err != nil {
				return "", fmt.Errorf("could not edit %s: %w", displayHomePath(options.home, target.file(options.mcp)), result.err)
			}
			return result.short(), nil
		}})
	}
	if enabled, offered := chosen["tracking"]; offered {
		tasks = append(tasks, ui.Task{ID: "tracking", Label: "Error reports", Run: func(context.Context, *ui.Control) (string, error) {
			// Record the choice either way, so the CLI does not ask again.
			options.setTracking(enabled)
			outcome.mu.Lock()
			outcome.tracking = &enabled
			outcome.mu.Unlock()
			if enabled {
				return "on · anonymous", nil
			}
			return "off", nil
		}})
	}
	if chosen["login"] {
		var others []string
		for _, task := range tasks {
			others = append(others, task.ID)
		}
		tasks = append(tasks, ui.Task{ID: "login", Label: "Log in", After: others, Skippable: true, Run: func(ctx context.Context, c *ui.Control) (string, error) {
			workspace, err := options.login(ctx, c, options.workspace)
			if err != nil {
				return "", err
			}
			outcome.mu.Lock()
			outcome.workspace = workspace
			outcome.mu.Unlock()
			return "workspace " + workspace, nil
		}})
	}
	return tasks
}

// setupSummary is the final screen: everywhere Blaxel went, then what to do next.
func setupSummary(options setupOptions, plan setupPlan, chosen map[string]bool, results map[string]ui.Result, outcome *setupOutcome) ui.Summary {
	outcome.mu.Lock()
	defer outcome.mu.Unlock()
	summary := ui.Summary{Group: "Installed into"}
	skillsFailed := results["skills"].Err
	agents := chosenAgents(plan, chosen)
	for _, agent := range agents {
		var parts []string
		line := ui.Line{Label: agent.name, Group: "agents"}
		status := plan.status[agent.id]
		_, ran := results["mcp:"+agent.id]
		if offerSkills, offerMCP := setupOffers(options); !ran && status.complete(offerSkills, offerMCP) {
			parts = append(parts, "already set up")
			if !status.noSkills {
				parts = append(parts, "skills")
			}
			for _, name := range status.has {
				parts = append(parts, status.label(name))
			}
			line.Detail = strings.Join(parts, " · ")
			summary.Lines = append(summary.Lines, line)
			continue
		}
		if chosen["skills"] && skillsFailed == nil && len(outcome.skills) > 0 && !status.noSkills {
			parts = append(parts, "skills")
		}
		if result, ok := results["mcp:"+agent.id]; ok {
			if result.Err != nil {
				line.Failed, parts = true, []string{setupError(result.Err)}
			} else {
				parts = append(parts, outcome.mcp[agent.id].servers()...)
				if status.plugin && !slices.Contains(outcome.mcp[agent.id].plugin, "blaxel") {
					parts = append(parts, status.label("blaxel"))
				}
			}
		}
		if len(parts) == 0 {
			continue
		}
		line.Detail = strings.Join(parts, " · ")
		summary.Lines = append(summary.Lines, line)
	}
	// A failed skills install is one problem, shown once.
	switch {
	case chosen["skills"] && skillsFailed != nil:
		summary.Lines = append(summary.Lines, ui.Line{Label: "Agent skills", Detail: setupError(skillsFailed), Failed: true})
	case len(agents) == 0 && chosen["skills"]:
		summary.Lines = append(summary.Lines, ui.Line{Label: "Agent skills", Detail: "~/.agents/skills"})
	}
	if len(outcome.preserved) > 0 {
		detail := "links and contents unchanged"
		if len(outcome.repaired) > 0 {
			detail = "existing contents unchanged"
		}
		summary.Lines = append(summary.Lines, ui.Line{Label: "Kept managed", Detail: strings.Join(outcome.preserved, ", ") + " · " + detail})
	}
	if len(outcome.repaired) > 0 {
		summary.Lines = append(summary.Lines, ui.Line{Label: "Auto-fixed", Detail: "skill paths linked to existing copies"})
	}
	for _, backup := range outcome.backups {
		summary.Lines = append(summary.Lines, ui.Line{Label: "Backup", Detail: backup})
	}
	if shell := strings.TrimSpace(options.env(installerShellEnv)); shell != "" {
		summary.Lines = append(summary.Lines, ui.Line{Label: "Shell", Detail: shell})
	}
	var account []string
	loggedIn := plan.loggedIn
	if outcome.workspace != "" {
		loggedIn = outcome.workspace
	}
	if loggedIn != "" {
		account = append(account, "logged in to "+loggedIn)
	}
	if outcome.tracking != nil {
		account = append(account, map[bool]string{true: "error reports on", false: "error reports off"}[*outcome.tracking])
	}
	if len(account) > 0 {
		summary.Lines = append(summary.Lines, ui.Line{Label: "Blaxel", Detail: strings.Join(account, " · ")})
	}
	if result, ok := results["login"]; ok && result.Err != nil && !errors.Is(result.Err, ui.ErrSkipped) {
		summary.Lines = append(summary.Lines, ui.Line{Label: "Log in", Detail: setupError(result.Err), Failed: true})
	}
	for _, line := range summary.Lines {
		if line.Failed {
			summary.Problems++
		}
	}
	var remaining []string
	if loggedIn == "" {
		remaining = append(remaining, "bl login")
	}
	// A failed login is covered by bl login; other failed installs need a retry.
	if slices.ContainsFunc(summary.Lines, func(line ui.Line) bool { return line.Failed && line.Label != "Log in" }) {
		remaining = append(remaining, "bl setup")
	}
	summary.Title = "Blaxel is ready"
	if len(remaining) > 0 {
		summary.Title = fmt.Sprintf("%d %s left: %s", len(remaining), plural(len(remaining), "step", "steps"), strings.Join(remaining, ", "))
	}
	if reload := strings.TrimSpace(options.env(installerReloadEnv)); reload != "" {
		summary.Next = append(summary.Next, [2]string{reload, "use bl in this terminal"})
	}
	if loggedIn == "" && !options.skipLogin && !envDisabled(options.env, loginInstallEnv) {
		summary.Next = append(summary.Next, [2]string{"bl login", "log in to Blaxel"})
	}
	// The agents sign in through bl mcp with the bl login: no sign-in each.
	if len(summary.Lines) > 0 && len(agents) > 0 {
		summary.Next = append(summary.Next, [2]string{"Restart your agents", `and ask: "Create a Blaxel sandbox"`})
	}
	if summary.Problems > 0 {
		summary.Next = append(summary.Next, [2]string{"bl setup", "try again"})
	}
	return summary
}

type mcpAgentResult struct {
	added, updated, existing, plugin []string
	err                              error
}

func configureAgentMCP(ctx context.Context, env mcpEnv, target mcpTarget, servers []mcpServer) mcpAgentResult {
	result := mcpAgentResult{}
	plugin := target.pluginServes(env)
	for _, server := range servers {
		if !target.takes(server) {
			continue
		}
		if server.plugin && plugin {
			result.plugin = append(result.plugin, server.name)
			continue
		}
		change, err := addMCPServer(ctx, env, target, server)
		if err != nil {
			result.err = err
			return result
		}
		switch change {
		case mcpAdded:
			result.added = append(result.added, server.name)
		case mcpReplaced:
			result.updated = append(result.updated, server.name)
		default:
			result.existing = append(result.existing, server.name)
		}
	}
	return result
}

// short describes what changed, for a progress row.
func (r mcpAgentResult) short() string {
	var parts []string
	if len(r.added) > 0 {
		parts = append(parts, "added "+joinSkillsNames(r.added))
	}
	if len(r.updated) > 0 {
		parts = append(parts, "switched "+joinSkillsNames(r.updated)+" to bl mcp")
	}
	if len(r.existing) > 0 {
		parts = append(parts, joinSkillsNames(r.existing)+" already set up")
	}
	if len(r.plugin) > 0 {
		parts = append(parts, joinSkillsNames(r.plugin)+" from the Blaxel plugin")
	}
	return strings.Join(parts, " · ")
}

// servers names the MCP servers the agent now has, for the final screen.
func (r mcpAgentResult) servers() []string {
	var names []string
	for _, name := range slices.Concat(r.added, r.updated, r.existing) {
		names = append(names, mcpLabel(name))
	}
	for _, name := range r.plugin {
		names = append(names, mcpLabel(name)+pluginLabelSuffix)
	}
	return names
}

// pluginLabelSuffix tells that the Blaxel plugin supplies a server, which setup
// leaves as it is.
const pluginLabelSuffix = " (Blaxel plugin)"

// mcpLabel names an MCP server briefly: Blaxel MCP, docs MCP.
func mcpLabel(name string) string {
	switch name {
	case "blaxel":
		return "Blaxel MCP"
	case "blaxel-docs":
		return "docs MCP"
	}
	return name + " MCP"
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// displayHomePath shortens paths in the home directory to ~/...
func displayHomePath(home, path string) string {
	if relative, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(relative, "..") {
		return filepath.Join("~", relative)
	}
	return path
}

// setupRefreshEnv is the installer/upgrade handoff to the existing setup command.
// Refresh never runs the setup screens, authentication, or tracking tasks.
const setupRefreshEnv = "BL_INSTALL_REFRESH"

const setupRefreshCapability = "blaxel-setup-refresh-v1"

func automaticSetupOffers(env func(string) string) (skills, mcp bool) {
	if envDisabled(env, "BL_INSTALL_SETUP") {
		return false, false
	}
	return !skillsInstallDisabled(env), !automaticInstallDisabled(env, mcpInstallEnv)
}

func refreshSetup(options setupOptions) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Blaxel setup refresh could not find the home directory; retry with bl setup.")
		return
	}
	bl, err := blCommandPath(os.Executable)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Blaxel setup refresh could not locate bl; retry with bl setup.")
		return
	}
	options.home, options.env, options.out = home, os.Getenv, os.Stderr
	options.mcp = newMCPEnv(home)
	options.resourceServer, options.documentsServer = resourceMCPServer(bl), docsMCPServer()
	options.installSkills = installSkillsFor
	runSetupRefresh(context.Background(), options)
}

// Refresh deadlines. Each part has its own, so a slow skills download cannot
// leave MCP with an expired context; together they fit bl upgrade's wait.
var (
	setupRefreshSkillsTimeout = 90 * time.Second
	setupRefreshMCPTimeout    = 30 * time.Second
)

// runSetupRefresh shares setup's detection and conservative MCP writes. Each
// component is best effort, so an unavailable skills archive does not stop MCP.
func runSetupRefresh(ctx context.Context, options setupOptions) {
	agents := detectedSetupAgents(options.home, options.env)
	skills, mcp := automaticSetupOffers(options.env)
	skills = skills && !options.skipSkills
	mcp = mcp && !options.skipMCP
	parts := []string{"skills skipped", "MCP skipped"}
	problems := 0
	if skills {
		var selected []skillsAgent
		for _, agent := range agents {
			if !isMCPOnlyAgent(agent.id) {
				selected = append(selected, agent)
			}
		}
		skillsCtx, cancel := context.WithTimeout(ctx, setupRefreshSkillsTimeout)
		result, err := options.installSkills(skillsCtx, selected)
		cancel()
		if err != nil {
			parts[0] = "skills unavailable"
			problems++
		} else {
			parts[0] = fmt.Sprintf("%d skills refreshed", len(result.skills))
			if len(result.preserved) > 0 {
				parts[0] += fmt.Sprintf(", %d externally managed kept", len(result.preserved))
			}
			if len(result.backups) > 0 {
				parts[0] += fmt.Sprintf(", %d copies backed up outside skills folders", len(result.backups))
			}
		}
	}
	if mcp {
		mcpCtx, cancel := context.WithTimeout(ctx, setupRefreshMCPTimeout)
		defer cancel()
		added, updated, kept, plugin := 0, 0, 0, 0
		for _, agent := range agents {
			if target, ok := mcpTargets[agent.id]; ok {
				result := configureAgentMCP(mcpCtx, options.mcp, target, []mcpServer{options.resourceServer, options.documentsServer})
				added += len(result.added)
				updated += len(result.updated)
				kept += len(result.existing)
				plugin += len(result.plugin)
				if result.err != nil {
					problems++
				}
			}
		}
		parts[1] = fmt.Sprintf("MCP %d added, %d migrated, %d kept, %d from plugins", added, updated, kept, plugin)
	}
	if problems > 0 {
		parts = append(parts, fmt.Sprintf("%d %s; retry with bl setup", problems, plural(problems, "problem", "problems")))
	}
	_, _ = fmt.Fprintln(options.out, "Blaxel setup refresh: "+strings.Join(parts, "; ")+".")
}

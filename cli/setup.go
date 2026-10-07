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
	// trackingInstallEnv=false disables usage capture and setup's tracking choice.
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
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Set up Blaxel for your coding agents and log in",
		Long: `Set up everything Blaxel needs on this machine, then log in.

For the coding agents found on this machine (Claude Code, Codex, Cursor, ...),
setup installs the Blaxel agent skills and adds two hosted MCP servers:
blaxel, to manage your workspace resources, and blaxel-docs, to search the
Blaxel documentation. It then logs you in to Blaxel in your browser.

In a terminal, setup shows everything it found, selected, and installs it
when you press Enter; --yes installs it without showing the plan. Without a
terminal, setup installs the same defaults and skips the browser login, so
run bl login afterwards. The usage and error reports toggle controls the saved
tracking preference. New setup plans keep reports selected by default; an
existing saved choice is retained. Anonymous usage capture is disabled in CI,
by any nonempty DO_NOT_TRACK, or by BL_INSTALL_TRACKING=false. Error reports
keep the SDK's existing DO_NOT_TRACK semantics.

Events, properties and opt-outs: ` + core.UsageDisclosureURL + `

Setup only adds what is missing. Existing MCP server entries are left
unchanged, and it is safe to run again after installing another agent.`,
		Example: `  # See what setup found, then press Enter to install
  bl setup

  # Install the defaults without showing the plan
  bl setup --yes

  # Set up only Claude Code and Codex, without logging in
  bl setup --agent claude-code,codex --skip-login`,
		Args:         cobra.NoArgs,
		SilenceUsage: true, SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := os.UserHomeDir()
			if err != nil {
				return err
			}
			options.home = home
			options.env = os.Getenv
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
			options.resourceServer, options.documentsServer = resourceMCPServer(), docsMCPServer()
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
	_ = cmd.RegisterFlagCompletionFunc("agent", func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
		ids := make([]string, 0, len(skillsAgents))
		for _, agent := range skillsAgents {
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
		names, err := auth.LoginWorkspaces(creds)
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
	skills  bool
	mcp     bool     // the agent takes MCP servers
	has     []string // Blaxel MCP servers it already has
	missing []string // Blaxel MCP servers it lacks
}

// complete reports whether the agent has everything setup offers.
func (s agentStatus) complete(skills, mcp bool) bool {
	return (!skills || s.skills) && (!mcp || !s.mcp || len(s.missing) == 0)
}

// fresh reports whether an earlier setup left nothing in the agent.
func (s agentStatus) fresh() bool { return !s.skills && len(s.has) == 0 }

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
		plan.agents = detectedSkillsAgents(options.home, options.env)
	}
	plan.loggedIn = options.loginState(options.workspace)
	plan.skills = installedBlaxelSkills(options.home, options.env)
	plan.status = map[string]agentStatus{}
	for _, agent := range plan.agents {
		status := agentStatus{skills: skillsInstalledFor(agent, options.home, options.env, plan.skills)}
		if target, ok := mcpTargets[agent.id]; ok {
			status.mcp = true
			plugin := target.hasPlugin != nil && target.hasPlugin(options.mcp)
			for _, server := range []mcpServer{options.resourceServer, options.documentsServer} {
				if (server.plugin && plugin) || (target.has != nil && target.has(options.mcp, server.name)) {
					status.has = append(status.has, server.name)
				} else {
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
	for _, agent := range skillsAgents {
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
		if status.mcp && offerMCP {
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
			var missing []string
			if offerSkills && !status.skills {
				missing = append(missing, "skills")
			}
			if offerMCP && len(status.missing) == 2 {
				missing = append(missing, "MCP")
			} else if offerMCP {
				for _, name := range status.missing {
					missing = append(missing, name+" MCP")
				}
			}
			detail = "add " + strings.Join(missing, ", ")
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
		items = append(items, &ui.Item{ID: "tracking", Group: "This machine", Label: "Usage and error reports", Detail: "usage is anonymous; see privacy docs", On: enabled})
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

func runSetup(ctx context.Context, options setupOptions) (setupErr error) {
	var agents, components []string
	status := "success"
	var taskErr error
	defer func() {
		if setupErr != nil {
			status = "failure"
		}
		if taskErr == nil {
			taskErr = setupErr
		}
		core.TrackCLISetup(agents, components, status, taskErr)
	}()
	if ctx == nil {
		ctx = context.Background()
	}
	plan, err := newSetupPlan(options)
	if err != nil {
		return err
	}
	for _, agent := range detectedSkillsAgents(options.home, options.env) {
		agents = append(agents, agent.id)
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
			if item.On {
				components = append(components, item.ID)
			}
		}
		ids := make([]string, 0, len(results))
		for id := range results {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		for _, id := range ids {
			if err := results[id].Err; err != nil && !errors.Is(err, ui.ErrSkipped) {
				taskErr = err
				break
			}
		}
		return setupSummary(options, plan, chosen, results, outcome)
	}
	summary, err := setup.Run(ctx, ui.Options{Out: options.out, Interactive: options.interactive && !options.yes, Yes: options.interactive && options.yes})
	if errors.Is(err, ui.ErrCancelled) {
		status = "cancelled"
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
		if chosen["agent:"+agent.id] || plan.status[agent.id].skills {
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
		if !ok || len(servers) == 0 {
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
		tasks = append(tasks, ui.Task{ID: "tracking", Label: "Usage and error reports", Run: func(context.Context, *ui.Control) (string, error) {
			// Record the choice either way, so the CLI does not ask again.
			options.setTracking(enabled)
			outcome.mu.Lock()
			outcome.tracking = &enabled
			outcome.mu.Unlock()
			if enabled {
				return "on", nil
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
			core.TrackCLILogin("started", nil)
			workspace, err := options.login(ctx, c, options.workspace)
			switch {
			case err == nil:
				core.TrackCLILogin("success", nil)
			case errors.Is(ctx.Err(), context.Canceled):
				// Esc skips this task by cancelling its context; quitting setup
				// does too. Neither is a failed login.
				core.TrackCLILogin("cancelled", nil)
			default:
				core.TrackCLILogin("failure", err)
			}
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
			parts = append(parts, "already set up", "skills")
			for _, name := range status.has {
				parts = append(parts, mcpLabel(name))
			}
			line.Detail = strings.Join(parts, " · ")
			summary.Lines = append(summary.Lines, line)
			continue
		}
		if chosen["skills"] && skillsFailed == nil && len(outcome.skills) > 0 {
			parts = append(parts, "skills")
		}
		if result, ok := results["mcp:"+agent.id]; ok {
			if result.Err != nil {
				line.Failed, parts = true, []string{setupError(result.Err)}
			} else {
				parts = append(parts, outcome.mcp[agent.id].servers()...)
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
		account = append(account, map[bool]string{true: "usage and error reports on", false: "usage and error reports off"}[*outcome.tracking])
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
	// The hosted Blaxel MCP server signs each agent in once, with OAuth.
	var signIns [][2]string
	for _, agent := range agents {
		target, ok := mcpTargets[agent.id]
		if result, added := outcome.mcp[agent.id]; added && result.needsSignIn && ok && target.signIn != "" {
			signIns = append(signIns, [2]string{agent.name, target.signIn})
		}
	}
	switch {
	case len(signIns) > 0:
		summary.Next = append(summary.Next, [2]string{"Restart your agents", "and sign each in to Blaxel MCP:"})
		summary.Next = append(summary.Next, signIns...)
	case len(summary.Lines) > 0 && len(agents) > 0:
		summary.Next = append(summary.Next, [2]string{"Restart your agents", `and ask: "Create a Blaxel sandbox"`})
	}
	if summary.Problems > 0 {
		summary.Next = append(summary.Next, [2]string{"bl setup", "try again"})
	}
	return summary
}

type mcpAgentResult struct {
	added, existing, plugin []string
	needsSignIn             bool
	err                     error
}

func configureAgentMCP(ctx context.Context, env mcpEnv, target mcpTarget, servers []mcpServer) mcpAgentResult {
	result := mcpAgentResult{}
	for _, server := range servers {
		if server.plugin && target.hasPlugin != nil && target.hasPlugin(env) {
			result.plugin = append(result.plugin, server.name)
			continue
		}
		added, err := target.add(ctx, env, server)
		if err != nil {
			result.err = err
			return result
		}
		if added {
			result.added = append(result.added, server.name)
		} else {
			result.existing = append(result.existing, server.name)
		}
		if server.plugin {
			result.needsSignIn = added
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
	for _, name := range slices.Concat(r.added, r.existing, r.plugin) {
		names = append(names, mcpLabel(name))
	}
	return names
}

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

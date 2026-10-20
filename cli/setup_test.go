package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blaxel-ai/toolkit/cli/agentsetup"
	"github.com/blaxel-ai/toolkit/cli/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testResourceServer = agentsetup.ResourceMCPServer("/opt/blaxel/bin/bl")

var testDocsServer = agentsetup.MCPServer{Name: "blaxel-docs", URL: agentsetup.DocsMCPURL}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0644))
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

// archiveEntry is one entry of a test repository tarball.
type archiveEntry struct {
	name, body, link string
	mode             int64
}

const testSkillManifest = "---\nname: %s\ndescription: Use the %s.\n---\n\n# %s\n"

func skillManifest(name string) string {
	return strings.ReplaceAll(testSkillManifest, "%s", name)
}

// testSkillsEntries mirrors the agent-skills repository layout, with the edge
// cases the installer must handle.
func testSkillsEntries() []archiveEntry {
	return []archiveEntry{
		{name: "README.md", body: "# Agent skills\n"},
		{name: "skills/blaxel-cli/SKILL.md", body: skillManifest("blaxel-cli")},
		{name: "skills/blaxel-cli/metadata.json", body: `{"version":"1"}`},
		{name: "skills/blaxel-cli/references/login.md", body: "# bl login\n"},
		{name: "skills/blaxel-cli/references/alias.md", link: "login.md"},
		{name: "skills/blaxel-cli/outside.md", link: "../../README.md"},
		{name: "skills/blaxel-cli/scripts/generate.sh", body: "#!/bin/sh\necho docs\n", mode: 0o775},
		{name: "skills/blaxel-cli/__pycache__/cache.pyc", body: "cache"},
		{name: "skills/blaxel-sdk/SKILL.md", body: skillManifest("blaxel-sdk")},
		{name: "skills/blaxel-sdk/references/sdk-python.md", body: "# Python\n"},
		{name: "skills/internal-tool/SKILL.md", body: "---\nname: internal-tool\ndescription: Internal.\nmetadata:\n  internal: true\n---\n"},
		{name: "skills/broken/SKILL.md", body: "---\nname: broken\n---\n"},
		{name: "skills/notes/README.md", body: "no skill here"},
		{name: "skills/../../escape/SKILL.md", body: skillManifest("escape")},
		{name: "prompts/SKILL.md", body: skillManifest("prompt")},
	}
}

func buildSkillsArchive(t *testing.T, entries []archiveEntry) []byte {
	t.Helper()
	var buffer bytes.Buffer
	compressed := gzip.NewWriter(&buffer)
	writer := tar.NewWriter(compressed)
	// GitHub archives start with the commit as a global header.
	require.NoError(t, writer.WriteHeader(&tar.Header{Typeflag: tar.TypeXGlobalHeader, Name: "pax_global_header",
		PAXRecords: map[string]string{"comment": "0123456789abcdef0123456789abcdef01234567"}}))
	require.NoError(t, writer.WriteHeader(&tar.Header{Typeflag: tar.TypeDir, Name: "agent-skills-HEAD/", Mode: 0o775}))
	for _, entry := range entries {
		header := &tar.Header{Name: "agent-skills-HEAD/" + entry.name, Mode: 0o664, Size: int64(len(entry.body)), Typeflag: tar.TypeReg}
		if entry.mode != 0 {
			header.Mode = entry.mode
		}
		if entry.link != "" {
			header.Typeflag, header.Linkname, header.Size, header.Mode = tar.TypeSymlink, entry.link, 0, 0o777
		}
		require.NoError(t, writer.WriteHeader(header))
		if entry.link == "" {
			_, err := writer.Write([]byte(entry.body))
			require.NoError(t, err)
		}
	}
	require.NoError(t, writer.Close())
	require.NoError(t, compressed.Close())
	return buffer.Bytes()
}

func noEnv(string) string { return "" }

// resolvedTempDir returns t.TempDir() with links and Windows 8.3 short names
// (C:\Users\RUNNER~1) expanded, matching the resolved paths in error messages.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := agentsetup.EvalSkillLinks(t.TempDir())
	require.NoError(t, err)
	return dir
}

func managedSkillLink(t *testing.T, target, link string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0755))
	absolute := target
	if !filepath.IsAbs(absolute) {
		absolute = filepath.Join(filepath.Dir(link), target)
	}
	require.NoError(t, agentsetup.CreateSkillDirectoryLink(absolute, target, link))
}

type fakeCommand struct {
	name string
	args []string
}

func testMCPEnv(home string, env map[string]string, commands *[]fakeCommand, output string, runErr error) agentsetup.MCPEnv {
	return agentsetup.MCPEnv{
		Home: home, Config: filepath.Join(home, ".config"),
		Env: func(key string) string { return env[key] },
		LookPath: func(name string) (string, error) {
			if _, ok := env["PATH_HAS_"+name]; ok {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not found")
		},
		Exists: func(path string) bool { return env["MISSING:"+path] == "" },
		Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			*commands = append(*commands, fakeCommand{name, args})
			return []byte(output), runErr
		},
	}
}

type setupRecorder struct {
	skillsAgents [][]string
	logins       []string
	tracking     []bool
	output       *os.File
}

// text is everything setup printed.
func (r *setupRecorder) text(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(r.output.Name())
	require.NoError(t, err)
	return string(data)
}

func testSetupOptions(t *testing.T, home string, env map[string]string, recorder *setupRecorder) setupOptions {
	t.Helper()
	var commands []fakeCommand
	if recorder.output == nil {
		output, err := os.Create(filepath.Join(t.TempDir(), "output.txt"))
		require.NoError(t, err)
		t.Cleanup(func() { _ = output.Close() })
		recorder.output = output
	}
	return setupOptions{
		home: home,
		env:  func(key string) string { return env[key] },
		out:  recorder.output,
		installSkills: func(_ context.Context, agents []agentsetup.SkillsAgent) (agentsetup.SkillsInstallResult, error) {
			targets, names := agentsetup.SkillsTargets(agents)
			recorder.skillsAgents = append(recorder.skillsAgents, targets)
			if env["SKILLS_OFFLINE"] != "" {
				return agentsetup.SkillsInstallResult{}, errors.New("downloading the skills: no network")
			}
			return agentsetup.SkillsInstallResult{Skills: []string{"blaxel-cli", "blaxel-sdk"}, Agents: names}, nil
		},
		login: func(_ context.Context, _ *ui.Control, workspace string) (string, error) {
			recorder.logins = append(recorder.logins, workspace)
			if env["LOGIN_FAILS"] != "" {
				return "", errors.New("no workspaces are available for your account")
			}
			return cmpOr(workspace, "main"), nil
		},
		trackingConfigured: func() bool { return env["TRACKING_SET"] != "" },
		trackingEnabled:    func() bool { return env["TRACKING_ENABLED"] == "true" },
		setTracking:        func(enabled bool) { recorder.tracking = append(recorder.tracking, enabled) },
		loginState:         func(string) string { return env["LOGGED_IN"] },
		mcp:                testMCPEnv(home, env, &commands, "", nil),
		resourceServer:     testResourceServer, documentsServer: testDocsServer,
	}
}

func TestRunSetupDefaultsSetUpDetectedAgents(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{".claude", ".codex", ".cursor", ".pi/agent"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, dir), 0755))
	}
	recorder := &setupRecorder{}
	require.NoError(t, runSetup(context.Background(), testSetupOptions(t, home, map[string]string{}, recorder)))

	assert.Equal(t, [][]string{{"universal", "claude-code", "pi"}}, recorder.skillsAgents)
	assert.Contains(t, readTestFile(t, filepath.Join(home, ".claude.json")), `"blaxel-docs"`)
	codex := readTestFile(t, filepath.Join(home, ".codex", "config.toml"))
	assert.Contains(t, codex, "[mcp_servers.blaxel]")
	assert.Contains(t, codex, "[mcp_servers.blaxel-docs]")
	cursor := readTestFile(t, filepath.Join(home, ".cursor", "mcp.json"))
	assert.Contains(t, cursor, `"command": "/opt/blaxel/bin/bl"`)
	assert.Contains(t, cursor, `"url": "https://docs.blaxel.ai/mcp"`)
	assert.NoFileExists(t, filepath.Join(home, ".pi", "agent", "mcp.json"), "agents without MCP support get the skills only")
	assert.Empty(t, recorder.logins, "the browser login needs a terminal")

	// Running again changes nothing.
	before := readTestFile(t, filepath.Join(home, ".cursor", "mcp.json"))
	require.NoError(t, runSetup(context.Background(), testSetupOptions(t, home, map[string]string{}, recorder)))
	assert.Equal(t, before, readTestFile(t, filepath.Join(home, ".cursor", "mcp.json")))
}

func TestRunSetupLogsInOnlyWhenNeeded(t *testing.T) {
	home := t.TempDir()
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{}, recorder)
	options.yes = true
	options.interactive = false
	require.NoError(t, runSetup(context.Background(), options))
	assert.Empty(t, recorder.logins)

	// In a terminal with --yes, setup logs in after setting up the agents.
	options.interactive = true
	options.workspace = "my-workspace"
	require.NoError(t, runSetup(context.Background(), options))
	assert.Equal(t, []string{"my-workspace"}, recorder.logins)

	options = testSetupOptions(t, home, map[string]string{"LOGGED_IN": "ws"}, recorder)
	options.interactive, options.yes = true, true
	require.NoError(t, runSetup(context.Background(), options))
	assert.Len(t, recorder.logins, 1, "an existing login is kept")

	options = testSetupOptions(t, home, map[string]string{loginInstallEnv: "false"}, recorder)
	options.interactive, options.yes = true, true
	require.NoError(t, runSetup(context.Background(), options))
	assert.Len(t, recorder.logins, 1)
}

func TestRunSetupFinishesWhenLoginFails(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".cursor"), 0755))
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{"LOGIN_FAILS": "1"}, recorder)
	options.interactive, options.yes = true, true
	err := runSetup(context.Background(), options)
	assert.ErrorContains(t, err, "1 problem")
	assert.Len(t, recorder.logins, 1)
	assert.Len(t, recorder.skillsAgents, 1, "the agents are set up before logging in")
	assert.FileExists(t, filepath.Join(home, ".cursor", "mcp.json"))
}

func TestRunSetupHonorsSwitches(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".cursor"), 0755))
	recorder := &setupRecorder{}
	require.NoError(t, runSetup(context.Background(), testSetupOptions(t, home, map[string]string{mcpInstallEnv: "false", agentsetup.SkillsInstallEnv: "false"}, recorder)))
	assert.Empty(t, recorder.skillsAgents)
	assert.NoFileExists(t, filepath.Join(home, ".cursor", "mcp.json"))

	options := testSetupOptions(t, home, map[string]string{}, recorder)
	options.skipMCP = true
	require.NoError(t, runSetup(context.Background(), options))
	assert.Len(t, recorder.skillsAgents, 1)
	assert.NoFileExists(t, filepath.Join(home, ".cursor", "mcp.json"))
}

func TestRunSetupSkillsFailureStillAddsMCP(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".cursor"), 0755))
	recorder := &setupRecorder{}
	err := runSetup(context.Background(), testSetupOptions(t, home, map[string]string{"SKILLS_OFFLINE": "1"}, recorder))
	assert.ErrorContains(t, err, "1 problem")
	assert.Len(t, recorder.skillsAgents, 1)
	assert.FileExists(t, filepath.Join(home, ".cursor", "mcp.json"))
}

func TestRunSetupExplicitAgents(t *testing.T) {
	home := t.TempDir()
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{}, recorder)
	options.agents = []string{"cursor", "claude-code", "cursor"}
	require.NoError(t, runSetup(context.Background(), options))
	assert.Equal(t, [][]string{{"universal", "claude-code"}}, recorder.skillsAgents)
	assert.FileExists(t, filepath.Join(home, ".cursor", "mcp.json"), "an explicit agent is set up even before it is detected")

	options.agents = []string{"nope"}
	err := runSetup(context.Background(), options)
	assert.ErrorContains(t, err, `unknown agent "nope"`)
	assert.ErrorContains(t, err, "claude-code")
}

func TestRunSetupReportsMCPProblems(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".gemini", "settings.json"), "{ // comment\n}")
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".cursor"), 0755))
	recorder := &setupRecorder{}
	err := runSetup(context.Background(), testSetupOptions(t, home, map[string]string{}, recorder))
	assert.ErrorContains(t, err, "1 problem")
	assert.FileExists(t, filepath.Join(home, ".cursor", "mcp.json"), "other agents are still set up")
	assert.Equal(t, "{ // comment\n}", readTestFile(t, filepath.Join(home, ".gemini", "settings.json")))
}

func TestDisplayHomePath(t *testing.T) {
	assert.Equal(t, filepath.Join("~", ".codex", "config.toml"), displayHomePath("/h", "/h/.codex/config.toml"))
	assert.Equal(t, "/etc/x", displayHomePath("/h", "/etc/x"))
}

func cmpOr(value, fallback string) string {
	if value != "" {
		return value
	}
	return fallback
}

// The plan shows what was found, with everything selected; error reports
// default to on and the choice is recorded either way.
func TestSetupPlanAndSummary(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{".claude", ".cursor", ".pi/agent"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, dir), 0755))
	}
	env := map[string]string{installerShellEnv: "bl on PATH · zsh completions", installerReloadEnv: "source ~/.zshrc"}
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, env, recorder)
	options.interactive = true
	plan, err := newSetupPlan(options)
	require.NoError(t, err)
	var ids []string
	for _, item := range setupItems(options, plan) {
		ids = append(ids, item.ID)
		if item.Done == "" {
			assert.True(t, item.On, item.ID)
		}
	}
	assert.Equal(t, []string{"agent:claude-code", "agent:cursor", "agent:pi", "skills", "mcp", "docs", "shell", "login", "tracking"}, ids)

	options.yes = true
	options.interactive = true
	require.NoError(t, runSetup(context.Background(), options))
	assert.Equal(t, []bool{true}, recorder.tracking)
	assert.Equal(t, []string{""}, recorder.logins)
	text := recorder.text(t)
	for _, expected := range []string{
		"Claude Code    skills · Blaxel MCP · docs MCP",
		"Pi             skills",
		"Shell          bl on PATH · zsh completions",
		"Blaxel         logged in to main · error reports on",
		"source ~/.zshrc",
		"Blaxel is ready",
		// bl mcp signs the agents in with the bl login.
		`Restart your agents  and ask: "Create a Blaxel sandbox"`,
	} {
		assert.Contains(t, text, expected)
	}
	assert.NotContains(t, text, "mcp login")
	assert.NotContains(t, text, "sign each in")

	// A recorded choice remains toggleable; CI never turns reports on.
	env["TRACKING_SET"] = "1"
	assert.Contains(t, itemIDs(setupItems(options, plan)), "tracking")
	delete(env, "TRACKING_SET")
	env["CI"] = "true"
	assert.NotContains(t, itemIDs(setupItems(options, plan)), "tracking")
}

func TestSetupRecordsErrorReportsOff(t *testing.T) {
	recorder := &setupRecorder{}
	options := testSetupOptions(t, t.TempDir(), map[string]string{trackingInstallEnv: "false"}, recorder)
	require.NoError(t, runSetup(context.Background(), options))
	assert.Equal(t, []bool{false}, recorder.tracking)
	assert.Contains(t, recorder.text(t), "error reports off")
}

func itemIDs(items []*ui.Item) []string {
	var ids []string
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

// A second run shows what is already set up and offers only what is new.
func TestSetupRerunShowsWhatIsNew(t *testing.T) {
	home := t.TempDir()
	for _, dir := range []string{".claude", ".cursor"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, dir), 0755))
	}
	// An earlier setup: skills for Claude Code (linked) and Cursor (shared folder), and both MCP servers in Claude Code.
	for _, skill := range []string{"blaxel-cli", "blaxel-sdk"} {
		writeTestFile(t, filepath.Join(home, ".agents", "skills", skill, "SKILL.md"), skillManifest(skill))
		writeTestFile(t, filepath.Join(home, ".claude", "skills", skill, "SKILL.md"), skillManifest(skill))
	}
	writeTestFile(t, filepath.Join(home, ".agents", ".skill-lock.json"), `{"version":3,"skills":{"blaxel-cli":{"source":"blaxel-ai/agent-skills"},"blaxel-sdk":{"source":"blaxel-ai/agent-skills"},"mine":{"source":"me/skills"}}}`)
	writeTestFile(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{"blaxel":{"type":"http","url":"https://example.test/mcp"},"blaxel-docs":{"type":"http","url":"x"}}}`)
	// A new agent since then.
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".codex"), 0755))

	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{"LOGGED_IN": "main", "TRACKING_SET": "1"}, recorder)
	plan, err := newSetupPlan(options)
	require.NoError(t, err)
	items := map[string]*ui.Item{}
	var group string
	for _, item := range setupItems(options, plan) {
		items[item.ID] = item
		if item.ID == "agent:codex" {
			group = item.Group
		}
	}
	assert.Equal(t, "Coding agents · 2 to set up", group)
	assert.Equal(t, "set up · skills · MCP", items["agent:claude-code"].Done)
	assert.Equal(t, "add MCP", items["agent:cursor"].Detail, "Cursor has the skills through ~/.agents/skills but no MCP servers")
	assert.Equal(t, "add MCP", items["agent:codex"].Detail)
	assert.True(t, items["skills"].Update)
	assert.Equal(t, "update to the latest", items["skills"].Detail)
	assert.True(t, items["mcp"].On)
	assert.Equal(t, "main", items["account"].Done)

	require.NoError(t, runSetup(context.Background(), options))
	text := recorder.text(t)
	assert.Contains(t, text, "Claude Code    already set up · skills · Blaxel MCP · docs MCP")
	assert.Contains(t, text, "Codex          skills · Blaxel MCP · docs MCP")
	assert.Contains(t, readTestFile(t, filepath.Join(home, ".claude.json")), "https://example.test/mcp", "existing servers are left alone")
	assert.NotContains(t, text, "mcp login", "no agent needs a sign-in of its own")
	require.Len(t, recorder.skillsAgents, 1)
	assert.Contains(t, recorder.skillsAgents[0], "claude-code", "updating the skills refreshes the agents that already have them")
}

func TestSetupLeavesDoNotTrackAlone(t *testing.T) {
	recorder := &setupRecorder{}
	options := testSetupOptions(t, t.TempDir(), map[string]string{"DO_NOT_TRACK": "1"}, recorder)
	plan, err := newSetupPlan(options)
	require.NoError(t, err)
	assert.NotContains(t, itemIDs(setupItems(options, plan)), "tracking")
	require.NoError(t, runSetup(context.Background(), options))
	assert.Empty(t, recorder.tracking, "DO_NOT_TRACK is already a choice")
}

// A failure adding one server is reported, and setup suggests trying again.
func TestSetupReportsAPartialMCPFailure(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0755))
	recorder := &setupRecorder{}
	env := map[string]string{"PATH_HAS_claude": "", "LOGGED_IN": "main", "TRACKING_SET": "1"}
	options := testSetupOptions(t, home, env, recorder)
	calls := 0
	options.mcp.Run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		calls++
		if calls > 1 {
			return []byte("boom"), errors.New("exit status 1")
		}
		return []byte("Added HTTP MCP server"), nil
	}
	err := runSetup(context.Background(), options)
	var problems setupProblems
	require.ErrorAs(t, err, &problems)
	text := recorder.text(t)
	assert.Contains(t, text, "boom")
	assert.Contains(t, text, "try again")
}

// Agents set up by v0.1.119 have the hosted server, with its own sign-in:
// the next setup switches them to bl mcp and leaves other entries alone.
func TestRunSetupSwitchesHostedServersToBlMCP(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".cursor", "mcp.json"), `{"mcpServers":{"other":{"url":"https://example.com"},"blaxel":{"url":"https://api.blaxel.ai/v0/mcp"},"blaxel-docs":{"url":"https://docs.blaxel.ai/mcp"}}}`)
	writeTestFile(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.blaxel]\nurl = \"https://api.blaxel.ai/v0/mcp\"\n\n[mcp_servers.blaxel-docs]\nurl = \"https://docs.blaxel.ai/mcp\"\n")
	writeTestFile(t, filepath.Join(home, ".gemini", "settings.json"), `{"mcpServers":{"blaxel":{"httpUrl":"https://api.blaxel.ai/v0/mcp","headers":{"Authorization":"Bearer mine"}}}}`)
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{"LOGGED_IN": "main", "TRACKING_SET": "1", agentsetup.SkillsInstallEnv: "false"}, recorder)
	plan, err := newSetupPlan(options)
	require.NoError(t, err)
	items := map[string]*ui.Item{}
	for _, item := range setupItems(options, plan) {
		items[item.ID] = item
	}
	assert.Equal(t, "switch Blaxel MCP to your bl login", items["agent:cursor"].Detail)
	assert.Equal(t, "switch Blaxel MCP to your bl login", items["agent:codex"].Detail)
	assert.Equal(t, "add docs MCP", items["agent:gemini-cli"].Detail, "a hand-made entry is left alone")

	require.NoError(t, runSetup(context.Background(), options))
	cursor := readTestFile(t, filepath.Join(home, ".cursor", "mcp.json"))
	assert.Contains(t, cursor, `"command": "/opt/blaxel/bin/bl"`)
	assert.NotContains(t, cursor, "api.blaxel.ai")
	assert.Less(t, strings.Index(cursor, `"other"`), strings.Index(cursor, `"blaxel"`), "the servers keep their order")
	codex := readTestFile(t, filepath.Join(home, ".codex", "config.toml"))
	assert.Contains(t, codex, "[mcp_servers.blaxel]\ncommand = \"/opt/blaxel/bin/bl\"\nargs = [\"mcp\"]\n")
	assert.Contains(t, codex, "[mcp_servers.blaxel-docs]\nurl = \"https://docs.blaxel.ai/mcp\"\n")
	assert.Contains(t, readTestFile(t, filepath.Join(home, ".gemini", "settings.json")), "Bearer mine")
	assert.Contains(t, recorder.text(t), "Cursor         Blaxel MCP · docs MCP")

	// Nothing changes the second time.
	before := cursor
	require.NoError(t, runSetup(context.Background(), testSetupOptions(t, home, map[string]string{"LOGGED_IN": "main", "TRACKING_SET": "1", agentsetup.SkillsInstallEnv: "false"}, recorder)))
	assert.Equal(t, before, readTestFile(t, filepath.Join(home, ".cursor", "mcp.json")))
}

func TestRunSetupAddsBlMCPToClaudeDesktop(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{"APPDATA": filepath.Join(home, "AppData", "Roaming"), "XDG_CONFIG_HOME": filepath.Join(home, ".config")}
	dir := agentsetup.ClaudeDesktopDir(agentsetup.NewSkillsAgentPaths(home, func(key string) string { return env[key] }))
	writeTestFile(t, filepath.Join(dir, "claude_desktop_config.json"), `{"preferences":{"theme":"dark"}}`)
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, env, recorder)
	options.mcp.Config = env["XDG_CONFIG_HOME"]
	plan, err := newSetupPlan(options)
	require.NoError(t, err)
	require.Len(t, plan.agents, 1)
	assert.Equal(t, "claude-desktop", plan.agents[0].ID)
	items := setupItems(options, plan)
	assert.Equal(t, "MCP", items[0].Detail, "Claude Desktop takes no skills folder")

	require.NoError(t, runSetup(context.Background(), options))
	config := readTestFile(t, filepath.Join(dir, "claude_desktop_config.json"))
	assert.Contains(t, config, `"theme": "dark"`)
	assert.Contains(t, config, `"command": "/opt/blaxel/bin/bl"`)
	assert.NotContains(t, config, "blaxel-docs", "its configuration file runs local servers only")
	assert.Equal(t, [][]string{{"universal"}}, recorder.skillsAgents, "no skills are linked into Claude Desktop")
	assert.Contains(t, recorder.text(t), "Claude Desktop Blaxel MCP")
}

// With the Blaxel plugin installed, setup leaves the plugin's blaxel server
// alone and adds no second one. It exits cleanly, every time it runs.
func TestSetupLeavesThePluginServerAlone(t *testing.T) {
	for _, agent := range []string{"claude-code", "codex"} {
		t.Run(agent, func(t *testing.T) {
			home := t.TempDir()
			root := filepath.Join(home, "plugin", "blaxel", "1.0.0")
			if agent == "claude-code" {
				data, _ := json.Marshal(map[string]any{"plugins": map[string]any{"blaxel@blaxel": []any{map[string]any{"installPath": root}}}})
				writeTestFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), string(data))
			} else {
				root = filepath.Join(home, ".codex", "plugins", "cache", "blaxel", "blaxel", "1.0.0")
				writeTestFile(t, filepath.Join(home, ".codex", "config.toml"), "[plugins.\"blaxel@blaxel\"]\nenabled = true\n")
			}
			hosted := `{"mcpServers":{"blaxel":{"type":"http","url":"https://api.blaxel.ai/v0/mcp"}}}`
			writeTestFile(t, filepath.Join(root, ".mcp.json"), hosted)
			recorder := &setupRecorder{}
			run := func() {
				options := testSetupOptions(t, home, map[string]string{"LOGGED_IN": "main", "TRACKING_SET": "1", agentsetup.SkillsInstallEnv: "false"}, recorder)
				options.agents = []string{agent}
				require.NoError(t, runSetup(context.Background(), options))
			}
			run()
			assert.Contains(t, recorder.text(t), "docs MCP · Blaxel MCP (Blaxel plugin)")
			run()
			target := agentsetup.MCPTargets[agent]
			e := testMCPEnv(home, map[string]string{}, &[]fakeCommand{}, "", nil)
			assert.Nil(t, target.Entry(e, "blaxel"), "the plugin's server is the only Blaxel server")
			assert.NotNil(t, target.Entry(e, "blaxel-docs"))
			assert.Equal(t, hosted, readTestFile(t, filepath.Join(root, ".mcp.json")), "plugin files are never edited")

			// A plugin that no longer bundles the server leaves the job to setup.
			require.NoError(t, os.Remove(filepath.Join(root, ".mcp.json")))
			run()
			command, args := agentsetup.EntryCommand(target.Entry(e, "blaxel"))
			assert.Equal(t, "/opt/blaxel/bin/bl", command)
			assert.Equal(t, []string{"mcp"}, args)
		})
	}
}

func TestSetupReportsAutomaticSkillRepair(t *testing.T) {
	home := resolvedTempDir(t)
	root := filepath.Join(home, ".agents", "skills")
	writeTestFile(t, filepath.Join(root, "blaxel", "blaxel-cli", "SKILL.md"), skillManifest("blaxel-cli"))
	writeTestFile(t, filepath.Join(root, "blaxel-cli", "SKILL.md"), "old flat")
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{"LOGGED_IN": "main", "TRACKING_SET": "1"}, recorder)
	options.skipMCP, options.skipLogin = true, true
	options.installSkills = func(_ context.Context, agents []agentsetup.SkillsAgent) (agentsetup.SkillsInstallResult, error) {
		return agentsetup.InstallSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, agents, time.Now())
	}
	require.NoError(t, runSetup(context.Background(), options))
	output := recorder.text(t)
	assert.Contains(t, output, "Blaxel is ready")
	assert.Contains(t, output, "Auto-fixed")
	assert.Contains(t, output, "Backup")
	assert.Contains(t, output, "existing contents unchanged")
	assert.NotContains(t, output, "try again")
	assert.NotContains(t, output, "links and contents unchanged")
}

func TestSetupKeepsSavedErrorReportingToggleVisible(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(map[bool]string{true: "enabled", false: "disabled"}[enabled], func(t *testing.T) {
			recorder := &setupRecorder{}
			env := map[string]string{"TRACKING_SET": "1", "TRACKING_ENABLED": map[bool]string{true: "true", false: "false"}[enabled]}
			options := testSetupOptions(t, t.TempDir(), env, recorder)
			plan, err := newSetupPlan(options)
			require.NoError(t, err)
			var found bool
			for _, item := range setupItems(options, plan) {
				if item.ID == "tracking" {
					found = true
					assert.Equal(t, "This machine", item.Group)
					assert.Equal(t, "Error reports", item.Label)
					assert.Empty(t, item.Done, "a saved preference must remain toggleable")
					assert.Equal(t, enabled, item.On)
				}
			}
			require.True(t, found)
			options.skipSkills, options.skipMCP, options.skipLogin = true, true, true
			require.NoError(t, runSetup(context.Background(), options))
			assert.Equal(t, []bool{enabled}, recorder.tracking, "repeat setup must not reset the saved preference")
			tasks := setupTasks(options, plan, map[string]bool{"tracking": !enabled}, &setupOutcome{})
			require.Len(t, tasks, 1)
			_, err = tasks[0].Run(context.Background(), nil)
			require.NoError(t, err)
			assert.Equal(t, []bool{enabled, !enabled}, recorder.tracking, "the toggle must save a new choice")
		})
	}
}

func TestSetupErrorReportingEnvironmentOverridesSavedChoice(t *testing.T) {
	for _, value := range []string{"true", "false"} {
		recorder := &setupRecorder{}
		options := testSetupOptions(t, t.TempDir(), map[string]string{
			"TRACKING_SET": "1", "TRACKING_ENABLED": "true", trackingInstallEnv: value,
		}, recorder)
		options.skipSkills, options.skipMCP, options.skipLogin = true, true, true
		require.NoError(t, runSetup(context.Background(), options))
		assert.Equal(t, []bool{value == "true"}, recorder.tracking)
	}
}

func TestSetupReportsPreservedManagedSkills(t *testing.T) {
	home, external := t.TempDir(), t.TempDir()
	root := filepath.Join(home, ".agents", "skills")
	for _, name := range []string{"blaxel-cli", "blaxel-sdk"} {
		target := filepath.Join(external, name)
		writeTestFile(t, filepath.Join(target, "SKILL.md"), skillManifest(name))
		managedSkillLink(t, target, filepath.Join(root, name))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".pi", "agent"), 0755))
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{"LOGGED_IN": "main", "TRACKING_SET": "1"}, recorder)
	options.skipMCP, options.skipLogin = true, true
	options.installSkills = func(_ context.Context, agents []agentsetup.SkillsAgent) (agentsetup.SkillsInstallResult, error) {
		return agentsetup.InstallSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, agents, time.Now())
	}
	require.NoError(t, runSetup(context.Background(), options))
	text := recorder.text(t)
	assert.Contains(t, text, "kept externally managed: blaxel-cli, blaxel-sdk")
	assert.Contains(t, text, "Kept managed")
	assert.Contains(t, text, "links and contents unchanged")
	assert.Contains(t, text, "Pi")
	assert.NoFileExists(t, agentsetup.SkillsLockPath(home, noEnv))
}

func TestRefreshCapabilityCheckHasNoSetupSideEffects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(setupRefreshEnv, "")
	cmd := SetupCmd()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetArgs([]string{"--refresh-check"})
	require.NoError(t, cmd.Execute())
	assert.Equal(t, setupRefreshCapability+"\n", output.String())
	entries, err := os.ReadDir(home)
	require.NoError(t, err)
	assert.Empty(t, entries)
	assert.True(t, cmd.Flags().Lookup("refresh-check").Hidden)
}

func TestRefreshUpdatesSkillsAndNewAgentsWithoutLogin(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0755))
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{}, recorder)
	archive := buildSkillsArchive(t, testSkillsEntries())
	options.installSkills = func(_ context.Context, agents []agentsetup.SkillsAgent) (agentsetup.SkillsInstallResult, error) {
		return agentsetup.InstallSkillsArchive(archive, home, options.env, agents, time.Now())
	}
	runSetupRefresh(context.Background(), options)
	manifest := filepath.Join(home, ".agents", "skills", "blaxel-cli", "SKILL.md")
	writeTestFile(t, manifest, "outdated skill")
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".codex"), 0755))
	runSetupRefresh(context.Background(), options)
	assert.Equal(t, skillManifest("blaxel-cli"), readTestFile(t, manifest))
	command, args := agentsetup.EntryCommand(agentsetup.MCPTargets["codex"].Entry(options.mcp, "blaxel"))
	assert.Equal(t, testResourceServer.Command[0], command)
	assert.Equal(t, []string{"mcp"}, args)
	assert.NotNil(t, agentsetup.MCPTargets["codex"].Entry(options.mcp, "blaxel-docs"))
	assert.Empty(t, recorder.logins)
	assert.Empty(t, recorder.tracking)
	assert.Len(t, strings.Split(strings.TrimSpace(recorder.text(t)), "\n"), 2, "one summary per refresh")
}

func TestRefreshMigratesOwnedEntriesAndKeepsCustomAndPluginServers(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".cursor", "mcp.json"), `{"mcpServers":{"blaxel":{"url":"https://api.blaxel.ai/v0/mcp"},"other":{"command":"custom"}}}`)
	custom := map[string]any{"command": "bl", "args": []any{"mcp"}, "env": map[string]any{"CUSTOM": "1"}}
	encoded, err := json.Marshal(map[string]any{"mcpServers": map[string]any{"blaxel": custom}})
	require.NoError(t, err)
	writeTestFile(t, filepath.Join(home, ".gemini", "settings.json"), string(encoded))
	plugin := filepath.Join(home, "plugin", "blaxel")
	encoded, err = json.Marshal(map[string]any{"plugins": map[string]any{"blaxel@blaxel": []any{map[string]any{"installPath": plugin}}}})
	require.NoError(t, err)
	writeTestFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), string(encoded))
	pluginConfig := `{"mcpServers":{"blaxel":{"url":"https://api.blaxel.ai/v0/mcp"}}}`
	writeTestFile(t, filepath.Join(plugin, ".mcp.json"), pluginConfig)
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{agentsetup.SkillsInstallEnv: "false"}, recorder)
	runSetupRefresh(context.Background(), options)
	command, args := agentsetup.EntryCommand(agentsetup.MCPTargets["cursor"].Entry(options.mcp, "blaxel"))
	assert.Equal(t, testResourceServer.Command[0], command)
	assert.Equal(t, []string{"mcp"}, args)
	assert.Equal(t, custom, agentsetup.MCPTargets["gemini-cli"].Entry(options.mcp, "blaxel"))
	assert.Nil(t, agentsetup.MCPTargets["claude-code"].Entry(options.mcp, "blaxel"))
	assert.NotNil(t, agentsetup.MCPTargets["claude-code"].Entry(options.mcp, "blaxel-docs"))
	assert.Contains(t, readTestFile(t, filepath.Join(home, ".cursor", "mcp.json")), `"command": "custom"`)
	assert.Equal(t, pluginConfig, readTestFile(t, filepath.Join(plugin, ".mcp.json")))
	assert.Contains(t, recorder.text(t), "1 migrated")
	assert.Contains(t, recorder.text(t), "1 from plugins")
}

func TestRefreshHonorsIndependentOptOutsAndCI(t *testing.T) {
	for _, test := range []struct {
		name        string
		env         map[string]string
		skills, mcp bool
	}{
		{"defaults", nil, true, true},
		{"skills disabled", map[string]string{agentsetup.SkillsInstallEnv: " FALSE "}, false, true},
		{"MCP disabled", map[string]string{mcpInstallEnv: "false"}, true, false},
		{"both disabled", map[string]string{agentsetup.SkillsInstallEnv: "false", mcpInstallEnv: "false"}, false, false},
		{"setup disabled", map[string]string{"BL_INSTALL_SETUP": "false"}, false, false},
		{"CI", map[string]string{"CI": "true"}, false, false},
		{"CI skills forced", map[string]string{"CI": "true", agentsetup.SkillsInstallEnv: "true"}, true, false},
		{"CI MCP forced", map[string]string{"GITHUB_ACTIONS": "true", mcpInstallEnv: "true"}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(home, ".cursor"), 0755))
			recorder := &setupRecorder{}
			options := testSetupOptions(t, home, test.env, recorder)
			runSetupRefresh(context.Background(), options)
			assert.Equal(t, test.skills, len(recorder.skillsAgents) > 0)
			assert.Equal(t, test.mcp, agentsetup.MCPTargets["cursor"].Entry(options.mcp, "blaxel") != nil)
			assert.Empty(t, recorder.logins)
			assert.Empty(t, recorder.tracking)
			assert.Len(t, strings.Split(strings.TrimSpace(recorder.text(t)), "\n"), 1)
		})
	}
}

func TestRefreshKeepsManagedSkillsAndContinuesAfterDownloadFailure(t *testing.T) {
	for _, offline := range []bool{false, true} {
		t.Run(map[bool]string{false: "managed", true: "offline"}[offline], func(t *testing.T) {
			home, external := t.TempDir(), t.TempDir()
			manifest := skillManifest("blaxel-cli") + "Local content.\n"
			writeTestFile(t, filepath.Join(external, "SKILL.md"), manifest)
			managedSkillLink(t, external, filepath.Join(home, ".agents", "skills", "blaxel-cli"))
			require.NoError(t, os.MkdirAll(filepath.Join(home, ".cursor"), 0755))
			env := map[string]string{}
			if offline {
				env["SKILLS_OFFLINE"] = "1"
			}
			recorder := &setupRecorder{}
			options := testSetupOptions(t, home, env, recorder)
			if !offline {
				options.installSkills = func(_ context.Context, agents []agentsetup.SkillsAgent) (agentsetup.SkillsInstallResult, error) {
					return agentsetup.InstallSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, options.env, agents, time.Now())
				}
			}
			runSetupRefresh(context.Background(), options)
			assert.Equal(t, manifest, readTestFile(t, filepath.Join(external, "SKILL.md")))
			assert.NotNil(t, agentsetup.MCPTargets["cursor"].Entry(options.mcp, "blaxel"))
			if offline {
				assert.Contains(t, recorder.text(t), "1 problem; retry with bl setup")
			} else {
				assert.Contains(t, recorder.text(t), "1 externally managed kept")
				assert.NotContains(t, readTestFile(t, agentsetup.SkillsLockPath(home, options.env)), `"blaxel-cli"`)
			}
		})
	}
}

func TestRefreshContinuesAfterAnAgentConfigFailure(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".codex"), 0755))
	writeTestFile(t, filepath.Join(home, ".claude.json"), "broken configuration")
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{agentsetup.SkillsInstallEnv: "false"}, recorder)
	runSetupRefresh(context.Background(), options)
	assert.Equal(t, "broken configuration", readTestFile(t, filepath.Join(home, ".claude.json")))
	assert.NotNil(t, agentsetup.MCPTargets["codex"].Entry(options.mcp, "blaxel"))
	assert.Contains(t, recorder.text(t), "1 problem; retry with bl setup")
	assert.Len(t, strings.Split(strings.TrimSpace(recorder.text(t)), "\n"), 1)
}

func TestSkillsSavedOptOutKeepsAutomaticRefreshMCPIndependent(t *testing.T) {
	home := resolvedTempDir(t)
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".cursor"), 0755))
	enabled := false
	require.NoError(t, agentsetup.WriteSkillsUpdateState(home, agentsetup.SkillsUpdateState{AutoUpdate: &enabled}))
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{agentsetup.SkillsInstallEnv: "true"}, recorder)
	runSetupRefresh(context.Background(), options)
	assert.Empty(t, recorder.skillsAgents, "saved preference wins over automatic installer refresh")
	assert.NotNil(t, agentsetup.MCPTargets["cursor"].Entry(options.mcp, "blaxel"), "MCP refresh remains independent")
	assert.Empty(t, recorder.logins)
	assert.Empty(t, recorder.tracking)
}

func TestRefreshSlowSkillsDownloadLeavesMCPItsOwnDeadline(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0755))
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{"PATH_HAS_claude": ""}, recorder)
	ran := 0
	options.mcp.Run = func(ctx context.Context, _ string, _ ...string) ([]byte, error) {
		ran++
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return []byte("Added HTTP MCP server"), nil
	}
	saved := setupRefreshSkillsTimeout
	setupRefreshSkillsTimeout = 10 * time.Millisecond
	t.Cleanup(func() { setupRefreshSkillsTimeout = saved })
	options.installSkills = func(ctx context.Context, _ []agentsetup.SkillsAgent) (agentsetup.SkillsInstallResult, error) {
		<-ctx.Done()
		return agentsetup.SkillsInstallResult{}, ctx.Err()
	}
	runSetupRefresh(context.Background(), options)
	assert.Positive(t, ran)
	assert.Contains(t, recorder.text(t), "skills unavailable")
	assert.NotContains(t, recorder.text(t), "2 problems")
}

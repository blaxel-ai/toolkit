package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/blaxel-ai/toolkit/cli/ui"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testResourceServer = resourceMCPServer("/opt/blaxel/bin/bl")

// movedBl is an absolute path, on any OS, to a bl that is no longer there.
var movedBl = filepath.Join(os.TempDir(), "old", "bin", "bl")

// testHostedServer is the hosted form setup wrote before bl mcp.
var testHostedServer = mcpServer{name: "blaxel", url: "https://api.blaxel.ai/v0/mcp", plugin: true}
var testDocsServer = mcpServer{name: "blaxel-docs", url: docsMCPURL}

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

func TestUpsertJSONConfigPreservesOrderAndValues(t *testing.T) {
	file := filepath.Join(t.TempDir(), "mcp.json")
	writeTestFile(t, file, `{"zeta": 1, "mcpServers": {"other": {"command": "x", "args": ["b", "a"]}}, "big": 12345678901234567890, "alpha": true}`)

	changed, err := upsertJSONConfig(file, "mcpServers", "blaxel", map[string]string{"url": "https://api.blaxel.ai/v0/mcp?a=1&b=2"}, false)
	require.NoError(t, err)
	assert.True(t, changed)

	assert.Equal(t, `{
  "zeta": 1,
  "mcpServers": {
    "other": {
      "command": "x",
      "args": [
        "b",
        "a"
      ]
    },
    "blaxel": {
      "url": "https://api.blaxel.ai/v0/mcp?a=1&b=2"
    }
  },
  "big": 12345678901234567890,
  "alpha": true
}
`, readTestFile(t, file))
	// Windows has no Unix permissions to keep.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(file)
		require.NoError(t, err)
		assert.Equal(t, os.FileMode(0644), info.Mode().Perm())
	}
}

func TestUpsertJSONConfigCreatesFileAndContainer(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "opencode", "opencode.json")
	changed, err := upsertJSONConfig(file, "mcp", "blaxel", map[string]string{"url": "u"}, false)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "{\n  \"mcp\": {\n    \"blaxel\": {\n      \"url\": \"u\"\n    }\n  }\n}\n", readTestFile(t, file))

	other := filepath.Join(dir, "settings.json")
	writeTestFile(t, other, `{"theme": "dark"}`)
	_, err = upsertJSONConfig(other, "mcpServers", "blaxel", map[string]string{"httpUrl": "u"}, false)
	require.NoError(t, err)
	assert.Equal(t, "{\n  \"theme\": \"dark\",\n  \"mcpServers\": {\n    \"blaxel\": {\n      \"httpUrl\": \"u\"\n    }\n  }\n}\n", readTestFile(t, other))
}

func TestUpsertJSONConfigLeavesExistingServerAlone(t *testing.T) {
	file := filepath.Join(t.TempDir(), "mcp.json")
	original := `{"mcpServers":{"blaxel":{"url":"https://api.blaxel.ai/v0/mcp","headers":{"Authorization":"Bearer KEY"}}}}`
	writeTestFile(t, file, original)
	changed, err := upsertJSONConfig(file, "mcpServers", "blaxel", map[string]string{"url": "other"}, false)
	require.NoError(t, err)
	assert.False(t, changed)
	assert.Equal(t, original, readTestFile(t, file))
}

func TestUpsertJSONConfigRefusesFilesItCannotParse(t *testing.T) {
	for name, content := range map[string]string{
		"comments":      "{\n  // my servers\n  \"mcpServers\": {}\n}",
		"not an object": `["a"]`,
		"bad container": `{"mcpServers": []}`,
		"trailing data": `{} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "settings.json")
			writeTestFile(t, file, content)
			_, err := upsertJSONConfig(file, "mcpServers", "blaxel", map[string]string{"url": "u"}, false)
			assert.Error(t, err)
			assert.Equal(t, content, readTestFile(t, file))
		})
	}
}

func TestWriteConfigFileFollowsSymlinks(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "dotfiles", "mcp.json")
	writeTestFile(t, target, `{}`)
	link := filepath.Join(dir, "home", "mcp.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0755))
	require.NoError(t, os.Symlink(target, link))

	_, err := upsertJSONConfig(link, "mcpServers", "blaxel", map[string]string{"url": "u"}, false)
	require.NoError(t, err)
	info, err := os.Lstat(link)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "the symlink is kept")
	assert.Contains(t, readTestFile(t, target), `"blaxel"`)
}

func TestAppendCodexMCPServer(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.toml")
	original := "# my settings\nmodel = \"gpt-5\"\n\n[mcp_servers.other]\nurl = \"https://example.com/mcp\" # keep\n\n[plugins.\"x@y\"]\nenabled = true"
	writeTestFile(t, file, original)

	changed, err := appendCodexMCPServer(file, testHostedServer)
	require.NoError(t, err)
	assert.True(t, changed)
	content := readTestFile(t, file)
	assert.Equal(t, original+"\n\n[mcp_servers.blaxel]\nurl = \"https://api.blaxel.ai/v0/mcp\"\n", content)

	var config struct {
		MCPServers map[string]struct {
			URL string `toml:"url"`
		} `toml:"mcp_servers"`
	}
	_, err = toml.Decode(content, &config)
	require.NoError(t, err)
	assert.Equal(t, "https://api.blaxel.ai/v0/mcp", config.MCPServers["blaxel"].URL)
	assert.Equal(t, "https://example.com/mcp", config.MCPServers["other"].URL)

	changed, err = appendCodexMCPServer(file, testHostedServer)
	require.NoError(t, err)
	assert.False(t, changed, "an existing server is left alone")
	assert.Equal(t, content, readTestFile(t, file))
}

func TestAppendCodexMCPServerAcceptsValidLayouts(t *testing.T) {
	for name, content := range map[string]string{
		"dotted keys":         "mcp_servers.other.url = \"x\"\n",
		"inline server":       "[mcp_servers]\nother = { url = \"x\" }\n",
		"assignment in table": "[profiles.work]\nmcp_servers = {}\n",
	} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "config.toml")
			writeTestFile(t, file, content)
			changed, err := appendCodexMCPServer(file, testResourceServer)
			require.NoError(t, err)
			assert.True(t, changed)
		})
	}
}

func TestAppendCodexMCPServerCreatesConfig(t *testing.T) {
	file := filepath.Join(t.TempDir(), "codex", "config.toml")
	changed, err := appendCodexMCPServer(file, testDocsServer)
	require.NoError(t, err)
	assert.True(t, changed)
	assert.Equal(t, "[mcp_servers.blaxel-docs]\nurl = \"https://docs.blaxel.ai/mcp\"\n", readTestFile(t, file))
}

func TestAppendCodexMCPServerRefusesConfigsItCannotExtend(t *testing.T) {
	for name, content := range map[string]string{
		"inline table":        "mcp_servers = { other = { url = \"x\" } }\n",
		"quoted inline table": "model = \"x\"\n\"mcp_servers\" = {}\n[tui]\nx = 1\n",
		"invalid":             "model = \n",
	} {
		t.Run(name, func(t *testing.T) {
			file := filepath.Join(t.TempDir(), "config.toml")
			writeTestFile(t, file, content)
			_, err := appendCodexMCPServer(file, testResourceServer)
			assert.Error(t, err)
			assert.Equal(t, content, readTestFile(t, file))
		})
	}
}

type fakeCommand struct {
	name string
	args []string
}

func testMCPEnv(home string, env map[string]string, commands *[]fakeCommand, output string, runErr error) mcpEnv {
	return mcpEnv{
		home: home, config: filepath.Join(home, ".config"),
		env: func(key string) string { return env[key] },
		lookPath: func(name string) (string, error) {
			if _, ok := env["PATH_HAS_"+name]; ok {
				return "/usr/bin/" + name, nil
			}
			return "", errors.New("not found")
		},
		exists: func(path string) bool { return env["MISSING:"+path] == "" },
		run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			*commands = append(*commands, fakeCommand{name, args})
			return []byte(output), runErr
		},
	}
}

func TestClaudeMCPUsesTheClaudeCLI(t *testing.T) {
	home := t.TempDir()
	var commands []fakeCommand
	env := testMCPEnv(home, map[string]string{"PATH_HAS_claude": ""}, &commands, "Added stdio MCP server", nil)

	change, err := addMCPServer(context.Background(), env, mcpTargets["claude-code"], testResourceServer)
	require.NoError(t, err)
	assert.Equal(t, mcpAdded, change)
	require.Len(t, commands, 1)
	assert.Equal(t, "/usr/bin/claude", commands[0].name)
	assert.Equal(t, []string{"mcp", "add", "--scope", "user", "blaxel", "--", "/opt/blaxel/bin/bl", "mcp"}, commands[0].args)

	// Someone else's server with that name is left alone.
	writeTestFile(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{"blaxel":{"type":"http","url":"x"}}}`)
	change, err = addMCPServer(context.Background(), env, mcpTargets["claude-code"], testResourceServer)
	require.NoError(t, err)
	assert.Equal(t, mcpUnchanged, change)
	assert.Len(t, commands, 1)

	// The hosted server an earlier setup added is switched to bl mcp.
	writeTestFile(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{"blaxel":{"type":"http","url":"https://api.blaxel.ai/v0/mcp"}}}`)
	change, err = addMCPServer(context.Background(), env, mcpTargets["claude-code"], testResourceServer)
	require.NoError(t, err)
	assert.Equal(t, mcpReplaced, change)
	require.Len(t, commands, 3)
	assert.Equal(t, []string{"mcp", "remove", "--scope", "user", "blaxel"}, commands[1].args)
	assert.Equal(t, []string{"mcp", "add", "--scope", "user", "blaxel", "--", "/opt/blaxel/bin/bl", "mcp"}, commands[2].args)
}

// claude mcp add writes "env": {}, which is not a customization: a bl that
// moved is repaired.
func TestClaudeMCPRepairsAMovedBl(t *testing.T) {
	home := t.TempDir()
	var commands []fakeCommand
	env := testMCPEnv(home, map[string]string{"PATH_HAS_claude": "", "MISSING:" + movedBl: "1"}, &commands, "", nil)
	writeTestFile(t, filepath.Join(home, ".claude.json"), fmt.Sprintf(`{"mcpServers":{"blaxel":{"type":"stdio","command":%q,"args":["mcp"],"env":{}}}}`, movedBl))
	change, err := addMCPServer(context.Background(), env, mcpTargets["claude-code"], testResourceServer)
	require.NoError(t, err)
	assert.Equal(t, mcpReplaced, change)
	require.Len(t, commands, 2)
	assert.Equal(t, []string{"mcp", "add", "--scope", "user", "blaxel", "--", "/opt/blaxel/bin/bl", "mcp"}, commands[1].args)
}

func TestClaudeMCPWithoutTheCLIEditsClaudeJSON(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, "claude-config")
	var commands []fakeCommand
	env := testMCPEnv(home, map[string]string{"CLAUDE_CONFIG_DIR": configDir}, &commands, "", nil)
	writeTestFile(t, filepath.Join(configDir, ".claude.json"), `{"numStartups": 3}`)

	change, err := addMCPServer(context.Background(), env, mcpTargets["claude-code"], testDocsServer)
	require.NoError(t, err)
	assert.Equal(t, mcpAdded, change)
	assert.Empty(t, commands)
	assert.Equal(t, "{\n  \"numStartups\": 3,\n  \"mcpServers\": {\n    \"blaxel-docs\": {\n      \"type\": \"http\",\n      \"url\": \"https://docs.blaxel.ai/mcp\"\n    }\n  }\n}\n",
		readTestFile(t, filepath.Join(configDir, ".claude.json")))
}

func TestClaudeMCPReportsCLIFailures(t *testing.T) {
	var commands []fakeCommand
	env := testMCPEnv(t.TempDir(), map[string]string{"PATH_HAS_claude": ""}, &commands, "MCP server blaxel already exists in user config", errors.New("exit status 1"))
	change, err := addMCPServer(context.Background(), env, mcpTargets["claude-code"], testResourceServer)
	require.NoError(t, err)
	assert.Equal(t, mcpUnchanged, change)

	env = testMCPEnv(t.TempDir(), map[string]string{"PATH_HAS_claude": ""}, &commands, "boom", errors.New("exit status 2"))
	_, err = addMCPServer(context.Background(), env, mcpTargets["claude-code"], testResourceServer)
	assert.ErrorContains(t, err, "boom")
}

func TestConfigureAgentMCPLeavesInstalledPluginsToThemselves(t *testing.T) {
	home := t.TempDir()
	var commands []fakeCommand
	env := testMCPEnv(home, map[string]string{}, &commands, "", nil)
	writeTestFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), `{"version":2,"plugins":{"blaxel@blaxel":[{"scope":"user"}]}}`)

	result := configureAgentMCP(context.Background(), env, mcpTargets["claude-code"], []mcpServer{testResourceServer, testDocsServer})
	require.NoError(t, result.err)
	assert.Equal(t, []string{"blaxel"}, result.plugin, "a plugin whose files cannot be found is left alone")
	assert.Equal(t, []string{"blaxel-docs"}, result.added)

	writeTestFile(t, filepath.Join(home, ".codex", "config.toml"), "[plugins.\"blaxel@blaxel\"]\nenabled = false\n")
	result = configureAgentMCP(context.Background(), env, mcpTargets["codex"], []mcpServer{testResourceServer})
	require.NoError(t, result.err)
	assert.Equal(t, []string{"blaxel"}, result.added, "a disabled plugin does not provide the server")
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
		installSkills: func(_ context.Context, agents []skillsAgent) (skillsInstallResult, error) {
			targets, names := skillsTargets(agents)
			recorder.skillsAgents = append(recorder.skillsAgents, targets)
			if env["SKILLS_OFFLINE"] != "" {
				return skillsInstallResult{}, errors.New("downloading the skills: no network")
			}
			return skillsInstallResult{skills: []string{"blaxel-cli", "blaxel-sdk"}, agents: names}, nil
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
	require.NoError(t, runSetup(context.Background(), testSetupOptions(t, home, map[string]string{mcpInstallEnv: "false", skillsInstallEnv: "false"}, recorder)))
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

func TestMCPAgentResultText(t *testing.T) {
	assert.Equal(t, "added blaxel and blaxel-docs", mcpAgentResult{added: []string{"blaxel", "blaxel-docs"}}.short())
	assert.Equal(t, "added blaxel-docs · blaxel from the Blaxel plugin",
		mcpAgentResult{added: []string{"blaxel-docs"}, plugin: []string{"blaxel"}}.short())
	assert.Equal(t, "blaxel and blaxel-docs already set up", mcpAgentResult{existing: []string{"blaxel", "blaxel-docs"}}.short())
	assert.Equal(t, []string{"docs MCP", "Blaxel MCP (Blaxel plugin)"}, mcpAgentResult{added: []string{"blaxel-docs"}, plugin: []string{"blaxel"}}.servers())
	assert.Equal(t, filepath.Join("~", ".codex", "config.toml"), displayHomePath("/h", "/h/.codex/config.toml"))
	assert.Equal(t, "/etc/x", displayHomePath("/h", "/etc/x"))
}

func TestIsSkillsCommandIncludesSetup(t *testing.T) {
	assert.True(t, isSkillsCommand([]string{"setup", "--yes"}))
	assert.False(t, isSkillsCommand([]string{"get", "setup"}))
	assert.Equal(t, []string{"/x/bl", "mcp"}, resourceMCPServer("/x/bl").command)
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
	options.mcp.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
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

func TestClaudeMCPLeavesFilesItCannotReadToTheCLI(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".claude.json"), "// settings\n{\"numStartups\": 3}\n")
	var commands []fakeCommand
	env := testMCPEnv(home, map[string]string{"PATH_HAS_claude": ""}, &commands, "Added HTTP MCP server", nil)
	change, err := addMCPServer(context.Background(), env, mcpTargets["claude-code"], testResourceServer)
	require.NoError(t, err)
	assert.Equal(t, mcpAdded, change)
	assert.Len(t, commands, 1, "the claude CLI applies the server")

	// Without the CLI, the file is not rewritten.
	env = testMCPEnv(home, map[string]string{}, &commands, "", nil)
	_, err = addMCPServer(context.Background(), env, mcpTargets["claude-code"], testResourceServer)
	assert.Error(t, err)
	assert.Equal(t, "// settings\n{\"numStartups\": 3}\n", readTestFile(t, filepath.Join(home, ".claude.json")))
}

func TestClassifyMCPEntry(t *testing.T) {
	env := testMCPEnv(t.TempDir(), map[string]string{"MISSING:" + movedBl: "1"}, &[]fakeCommand{}, "", nil)
	for name, test := range map[string]struct {
		entry map[string]any
		want  mcpEntryState
	}{
		"absent":                {nil, mcpEntryAbsent},
		"bl mcp":                {map[string]any{"command": "/opt/blaxel/bin/bl", "args": []any{"mcp"}}, mcpEntryCurrent},
		"another bl":            {map[string]any{"command": "/usr/local/bin/blaxel", "args": []any{"mcp", "-w", "x"}}, mcpEntryCustom},
		"bare bl":               {map[string]any{"type": "stdio", "command": "bl", "args": []any{"mcp"}}, mcpEntryCurrent},
		"Windows bl":            {map[string]any{"command": `C:\Users\me\bin\bl.exe`, "args": []any{"mcp"}}, mcpEntryCurrent},
		"OpenCode list":         {map[string]any{"type": "local", "command": []any{"/opt/blaxel/bin/bl", "mcp"}, "enabled": true}, mcpEntryCurrent},
		"bl that moved":         {map[string]any{"command": movedBl, "args": []any{"mcp"}}, mcpEntryOutdated},
		"moved, empty env":      {map[string]any{"type": "stdio", "command": movedBl, "args": []any{"mcp"}, "env": map[string]any{}}, mcpEntryOutdated},
		"moved, custom env":     {map[string]any{"type": "stdio", "command": movedBl, "args": []any{"mcp"}, "env": map[string]any{"BL_WORKSPACE": "x"}}, mcpEntryCustom},
		"hosted, Claude":        {map[string]any{"type": "http", "url": "https://api.blaxel.ai/v0/mcp"}, mcpEntryOutdated},
		"hosted, Gemini":        {map[string]any{"httpUrl": "https://api.blaxel.ai/v0/mcp"}, mcpEntryOutdated},
		"hosted, OpenCode":      {map[string]any{"type": "remote", "url": "https://api.blaxel.dev/v0/mcp", "enabled": true}, mcpEntryOutdated},
		"hosted with a header":  {map[string]any{"url": "https://api.blaxel.ai/v0/mcp", "headers": map[string]any{"X": "y"}}, mcpEntryCustom},
		"another URL":           {map[string]any{"url": "https://example.com/mcp"}, mcpEntryCustom},
		"another command":       {map[string]any{"command": "npx", "args": []any{"mcp-remote", "https://api.blaxel.ai/v0/mcp"}}, mcpEntryCustom},
		"bl, another command":   {map[string]any{"command": "bl", "args": []any{"get", "sandboxes"}}, mcpEntryCustom},
		"empty":                 {map[string]any{}, mcpEntryCustom},
		"only a type":           {map[string]any{"type": "http"}, mcpEntryCustom},
		"hosted, trailing path": {map[string]any{"serverUrl": "https://api.blaxel.ai/v0/mcp/"}, mcpEntryOutdated},
	} {
		assert.Equal(t, test.want, classifyMCPEntry(env, testResourceServer, test.entry), name)
	}
	assert.Equal(t, mcpEntryCurrent, classifyMCPEntry(env, testDocsServer, map[string]any{"url": "anything"}), "a docs entry is the user's choice")
}

func TestReplaceCodexMCPServerKeepsTheRestOfTheFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.toml")
	original := "model = \"gpt-5\"\n\n[mcp_servers.blaxel]\nurl = \"https://api.blaxel.ai/v0/mcp\"\n\n[mcp_servers.blaxel-docs] # docs\nurl = \"https://docs.blaxel.ai/mcp\"\n"
	writeTestFile(t, file, original)
	require.NoError(t, replaceCodexMCPServer(file, resourceMCPServer(`C:\Program Files\bl.exe`)))
	assert.Equal(t, "model = \"gpt-5\"\n\n[mcp_servers.blaxel]\ncommand = \"C:\\\\Program Files\\\\bl.exe\"\nargs = [\"mcp\"]\n\n[mcp_servers.blaxel-docs] # docs\nurl = \"https://docs.blaxel.ai/mcp\"\n",
		readTestFile(t, file))
	var config struct {
		MCPServers map[string]struct {
			Command string   `toml:"command"`
			Args    []string `toml:"args"`
			URL     string   `toml:"url"`
		} `toml:"mcp_servers"`
	}
	_, err := toml.DecodeFile(file, &config)
	require.NoError(t, err)
	assert.Equal(t, `C:\Program Files\bl.exe`, config.MCPServers["blaxel"].Command, "backslashes survive")
	assert.Equal(t, []string{"mcp"}, config.MCPServers["blaxel"].Args)
	assert.Equal(t, "https://docs.blaxel.ai/mcp", config.MCPServers["blaxel-docs"].URL)

	// The last table of a file, quoted.
	writeTestFile(t, file, "[mcp_servers.\"blaxel\"]\nurl = \"https://api.blaxel.ai/v0/mcp\"")
	require.NoError(t, replaceCodexMCPServer(file, testResourceServer))
	assert.Equal(t, "[mcp_servers.blaxel]\ncommand = \"/opt/blaxel/bin/bl\"\nargs = [\"mcp\"]\n", readTestFile(t, file))
}

// Agents set up by v0.1.119 have the hosted server, with its own sign-in:
// the next setup switches them to bl mcp and leaves other entries alone.
func TestRunSetupSwitchesHostedServersToBlMCP(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".cursor", "mcp.json"), `{"mcpServers":{"other":{"url":"https://example.com"},"blaxel":{"url":"https://api.blaxel.ai/v0/mcp"},"blaxel-docs":{"url":"https://docs.blaxel.ai/mcp"}}}`)
	writeTestFile(t, filepath.Join(home, ".codex", "config.toml"), "[mcp_servers.blaxel]\nurl = \"https://api.blaxel.ai/v0/mcp\"\n\n[mcp_servers.blaxel-docs]\nurl = \"https://docs.blaxel.ai/mcp\"\n")
	writeTestFile(t, filepath.Join(home, ".gemini", "settings.json"), `{"mcpServers":{"blaxel":{"httpUrl":"https://api.blaxel.ai/v0/mcp","headers":{"Authorization":"Bearer mine"}}}}`)
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{"LOGGED_IN": "main", "TRACKING_SET": "1", skillsInstallEnv: "false"}, recorder)
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
	require.NoError(t, runSetup(context.Background(), testSetupOptions(t, home, map[string]string{"LOGGED_IN": "main", "TRACKING_SET": "1", skillsInstallEnv: "false"}, recorder)))
	assert.Equal(t, before, readTestFile(t, filepath.Join(home, ".cursor", "mcp.json")))
}

func TestRunSetupAddsBlMCPToClaudeDesktop(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{"APPDATA": filepath.Join(home, "AppData", "Roaming"), "XDG_CONFIG_HOME": filepath.Join(home, ".config")}
	dir := claudeDesktopDir(newSkillsAgentPaths(home, func(key string) string { return env[key] }))
	writeTestFile(t, filepath.Join(dir, "claude_desktop_config.json"), `{"preferences":{"theme":"dark"}}`)
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, env, recorder)
	options.mcp.config = env["XDG_CONFIG_HOME"]
	plan, err := newSetupPlan(options)
	require.NoError(t, err)
	require.Len(t, plan.agents, 1)
	assert.Equal(t, "claude-desktop", plan.agents[0].id)
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

func TestBlCommandPath(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	keg := filepath.Join(dir, "Cellar", "blaxel", "0.1.120", "bin", "blaxel")
	writeTestFile(t, keg, "")
	path, err := blCommandPath(func() (string, error) { return keg, nil })
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "bin", "bl"), path)
	plain := filepath.Join(dir, ".local", "bin", "bl")
	writeTestFile(t, plain, "")
	path, err = blCommandPath(func() (string, error) { return plain, nil })
	require.NoError(t, err)
	assert.Equal(t, plain, path)
	path, err = blCommandPath(func() (string, error) { return "", errors.New("unknown") })
	assert.Empty(t, path)
	assert.ErrorContains(t, err, "cannot locate")
}

package agentsetup

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testResourceServer = ResourceMCPServer("/opt/blaxel/bin/bl")

var testDocsServer = MCPServer{Name: "blaxel-docs", URL: DocsMCPURL}

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

type fakeCommand struct {
	name string
	args []string
}

func testMCPEnv(home string, env map[string]string, commands *[]fakeCommand, output string, runErr error) MCPEnv {
	return MCPEnv{
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

// Setup replaces the hosted server it added before bl mcp, and leaves every
// customized or disabled entry as it is.
func TestMCPMigrationPreservesCustomAndDisabledEntries(t *testing.T) {
	for _, fixture := range []struct{ name, target, config string }{
		{"missing custom command", "cursor", `{"mcpServers":{"blaxel":{"command":"/old/bin/bl","args":["mcp","--workspace","production"],"env":{"BL_ENV":"dev"}}}}`},
		{"disabled hosted", "opencode", `{"mcp":{"blaxel":{"type":"remote","url":"https://api.blaxel.ai/v0/mcp","enabled":false}}}`},
		{"disabled local", "opencode", `{"mcp":{"blaxel":{"type":"local","command":["/old/bin/bl","mcp"],"enabled":false}}}`},
		{"unknown local key", "cursor", `{"mcpServers":{"blaxel":{"command":"/old/bin/bl","args":["mcp"],"futureSetting":{"x":1}}}}`},
		{"hosted headers", "cursor", `{"mcpServers":{"blaxel":{"url":"https://api.blaxel.ai/v0/mcp","headers":{"X-Blaxel-Workspace":"production"}}}}`},
		{"hosted query", "cursor", `{"mcpServers":{"blaxel":{"url":"https://api.blaxel.ai/v0/mcp?workspace=production"}}}`},
		{"TOML nested table", "codex", "[mcp_servers.blaxel]\ncommand = \"/old/bin/bl\"\nargs = [\"mcp\"]\n[mcp_servers.blaxel.env]\nBL_ENV = \"dev\"\n\n[mcp_servers.other]\nurl = \"https://example.com\"\n"},
		{"TOML args", "codex", "[mcp_servers.blaxel]\ncommand = \"C:\\\\old\\\\bl.exe\"\nargs = [\"mcp\", \"--workspace\", \"production\"]\n"},
		{"TOML inline hosted entry", "codex", "[mcp_servers]\nblaxel = { url = \"https://api.blaxel.ai/v0/mcp\" }\n"},
		{"TOML disabled", "codex", "[mcp_servers.blaxel]\nurl = \"https://api.blaxel.ai/v0/mcp\"\nenabled = false\n"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var commands []fakeCommand
			e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
			e.Exists = func(string) bool { return false }
			target := MCPTargets[fixture.target]
			file := target.File(e)
			writeTestFile(t, file, fixture.config)
			change, err := addMCPServer(context.Background(), e, target, ResourceMCPServer("/new/bin/bl"))
			require.NoError(t, err)
			assert.Equal(t, mcpUnchanged, change)
			assert.Equal(t, fixture.config, readTestFile(t, file))
		})
	}
}

func TestMCPMigrationLeavesJSONCAlone(t *testing.T) {
	for _, agent := range []string{"cursor", "opencode", "claude-code"} {
		t.Run(agent, func(t *testing.T) {
			var commands []fakeCommand
			e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
			target := MCPTargets[agent]
			file := target.File(e)
			original := "{\n// user's pinned transport\n\"mcpServers\": {\"blaxel\": {\"command\": \"/old/bin/bl\", \"args\": [\"mcp\"], \"env\": {\"BL_WORKSPACE\": \"production\"}}}\n}"
			writeTestFile(t, file, original)
			_, err := addMCPServer(context.Background(), e, target, testResourceServer)
			require.Error(t, err)
			assert.Equal(t, original, readTestFile(t, file))
		})
	}
}

func TestMCPClaudeReplacementRollsBackOnFailure(t *testing.T) {
	home := t.TempDir()
	var commands []fakeCommand
	e := testMCPEnv(home, map[string]string{"PATH_HAS_claude": ""}, &commands, "", nil)
	file := claudeConfigFile(e)
	old := `{"type":"http","url":"https://api.blaxel.ai/v0/mcp"}`
	writeTestFile(t, file, `{"mcpServers":{"other":{"command":"mine"},"blaxel":`+old+`},"theme":"dark"}`)
	e.Run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[1] == "remove" {
			require.NoError(t, writeConfigFile(file, []byte(`{"mcpServers":{"other":{"command":"mine"}},"theme":"dark"}`)))
			return nil, nil
		}
		return []byte("add failed"), errors.New("CLI failure")
	}
	_, err := addMCPServer(context.Background(), e, MCPTargets["claude-code"], testResourceServer)
	require.ErrorContains(t, err, "add failed")
	entry := jsonConfigEntry(file, "mcpServers", "blaxel")
	assert.Equal(t, "https://api.blaxel.ai/v0/mcp", entry["url"])
	assert.Equal(t, "mine", jsonConfigEntry(file, "mcpServers", "other")["command"])
	assert.Contains(t, readTestFile(t, file), `"theme": "dark"`)
}

// movedBl is an absolute path, on any OS, to a bl that is no longer there.
var movedBl = filepath.Join(os.TempDir(), "old", "bin", "bl")

// testHostedServer is the hosted form setup wrote before bl mcp.
var testHostedServer = MCPServer{Name: "blaxel", URL: "https://api.blaxel.ai/v0/mcp", Plugin: true}

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

func TestClaudeMCPUsesTheClaudeCLI(t *testing.T) {
	home := t.TempDir()
	var commands []fakeCommand
	env := testMCPEnv(home, map[string]string{"PATH_HAS_claude": ""}, &commands, "Added stdio MCP server", nil)

	change, err := addMCPServer(context.Background(), env, MCPTargets["claude-code"], testResourceServer)
	require.NoError(t, err)
	assert.Equal(t, mcpAdded, change)
	require.Len(t, commands, 1)
	assert.Equal(t, "/usr/bin/claude", commands[0].name)
	assert.Equal(t, []string{"mcp", "add", "--scope", "user", "blaxel", "--", "/opt/blaxel/bin/bl", "mcp"}, commands[0].args)

	// Someone else's server with that name is left alone.
	writeTestFile(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{"blaxel":{"type":"http","url":"x"}}}`)
	change, err = addMCPServer(context.Background(), env, MCPTargets["claude-code"], testResourceServer)
	require.NoError(t, err)
	assert.Equal(t, mcpUnchanged, change)
	assert.Len(t, commands, 1)

	// The hosted server an earlier setup added is switched to bl mcp.
	writeTestFile(t, filepath.Join(home, ".claude.json"), `{"mcpServers":{"blaxel":{"type":"http","url":"https://api.blaxel.ai/v0/mcp"}}}`)
	change, err = addMCPServer(context.Background(), env, MCPTargets["claude-code"], testResourceServer)
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
	change, err := addMCPServer(context.Background(), env, MCPTargets["claude-code"], testResourceServer)
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

	change, err := addMCPServer(context.Background(), env, MCPTargets["claude-code"], testDocsServer)
	require.NoError(t, err)
	assert.Equal(t, mcpAdded, change)
	assert.Empty(t, commands)
	assert.Equal(t, "{\n  \"numStartups\": 3,\n  \"mcpServers\": {\n    \"blaxel-docs\": {\n      \"type\": \"http\",\n      \"url\": \"https://docs.blaxel.ai/mcp\"\n    }\n  }\n}\n",
		readTestFile(t, filepath.Join(configDir, ".claude.json")))
}

// CLAUDE_CONFIG_DIR can come from an untrusted place, so a claude binary under
// it is never run; the config file is edited instead.
func TestClaudeMCPNeverRunsAClaudeUnderClaudeConfigDir(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, "repo", "claude-config")
	var commands []fakeCommand
	env := testMCPEnv(home, map[string]string{"CLAUDE_CONFIG_DIR": configDir}, &commands, "", nil)
	writeTestFile(t, filepath.Join(configDir, "local", "claude"), "#!/bin/sh\n")

	change, err := addMCPServer(context.Background(), env, MCPTargets["claude-code"], testDocsServer)
	require.NoError(t, err)
	assert.Equal(t, mcpAdded, change)
	assert.Empty(t, commands)
	assert.Contains(t, readTestFile(t, filepath.Join(configDir, ".claude.json")), `"blaxel-docs"`)
}

func TestClaudeMCPReportsCLIFailures(t *testing.T) {
	var commands []fakeCommand
	env := testMCPEnv(t.TempDir(), map[string]string{"PATH_HAS_claude": ""}, &commands, "MCP server blaxel already exists in user config", errors.New("exit status 1"))
	change, err := addMCPServer(context.Background(), env, MCPTargets["claude-code"], testResourceServer)
	require.NoError(t, err)
	assert.Equal(t, mcpUnchanged, change)

	env = testMCPEnv(t.TempDir(), map[string]string{"PATH_HAS_claude": ""}, &commands, "boom", errors.New("exit status 2"))
	_, err = addMCPServer(context.Background(), env, MCPTargets["claude-code"], testResourceServer)
	assert.ErrorContains(t, err, "boom")
}

func TestConfigureAgentMCPLeavesInstalledPluginsToThemselves(t *testing.T) {
	home := t.TempDir()
	var commands []fakeCommand
	env := testMCPEnv(home, map[string]string{}, &commands, "", nil)
	writeTestFile(t, filepath.Join(home, ".claude", "plugins", "installed_plugins.json"), `{"version":2,"plugins":{"blaxel@blaxel":[{"scope":"user"}]}}`)

	result := ConfigureAgentMCP(context.Background(), env, MCPTargets["claude-code"], []MCPServer{testResourceServer, testDocsServer})
	require.NoError(t, result.Err)
	assert.Equal(t, []string{"blaxel"}, result.Plugin, "a plugin whose files cannot be found is left alone")
	assert.Equal(t, []string{"blaxel-docs"}, result.Added)

	writeTestFile(t, filepath.Join(home, ".codex", "config.toml"), "[plugins.\"blaxel@blaxel\"]\nenabled = false\n")
	result = ConfigureAgentMCP(context.Background(), env, MCPTargets["codex"], []MCPServer{testResourceServer})
	require.NoError(t, result.Err)
	assert.Equal(t, []string{"blaxel"}, result.Added, "a disabled plugin does not provide the server")
}

func TestClaudeMCPLeavesFilesItCannotReadToTheCLI(t *testing.T) {
	home := t.TempDir()
	writeTestFile(t, filepath.Join(home, ".claude.json"), "// settings\n{\"numStartups\": 3}\n")
	var commands []fakeCommand
	env := testMCPEnv(home, map[string]string{"PATH_HAS_claude": ""}, &commands, "Added HTTP MCP server", nil)
	change, err := addMCPServer(context.Background(), env, MCPTargets["claude-code"], testResourceServer)
	require.NoError(t, err)
	assert.Equal(t, mcpAdded, change)
	assert.Len(t, commands, 1, "the claude CLI applies the server")

	// Without the CLI, the file is not rewritten.
	env = testMCPEnv(home, map[string]string{}, &commands, "", nil)
	_, err = addMCPServer(context.Background(), env, MCPTargets["claude-code"], testResourceServer)
	assert.Error(t, err)
	assert.Equal(t, "// settings\n{\"numStartups\": 3}\n", readTestFile(t, filepath.Join(home, ".claude.json")))
}

func TestClassifyMCPEntry(t *testing.T) {
	env := testMCPEnv(t.TempDir(), map[string]string{"MISSING:" + movedBl: "1"}, &[]fakeCommand{}, "", nil)
	for name, test := range map[string]struct {
		entry map[string]any
		want  MCPEntryState
	}{
		"absent":                {nil, mcpEntryAbsent},
		"bl mcp":                {map[string]any{"command": "/opt/blaxel/bin/bl", "args": []any{"mcp"}}, MCPEntryCurrent},
		"another bl":            {map[string]any{"command": "/usr/local/bin/blaxel", "args": []any{"mcp", "-w", "x"}}, MCPEntryCustom},
		"bare bl":               {map[string]any{"type": "stdio", "command": "bl", "args": []any{"mcp"}}, MCPEntryCurrent},
		"Windows bl":            {map[string]any{"command": `C:\Users\me\bin\bl.exe`, "args": []any{"mcp"}}, MCPEntryCurrent},
		"OpenCode list":         {map[string]any{"type": "local", "command": []any{"/opt/blaxel/bin/bl", "mcp"}, "enabled": true}, MCPEntryCurrent},
		"bl that moved":         {map[string]any{"command": movedBl, "args": []any{"mcp"}}, MCPEntryOutdated},
		"moved, empty env":      {map[string]any{"type": "stdio", "command": movedBl, "args": []any{"mcp"}, "env": map[string]any{}}, MCPEntryOutdated},
		"moved, custom env":     {map[string]any{"type": "stdio", "command": movedBl, "args": []any{"mcp"}, "env": map[string]any{"BL_WORKSPACE": "x"}}, MCPEntryCustom},
		"hosted, Claude":        {map[string]any{"type": "http", "url": "https://api.blaxel.ai/v0/mcp"}, MCPEntryOutdated},
		"hosted, Gemini":        {map[string]any{"httpUrl": "https://api.blaxel.ai/v0/mcp"}, MCPEntryOutdated},
		"hosted, OpenCode":      {map[string]any{"type": "remote", "url": "https://api.blaxel.dev/v0/mcp", "enabled": true}, MCPEntryOutdated},
		"hosted with a header":  {map[string]any{"url": "https://api.blaxel.ai/v0/mcp", "headers": map[string]any{"X": "y"}}, MCPEntryCustom},
		"another URL":           {map[string]any{"url": "https://example.com/mcp"}, MCPEntryCustom},
		"another command":       {map[string]any{"command": "npx", "args": []any{"mcp-remote", "https://api.blaxel.ai/v0/mcp"}}, MCPEntryCustom},
		"bl, another command":   {map[string]any{"command": "bl", "args": []any{"get", "sandboxes"}}, MCPEntryCustom},
		"empty":                 {map[string]any{}, MCPEntryCustom},
		"only a type":           {map[string]any{"type": "http"}, MCPEntryCustom},
		"hosted, trailing path": {map[string]any{"serverUrl": "https://api.blaxel.ai/v0/mcp/"}, MCPEntryOutdated},
	} {
		assert.Equal(t, test.want, ClassifyMCPEntry(env, testResourceServer, test.entry), name)
	}
	assert.Equal(t, MCPEntryCurrent, ClassifyMCPEntry(env, testDocsServer, map[string]any{"url": "anything"}), "a docs entry is the user's choice")
}

func TestReplaceCodexMCPServerKeepsTheRestOfTheFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "config.toml")
	original := "model = \"gpt-5\"\n\n[mcp_servers.blaxel]\nurl = \"https://api.blaxel.ai/v0/mcp\"\n\n[mcp_servers.blaxel-docs] # docs\nurl = \"https://docs.blaxel.ai/mcp\"\n"
	writeTestFile(t, file, original)
	require.NoError(t, replaceCodexMCPServer(file, ResourceMCPServer(`C:\Program Files\bl.exe`)))
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

func TestMCPAgentResultText(t *testing.T) {
	assert.Equal(t, "added blaxel and blaxel-docs", MCPAgentResult{Added: []string{"blaxel", "blaxel-docs"}}.Short())
	assert.Equal(t, "added blaxel-docs · blaxel from the Blaxel plugin",
		MCPAgentResult{Added: []string{"blaxel-docs"}, Plugin: []string{"blaxel"}}.Short())
	assert.Equal(t, "blaxel and blaxel-docs already set up", MCPAgentResult{Existing: []string{"blaxel", "blaxel-docs"}}.Short())
	assert.Equal(t, []string{"docs MCP", "Blaxel MCP (Blaxel plugin)"}, MCPAgentResult{Added: []string{"blaxel-docs"}, Plugin: []string{"blaxel"}}.Servers())
}

func TestResourceMCPServerRunsBlMCP(t *testing.T) {
	assert.Equal(t, []string{"/x/bl", "mcp"}, ResourceMCPServer("/x/bl").Command)
}

func TestBlCommandPath(t *testing.T) {
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	keg := filepath.Join(dir, "Cellar", "blaxel", "0.1.120", "bin", "blaxel")
	writeTestFile(t, keg, "")
	path, err := BlCommandPath(func() (string, error) { return keg, nil })
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "bin", "bl"), path)
	plain := filepath.Join(dir, ".local", "bin", "bl")
	writeTestFile(t, plain, "")
	path, err = BlCommandPath(func() (string, error) { return plain, nil })
	require.NoError(t, err)
	assert.Equal(t, plain, path)
	path, err = BlCommandPath(func() (string, error) { return "", errors.New("unknown") })
	assert.Empty(t, path)
	assert.ErrorContains(t, err, "cannot locate")
}

func TestHomebrewSkillsLocation(t *testing.T) {
	for _, tt := range []struct{ name, path, prefix, version string }{
		{"apple silicon", "/opt/homebrew/Cellar/blaxel/1.2.3/bin/blaxel", "/opt/homebrew", "1.2.3"},
		{"intel alias", "/usr/local/Cellar/blaxel/1.2.3_1/bin/bl", "/usr/local", "1.2.3_1"},
		{"linux", "/home/linuxbrew/.linuxbrew/Cellar/blaxel/1.2.3/bin/blaxel", "/home/linuxbrew/.linuxbrew", "1.2.3"},
		{"custom prefix", "/tmp/custom brew/Cellar/blaxel/1.2.3/bin/blaxel", "/tmp/custom brew", "1.2.3"},
		{"curl", "/home/user/.local/bin/blaxel", "", ""},
		{"other formula", "/opt/homebrew/Cellar/other/1.2.3/bin/blaxel", "", ""},
		{"lookalike formula", "/opt/homebrew/Cellar/blaxel-extra/1.2.3/bin/blaxel", "", ""},
		{"lookalike cellar", "/opt/homebrew/NotCellar/blaxel/1.2.3/bin/blaxel", "", ""},
		{"wrong directory", "/opt/homebrew/Cellar/blaxel/1.2.3/lib/blaxel", "", ""},
		{"wrong binary", "/opt/homebrew/Cellar/blaxel/1.2.3/bin/other", "", ""},
		{"relative", "Cellar/blaxel/1.2.3/bin/blaxel", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path, expectedPrefix := tt.path, tt.prefix
			if runtime.GOOS == "windows" && filepath.IsAbs(filepath.FromSlash("C:"+path)) {
				path = filepath.FromSlash("C:" + path)
				if expectedPrefix != "" {
					expectedPrefix = filepath.FromSlash("C:" + expectedPrefix)
				}
			}
			prefix, version := HomebrewSkillsLocation(path)
			require.Equal(t, expectedPrefix, prefix)
			require.Equal(t, tt.version, version)
		})
	}
}

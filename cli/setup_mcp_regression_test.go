package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Regression versions of both setup findings, plus the reviewed config shapes.
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
		{"TOML disabled", "codex", "[mcp_servers.blaxel]\nurl = \"https://api.blaxel.ai/v0/mcp\"\nenabled = false\n"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var commands []fakeCommand
			e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
			e.exists = func(string) bool { return false }
			target := mcpTargets[fixture.target]
			file := target.file(e)
			writeTestFile(t, file, fixture.config)
			change, err := addMCPServer(context.Background(), e, target, resourceMCPServer("/new/bin/bl"))
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
			target := mcpTargets[agent]
			file := target.file(e)
			original := "{\n// user's pinned transport\n\"mcpServers\": {\"blaxel\": {\"command\": \"/old/bin/bl\", \"args\": [\"mcp\"], \"env\": {\"BL_WORKSPACE\": \"production\"}}}\n}"
			writeTestFile(t, file, original)
			_, err := addMCPServer(context.Background(), e, target, testResourceServer)
			require.Error(t, err)
			assert.Equal(t, original, readTestFile(t, file))
		})
	}
}

func TestMCPWindowsPathRoundTripAndMinimalRepair(t *testing.T) {
	server := resourceMCPServer(`C:\Program Files\Blaxel\bl.exe`)
	table, err := codexServerTable(server)
	require.NoError(t, err)
	var config struct {
		MCPServers map[string]struct {
			Command string
			Args    []string
		} `toml:"mcp_servers"`
	}
	_, err = toml.Decode(table, &config)
	require.NoError(t, err)
	assert.Equal(t, server.command[0], config.MCPServers["blaxel"].Command)
	data, err := json.Marshal(commandOrURL("url")(server))
	require.NoError(t, err)
	var entry map[string]any
	require.NoError(t, json.Unmarshal(data, &entry))
	command, args := entryCommand(entry)
	assert.Equal(t, server.command[0], command)
	assert.Equal(t, []string{"mcp"}, args)
	e := mcpEnv{exists: func(string) bool { return false }}
	assert.Equal(t, mcpEntryOutdated, classifyMCPEntry(e, server, entry))
	entry["env"] = map[string]any{"BL_WORKSPACE": "production"}
	assert.Equal(t, mcpEntryCustom, classifyMCPEntry(e, server, entry))
}

func TestMCPClaudeReplacementRollsBackOnFailure(t *testing.T) {
	home := t.TempDir()
	var commands []fakeCommand
	e := testMCPEnv(home, map[string]string{"PATH_HAS_claude": ""}, &commands, "", nil)
	file := claudeConfigFile(e)
	old := `{"type":"http","url":"https://api.blaxel.ai/v0/mcp"}`
	writeTestFile(t, file, `{"mcpServers":{"other":{"command":"mine"},"blaxel":`+old+`},"theme":"dark"}`)
	e.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[1] == "remove" {
			require.NoError(t, writeConfigFile(file, []byte(`{"mcpServers":{"other":{"command":"mine"}},"theme":"dark"}`)))
			return nil, nil
		}
		return []byte("add failed"), errors.New("CLI failure")
	}
	_, err := addMCPServer(context.Background(), e, mcpTargets["claude-code"], testResourceServer)
	require.ErrorContains(t, err, "add failed")
	entry := jsonConfigEntry(file, "mcpServers", "blaxel")
	assert.Equal(t, "https://api.blaxel.ai/v0/mcp", entry["url"])
	assert.Equal(t, "mine", jsonConfigEntry(file, "mcpServers", "other")["command"])
	assert.Contains(t, readTestFile(t, file), `"theme": "dark"`)
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
			run := func() {
				options := testSetupOptions(t, home, map[string]string{"LOGGED_IN": "main", "TRACKING_SET": "1", skillsInstallEnv: "false"}, &setupRecorder{})
				options.agents = []string{agent}
				require.NoError(t, runSetup(context.Background(), options))
			}
			run()
			run()
			target := mcpTargets[agent]
			e := testMCPEnv(home, map[string]string{}, &[]fakeCommand{}, "", nil)
			assert.Nil(t, target.entry(e, "blaxel"), "the plugin's server is the only Blaxel server")
			assert.NotNil(t, target.entry(e, "blaxel-docs"))
			assert.Equal(t, hosted, readTestFile(t, filepath.Join(root, ".mcp.json")), "plugin files are never edited")

			// A plugin that no longer bundles the server leaves the job to setup.
			require.NoError(t, os.Remove(filepath.Join(root, ".mcp.json")))
			run()
			command, args := entryCommand(target.entry(e, "blaxel"))
			assert.Equal(t, "/opt/blaxel/bin/bl", command)
			assert.Equal(t, []string{"mcp"}, args)
		})
	}
}

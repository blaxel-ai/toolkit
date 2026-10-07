package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

func TestAdditionalAgentMCPMerges(t *testing.T) {
	for _, agent := range []struct{ id, container, urlKey, localType, remoteType string }{
		{"github-copilot", "mcpServers", "url", "local", "http"},
		{"vscode", "servers", "url", "stdio", "http"},
		{"amp", "amp.mcpServers", "url", "", ""},
		{"goose", "extensions", "uri", "stdio", "streamable_http"},
		{"kiro-cli", "mcpServers", "url", "", ""},
		{"qwen-code", "mcpServers", "httpUrl", "", ""},
		{"cline", "mcpServers", "url", "", "streamableHttp"},
		{"continue", "mcpServers", "url", "stdio", "http"},
		{"junie", "mcpServers", "url", "", ""},
		{"augment", "mcpServers", "url", "", "http"},
		{"openhands", "mcpServers", "url", "", ""},
		{"crush", "mcp", "url", "stdio", "http"},
		{"openclaw", "servers", "url", "stdio", "streamable-http"},
	} {
		t.Run(agent.id, func(t *testing.T) {
			oldCommand := filepath.Join(t.TempDir(), "old", "bl")
			encode := func(entry map[string]any) string {
				servers := map[string]any{"other": map[string]any{"command": "my-server", "setting": "keep"}}
				if entry != nil {
					servers["blaxel"] = entry
				}
				config := map[string]any{"theme": "dark", agent.container: servers}
				if agent.id == "openclaw" {
					config = map[string]any{"theme": "dark", "mcp": map[string]any{"servers": servers, "setting": "keep"}}
				}
				var data []byte
				var err error
				if agent.id == "goose" {
					data, err = yaml.Marshal(config)
				} else {
					data, err = json.Marshal(config)
				}
				require.NoError(t, err)
				return string(data)
			}
			local := func(command string) map[string]any {
				entry := map[string]any{"command": command, "args": []any{"mcp"}}
				if agent.localType != "" {
					entry["type"] = agent.localType
				}
				if agent.id == "github-copilot" {
					entry["tools"] = []any{"*"}
				}
				if agent.id == "goose" {
					delete(entry, "command")
					entry["cmd"], entry["name"], entry["enabled"] = command, "blaxel", true
				}
				if agent.id == "openclaw" {
					entry["transport"] = entry["type"]
					delete(entry, "type")
				}
				return entry
			}
			hosted := map[string]any{agent.urlKey: "https://api.blaxel.ai/v0/mcp"}
			if agent.remoteType != "" {
				hosted["type"] = agent.remoteType
			}
			if agent.id == "github-copilot" {
				hosted["tools"] = []string{"*"}
			}
			if agent.id == "goose" {
				hosted["name"], hosted["enabled"] = "blaxel", true
			}
			if agent.id == "openclaw" {
				hosted["transport"] = hosted["type"]
				delete(hosted, "type")
			}
			custom := local(oldCommand)
			custom["args"] = []string{"mcp", "--workspace", "my-workspace"}
			customEnv := local(oldCommand)
			envKey := "env"
			if agent.id == "goose" {
				envKey = "envs"
			}
			customEnv[envKey] = map[string]any{"BL_ENV": "dev"}
			disabled := local(oldCommand)
			disabled["enabled"] = false
			for _, fixture := range []struct {
				name   string
				entry  map[string]any
				change mcpChange
			}{
				{"new", nil, mcpAdded},
				{"hosted", hosted, mcpReplaced},
				{"old command", local(oldCommand), mcpReplaced},
				{"custom", custom, mcpUnchanged},
				{"custom environment", customEnv, mcpUnchanged},
				{"disabled", disabled, mcpUnchanged},
			} {
				t.Run(fixture.name, func(t *testing.T) {
					e := testMCPEnv(t.TempDir(), map[string]string{"MISSING:" + oldCommand: "1"}, &[]fakeCommand{}, "", nil)
					target := mcpTargets[agent.id]
					file := target.file(e)
					original := encode(fixture.entry)
					writeTestFile(t, file, original)
					change, err := addMCPServer(context.Background(), e, target, testResourceServer)
					require.NoError(t, err)
					assert.Equal(t, fixture.change, change)
					if fixture.change == mcpUnchanged {
						assert.Equal(t, original, readTestFile(t, file))
					} else {
						command, args := entryCommand(target.entry(e, "blaxel"))
						assert.Equal(t, testResourceServer.command[0], command)
						assert.Equal(t, []string{"mcp"}, args)
					}
					change, err = addMCPServer(context.Background(), e, target, testDocsServer)
					require.NoError(t, err)
					assert.Equal(t, mcpAdded, change)
					var config map[string]any
					data := []byte(readTestFile(t, file))
					if agent.id == "goose" {
						require.NoError(t, yaml.Unmarshal(data, &config))
					} else {
						require.NoError(t, json.Unmarshal(data, &config))
					}
					assert.Equal(t, "dark", config["theme"])
					container := config
					if agent.id == "openclaw" {
						container = config["mcp"].(map[string]any)
						assert.Equal(t, "keep", container["setting"])
					}
					servers := container[agent.container].(map[string]any)
					assert.Equal(t, map[string]any{"command": "my-server", "setting": "keep"}, servers["other"])
					assert.Equal(t, docsMCPURL, servers["blaxel-docs"].(map[string]any)[agent.urlKey])
					if fixture.change != mcpUnchanged {
						assert.Equal(t, local(testResourceServer.command[0]), servers["blaxel"])
					}
					before := readTestFile(t, file)
					for _, server := range []mcpServer{testResourceServer, testDocsServer} {
						change, err = addMCPServer(context.Background(), e, target, server)
						require.NoError(t, err)
						assert.Equal(t, mcpUnchanged, change)
					}
					assert.Equal(t, before, readTestFile(t, file))
				})
			}
			t.Run("malformed", func(t *testing.T) {
				e := testMCPEnv(t.TempDir(), map[string]string{}, &[]fakeCommand{}, "", nil)
				target := mcpTargets[agent.id]
				badContainer := `{"` + agent.container + `":[]}`
				if agent.id == "openclaw" {
					badContainer = `{"mcp":` + badContainer + `}`
				}
				for _, original := range []string{"{broken", badContainer} {
					writeTestFile(t, target.file(e), original)
					_, err := addMCPServer(context.Background(), e, target, testResourceServer)
					require.Error(t, err)
					assert.Equal(t, original, readTestFile(t, target.file(e)))
				}
			})
		})
	}
}

func TestAdditionalAgentConfigPathsAndDetection(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{"COPILOT_HOME": filepath.Join(home, "copilot"), "GOOSE_PATH_ROOT": filepath.Join(home, "goose-root")}
	p := newSkillsAgentPaths(home, func(key string) string { return env[key] })
	e := testMCPEnv(home, env, &[]fakeCommand{}, "", nil)
	assert.Equal(t, filepath.Join(env["COPILOT_HOME"], "mcp-config.json"), mcpTargets["github-copilot"].file(e))
	assert.Equal(t, filepath.Join(env["GOOSE_PATH_ROOT"], "config", "config.yaml"), mcpTargets["goose"].file(e))
	for _, dir := range []string{env["COPILOT_HOME"], gooseConfigDir(p), vscodeUserDir(p)} {
		require.NoError(t, os.MkdirAll(dir, 0700))
	}
	var ids []string
	for _, agent := range detectedSetupAgents(home, p.env) {
		ids = append(ids, agent.id)
	}
	assert.Equal(t, []string{"github-copilot", "goose", "vscode"}, ids)
	vscode, ok := findSkillsAgent("vscode")
	require.True(t, ok)
	targets, _ := skillsTargets([]skillsAgent{vscode})
	assert.Equal(t, []string{"universal"}, targets)
	if runtime.GOOS == "windows" {
		env["GOOSE_PATH_ROOT"] = ""
		env["APPDATA"] = filepath.Join(home, "roaming")
		assert.Equal(t, filepath.Join(env["APPDATA"], "Block", "goose", "config"), gooseConfigDir(p))
		assert.Equal(t, filepath.Join(env["APPDATA"], "Code", "User"), vscodeUserDir(p))
	}
}

func TestAmpLeavesExistingJSONCWithCommentsAlone(t *testing.T) {
	e := testMCPEnv(t.TempDir(), map[string]string{}, &[]fakeCommand{}, "", nil)
	file := ampConfigFile(e) + "c"
	original := "{\n// user settings\n\"amp.mcpServers\": {}\n}"
	writeTestFile(t, file, original)
	_, err := addMCPServer(context.Background(), e, mcpTargets["amp"], testResourceServer)
	require.Error(t, err)
	assert.Equal(t, original, readTestFile(t, file))
	assert.NoFileExists(t, file[:len(file)-1])
}

func TestGoosePreservesCommentsAndRejectsAmbiguousYAML(t *testing.T) {
	e := testMCPEnv(t.TempDir(), map[string]string{}, &[]fakeCommand{}, "", nil)
	target := mcpTargets["goose"]
	file := target.file(e)
	writeTestFile(t, file, "# my settings\ntheme: dark # keep this\nextensions: {}\n")
	_, err := addMCPServer(context.Background(), e, target, testResourceServer)
	require.NoError(t, err)
	assert.Contains(t, readTestFile(t, file), "# my settings")
	assert.Contains(t, readTestFile(t, file), "# keep this")
	for _, original := range []string{
		"extensions: {}\nextensions: {}\n",
		"extensions: {}\n---\ntheme: dark\n",
		"defaults: &defaults {extensions: {other: {cmd: mine}}}\n<<: *defaults\n",
		"defaults: &defaults {other: {cmd: mine}}\nextensions: {<<: *defaults}\n",
	} {
		writeTestFile(t, file, original)
		_, err := addMCPServer(context.Background(), e, target, testResourceServer)
		require.Error(t, err)
		assert.Equal(t, original, readTestFile(t, file))
	}
}

func TestAmpUsesXDGConfigHome(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{"XDG_CONFIG_HOME": filepath.Join(home, "xdg")}
	paths := newSkillsAgentPaths(home, func(key string) string { return env[key] })
	e := testMCPEnv(home, env, &[]fakeCommand{}, "", nil)
	e.config = paths.config
	target := mcpTargets["amp"]
	file := filepath.Join(env["XDG_CONFIG_HOME"], "amp", "settings.json")
	assert.Equal(t, file, target.file(e))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".config", "amp"), 0700))
	assert.Empty(t, detectedSetupAgents(home, paths.env))
	for _, server := range []mcpServer{testResourceServer, testDocsServer} {
		change, err := addMCPServer(context.Background(), e, target, server)
		require.NoError(t, err)
		assert.Equal(t, mcpAdded, change)
		assert.NotNil(t, target.entry(e, server.name))
	}
	assert.FileExists(t, file)
	assert.NoFileExists(t, filepath.Join(home, ".config", "amp", "settings.json"))
	agents := detectedSetupAgents(home, paths.env)
	require.Len(t, agents, 1)
	assert.Equal(t, "amp", agents[0].id)
}

func TestGoosePreservesCommentsInConfigLayouts(t *testing.T) {
	for _, original := range []string{
		"# my settings\n# keep this comment\n",
		"# my settings\nextensions: # keep this comment\n",
		"# my settings\n# keep this comment\n\ntheme: dark\nextensions: {}\n",
		"theme: dark\nextensions: {}\n\n# my settings\n# keep this comment\n",
		"# my settings\n\n# keep this comment\n",
	} {
		t.Run(original, func(t *testing.T) {
			e := testMCPEnv(t.TempDir(), map[string]string{}, &[]fakeCommand{}, "", nil)
			target := mcpTargets["goose"]
			file := target.file(e)
			writeTestFile(t, file, original)
			for _, server := range []mcpServer{testResourceServer, testDocsServer} {
				change, err := addMCPServer(context.Background(), e, target, server)
				require.NoError(t, err)
				assert.Equal(t, mcpAdded, change)
				assert.NotNil(t, target.entry(e, server.name))
			}
			before := readTestFile(t, file)
			for _, line := range strings.Split(original, "\n") {
				if line != "" && !strings.HasPrefix(line, "extensions:") {
					assert.Equal(t, 1, strings.Count(before, line+"\n"), "lost or duplicated line: %s", line)
				}
			}
			assert.Contains(t, before, "# keep this comment")
			assert.GreaterOrEqual(t, strings.Count(before, "\n\n"), strings.Count(original, "\n\n"))
			for _, server := range []mcpServer{testResourceServer, testDocsServer} {
				change, err := addMCPServer(context.Background(), e, target, server)
				require.NoError(t, err)
				assert.Equal(t, mcpUnchanged, change)
			}
			assert.Equal(t, before, readTestFile(t, file))
		})
	}
}

func TestContinueLeavesServersInOtherConfigsAlone(t *testing.T) {
	for _, fixture := range []struct{ file, content string }{
		{"config.yaml", "mcpServers:\n  - name: blaxel\n    command: mine\n"},
		{"mcpServers/custom.json", `{"mcpServers":{"blaxel":{"url":"https://example.com/mcp"}}}`},
		{"mcpServers/nested/custom.yaml", "mcpServers:\n  - name: blaxel\n    command: mine\n"},
		// Continue loads a single-server file as the server named after it.
		{"mcpServers/nested/blaxel.json", `{"command":"mine","args":["--flag"]}`},
	} {
		t.Run(fixture.file, func(t *testing.T) {
			e := testMCPEnv(t.TempDir(), map[string]string{}, &[]fakeCommand{}, "", nil)
			file := filepath.Join(e.home, ".continue", fixture.file)
			writeTestFile(t, file, fixture.content)
			result := configureAgentMCP(context.Background(), e, mcpTargets["continue"], []mcpServer{testResourceServer, testDocsServer})
			require.NoError(t, result.err)
			assert.Equal(t, []string{"blaxel"}, result.existing)
			assert.Equal(t, []string{"blaxel-docs"}, result.added)
			assert.Equal(t, fixture.content, readTestFile(t, file))
			assert.Nil(t, jsonConfigEntry(mcpTargets["continue"].file(e), "mcpServers", "blaxel"))
		})
	}
}

func TestContinueLeavesASingleServerBlaxelJSONAlone(t *testing.T) {
	e := testMCPEnv(t.TempDir(), map[string]string{}, &[]fakeCommand{}, "", nil)
	target := mcpTargets["continue"]
	original := `{"command":"mine","args":["--flag"],"env":{"TOKEN":"x"}}`
	writeTestFile(t, target.file(e), original)
	result := configureAgentMCP(context.Background(), e, target, []mcpServer{testResourceServer, testDocsServer})
	assert.Equal(t, []string{"blaxel"}, result.existing)
	require.ErrorContains(t, result.err, "your own server file")
	assert.Equal(t, original, readTestFile(t, target.file(e)))
}

func TestContinueLeavesUnparseableSiblingConfigsAlone(t *testing.T) {
	for _, original := range []string{
		"{ // my servers\n\"mcpServers\": {}\n}",
		`{"mcpServers":`,
	} {
		e := testMCPEnv(t.TempDir(), map[string]string{}, &[]fakeCommand{}, "", nil)
		file := filepath.Join(e.home, ".continue", "mcpServers", "custom.json")
		writeTestFile(t, file, original)
		_, err := addMCPServer(context.Background(), e, mcpTargets["continue"], testResourceServer)
		require.Error(t, err)
		assert.Equal(t, original, readTestFile(t, file))
		assert.NoFileExists(t, mcpTargets["continue"].file(e))
	}
}

func TestOpenClawLeavesIncludedAndJSON5ConfigsAlone(t *testing.T) {
	for _, original := range []string{
		`{"$include":"other.json"}`,
		`{"mcp":{"$include":"servers.json"}}`,
		`{"mcp":{"servers":{"$include":"servers.json"}}}`,
		"{ // my settings\n\"mcp\": {}\n}",
	} {
		e := testMCPEnv(t.TempDir(), map[string]string{}, &[]fakeCommand{}, "", nil)
		file := openclawConfigFile(e)
		writeTestFile(t, file, original)
		_, err := addMCPServer(context.Background(), e, mcpTargets["openclaw"], testResourceServer)
		require.Error(t, err)
		assert.Equal(t, original, readTestFile(t, file))
	}
}

// OpenClaw reads only ~/.openclaw, and openclaw doctor moves a legacy folder
// there only while ~/.openclaw does not exist.
func TestOpenClawLeavesALegacyOnlyInstallToDoctor(t *testing.T) {
	for _, legacy := range []string{".clawdbot", ".moltbot"} {
		t.Run(legacy, func(t *testing.T) {
			e := testMCPEnv(t.TempDir(), map[string]string{}, &[]fakeCommand{}, "", nil)
			require.NoError(t, os.MkdirAll(filepath.Join(e.home, legacy), 0700))
			_, err := addMCPServer(context.Background(), e, mcpTargets["openclaw"], testResourceServer)
			require.ErrorContains(t, err, "openclaw doctor")
			assert.NoDirExists(t, filepath.Join(e.home, ".openclaw"))

			require.NoError(t, os.MkdirAll(filepath.Join(e.home, ".openclaw"), 0700))
			change, err := addMCPServer(context.Background(), e, mcpTargets["openclaw"], testResourceServer)
			require.NoError(t, err)
			assert.Equal(t, mcpAdded, change)
		})
	}
}

func TestCrushLeavesShellConfigAlone(t *testing.T) {
	e := testMCPEnv(t.TempDir(), map[string]string{}, &[]fakeCommand{}, "", nil)
	file := filepath.Join(crushConfigDir(e.paths()), "crushrc")
	original := "# my server\nmcp add blaxel --command mine\n"
	writeTestFile(t, file, original)
	_, err := addMCPServer(context.Background(), e, mcpTargets["crush"], testResourceServer)
	require.ErrorContains(t, err, "crushrc")
	assert.Equal(t, original, readTestFile(t, file))
	assert.NoFileExists(t, mcpTargets["crush"].file(e))
}

func TestCopilotLeavesTheEnabledPluginServerAlone(t *testing.T) {
	for _, fixture := range []struct {
		name, output string
		plugin       bool
	}{
		{"enabled", `{"mcpServers":{"blaxel":{"sourcePlugin":"blaxel","source":"plugin","enabled":true}}}`, true},
		{"disabled", `{"mcpServers":{"blaxel":{"sourcePlugin":"blaxel","source":"plugin","enabled":false}}}`, false},
		{"skills only", `{"mcpServers":{}}`, false},
		{"user server", `{"mcpServers":{"blaxel":{"source":"user","enabled":true}}}`, false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var commands []fakeCommand
			e := testMCPEnv(t.TempDir(), map[string]string{"PATH_HAS_copilot": "1"}, &commands, fixture.output, nil)
			target := mcpTargets["github-copilot"]
			result := configureAgentMCP(context.Background(), e, target, []mcpServer{testResourceServer, testDocsServer})
			require.NoError(t, result.err)
			assert.Equal(t, fixture.plugin, len(result.plugin) == 1)
			assert.Equal(t, fixture.plugin, target.entry(e, "blaxel") == nil)
			assert.NotNil(t, target.entry(e, "blaxel-docs"))
			assert.Equal(t, []string{"mcp", "list", "--json"}, commands[0].args)
		})
	}
}

func TestAdditionalConfigOverridesAreDetected(t *testing.T) {
	for _, fixture := range []struct{ id, variable, relativeFile string }{
		{"cline", "CLINE_MCP_SETTINGS_PATH", "cline_mcp_settings.json"},
		{"cline", "CLINE_DATA_DIR", "settings/cline_mcp_settings.json"},
		{"cline", "CLINE_DIR", "data/settings/cline_mcp_settings.json"},
		{"continue", "CONTINUE_GLOBAL_DIR", "mcpServers/blaxel.json"},
		{"crush", "CRUSH_GLOBAL_CONFIG", "crush.json"},
		{"openclaw", "OPENCLAW_STATE_DIR", "openclaw.json"},
		{"openclaw", "OPENCLAW_CONFIG_PATH", "openclaw.json"},
	} {
		t.Run(fixture.variable, func(t *testing.T) {
			home := t.TempDir()
			dir := filepath.Join(home, "override")
			file := filepath.Join(dir, filepath.FromSlash(fixture.relativeFile))
			env := map[string]string{fixture.variable: dir}
			if fixture.variable == "CLINE_MCP_SETTINGS_PATH" || fixture.variable == "OPENCLAW_CONFIG_PATH" {
				env[fixture.variable] = file
			}
			e := testMCPEnv(home, env, &[]fakeCommand{}, "", nil)
			assert.Equal(t, file, mcpTargets[fixture.id].file(e))
			writeTestFile(t, file, "{}")
			agents := detectedSetupAgents(home, e.env)
			require.Len(t, agents, 1)
			assert.Equal(t, fixture.id, agents[0].id)
		})
	}
}

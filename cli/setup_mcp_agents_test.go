package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	} {
		t.Run(agent.id, func(t *testing.T) {
			oldCommand := filepath.Join(t.TempDir(), "old", "bl")
			encode := func(entry map[string]any) string {
				servers := map[string]any{"other": map[string]any{"command": "my-server", "setting": "keep"}}
				if entry != nil {
					servers["blaxel"] = entry
				}
				config := map[string]any{"theme": "dark", agent.container: servers}
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
					servers := config[agent.container].(map[string]any)
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
				for _, original := range []string{"{broken", `{"` + agent.container + `":[]}`} {
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

func TestGooseAddsServersToEmptyConfigLayouts(t *testing.T) {
	for _, original := range []string{
		"# my settings\n# keep this comment\n",
		"# my settings\nextensions: # keep this comment\n",
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
			assert.Contains(t, before, "# my settings")
			assert.Contains(t, before, "# keep this comment")
			for _, server := range []mcpServer{testResourceServer, testDocsServer} {
				change, err := addMCPServer(context.Background(), e, target, server)
				require.NoError(t, err)
				assert.Equal(t, mcpUnchanged, change)
			}
			assert.Equal(t, before, readTestFile(t, file))
		})
	}
}

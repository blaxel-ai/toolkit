package cli

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRefreshUpdatesSkillsAndNewAgentsWithoutLogin(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0755))
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{}, recorder)
	archive := buildSkillsArchive(t, testSkillsEntries())
	options.installSkills = func(_ context.Context, agents []skillsAgent) (skillsInstallResult, error) {
		return installSkillsArchive(archive, home, options.env, agents, time.Now())
	}
	runSetupRefresh(context.Background(), options)
	manifest := filepath.Join(home, ".agents", "skills", "blaxel-cli", "SKILL.md")
	writeTestFile(t, manifest, "outdated skill")
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".codex"), 0755))
	runSetupRefresh(context.Background(), options)
	assert.Equal(t, skillManifest("blaxel-cli"), readTestFile(t, manifest))
	command, args := entryCommand(mcpTargets["codex"].entry(options.mcp, "blaxel"))
	assert.Equal(t, testResourceServer.command[0], command)
	assert.Equal(t, []string{"mcp"}, args)
	assert.NotNil(t, mcpTargets["codex"].entry(options.mcp, "blaxel-docs"))
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
	options := testSetupOptions(t, home, map[string]string{skillsInstallEnv: "false"}, recorder)
	runSetupRefresh(context.Background(), options)
	command, args := entryCommand(mcpTargets["cursor"].entry(options.mcp, "blaxel"))
	assert.Equal(t, testResourceServer.command[0], command)
	assert.Equal(t, []string{"mcp"}, args)
	assert.Equal(t, custom, mcpTargets["gemini-cli"].entry(options.mcp, "blaxel"))
	assert.Nil(t, mcpTargets["claude-code"].entry(options.mcp, "blaxel"))
	assert.NotNil(t, mcpTargets["claude-code"].entry(options.mcp, "blaxel-docs"))
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
		{"skills disabled", map[string]string{skillsInstallEnv: " FALSE "}, false, true},
		{"MCP disabled", map[string]string{mcpInstallEnv: "false"}, true, false},
		{"both disabled", map[string]string{skillsInstallEnv: "false", mcpInstallEnv: "false"}, false, false},
		{"setup disabled", map[string]string{"BL_INSTALL_SETUP": "false"}, false, false},
		{"CI", map[string]string{"CI": "true"}, false, false},
		{"CI skills forced", map[string]string{"CI": "true", skillsInstallEnv: "true"}, true, false},
		{"CI MCP forced", map[string]string{"GITHUB_ACTIONS": "true", mcpInstallEnv: "true"}, false, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			require.NoError(t, os.MkdirAll(filepath.Join(home, ".cursor"), 0755))
			recorder := &setupRecorder{}
			options := testSetupOptions(t, home, test.env, recorder)
			runSetupRefresh(context.Background(), options)
			assert.Equal(t, test.skills, len(recorder.skillsAgents) > 0)
			assert.Equal(t, test.mcp, mcpTargets["cursor"].entry(options.mcp, "blaxel") != nil)
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
				options.installSkills = func(_ context.Context, agents []skillsAgent) (skillsInstallResult, error) {
					return installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, options.env, agents, time.Now())
				}
			}
			runSetupRefresh(context.Background(), options)
			assert.Equal(t, manifest, readTestFile(t, filepath.Join(external, "SKILL.md")))
			assert.NotNil(t, mcpTargets["cursor"].entry(options.mcp, "blaxel"))
			if offline {
				assert.Contains(t, recorder.text(t), "1 problem; retry with bl setup")
			} else {
				assert.Contains(t, recorder.text(t), "1 externally managed kept")
				assert.NotContains(t, readTestFile(t, skillsLockPath(home, options.env)), `"blaxel-cli"`)
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
	options := testSetupOptions(t, home, map[string]string{skillsInstallEnv: "false"}, recorder)
	runSetupRefresh(context.Background(), options)
	assert.Equal(t, "broken configuration", readTestFile(t, filepath.Join(home, ".claude.json")))
	assert.NotNil(t, mcpTargets["codex"].entry(options.mcp, "blaxel"))
	assert.Contains(t, recorder.text(t), "1 problem; retry with bl setup")
	assert.Len(t, strings.Split(strings.TrimSpace(recorder.text(t)), "\n"), 1)
}

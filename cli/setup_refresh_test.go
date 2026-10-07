package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blaxel-ai/toolkit/cli/agentsetup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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

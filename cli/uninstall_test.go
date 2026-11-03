package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/blaxel-ai/toolkit/cli/agentsetup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// setUpForUninstall installs what bl setup writes for Claude Code and Cursor
// in a temporary home, next to a server of the user's own.
func setUpForUninstall(t *testing.T) (uninstallOptions, *bytes.Buffer) {
	t.Helper()
	home := t.TempDir()
	for _, dir := range []string{".claude", ".cursor"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, dir), 0755))
	}
	_, err := agentsetup.InstallSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv,
		agentsetup.DetectedSetupAgents(home, noEnv), time.Now())
	require.NoError(t, err)
	var commands []fakeCommand
	mcp := testMCPEnv(home, map[string]string{}, &commands, "", nil)
	writeTestFile(t, filepath.Join(home, ".cursor", "mcp.json"), `{"mcpServers":{"mine":{"url":"https://example.com/mcp"}}}`)
	servers := []agentsetup.MCPServer{agentsetup.ResourceMCPServer("/opt/blaxel/bin/bl"), agentsetup.DocsMCPServer()}
	for _, id := range []string{"claude-code", "cursor"} {
		require.NoError(t, agentsetup.ConfigureAgentMCP(context.Background(), mcp, agentsetup.MCPTargets[id], servers).Err)
	}
	var out bytes.Buffer
	bin := filepath.Join(home, ".local", "bin")
	writeTestFile(t, filepath.Join(bin, "bl"), "")
	return uninstallOptions{
		home: home, env: noEnv, mcp: mcp, out: &out, in: strings.NewReader(""),
		cli: agentsetup.CLIInstall{Executable: filepath.Join(bin, "bl"), Home: home, GOOS: "linux", Env: noEnv, Sudo: func(string) bool { return false }},
	}, &out
}

func homeFiles(t *testing.T, home string) []string {
	t.Helper()
	var files []string
	require.NoError(t, filepath.Walk(home, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			files = append(files, strings.TrimPrefix(path, home))
		}
		return err
	}))
	return files
}

func TestUninstallDryRunChangesNothing(t *testing.T) {
	options, out := setUpForUninstall(t)
	options.dryRun = true
	before := homeFiles(t, options.home)
	claudeJSON := readTestFile(t, filepath.Join(options.home, ".claude.json"))
	require.NoError(t, runUninstall(context.Background(), options))
	assert.Equal(t, before, homeFiles(t, options.home))
	assert.Equal(t, claudeJSON, readTestFile(t, filepath.Join(options.home, ".claude.json")))
	text := out.String()
	for _, expected := range []string{
		"bl uninstall removes:",
		"  Skills\n    " + filepath.Join("~", ".claude", "skills", "blaxel-cli") + "  ",
		"    " + filepath.Join("~", ".agents", "skills", "blaxel-cli") + "\n",
		"    " + filepath.Join("~", ".agents", ".skill-lock.json") + "  entries for blaxel-cli and blaxel-sdk\n",
		"  Skills updates\n    " + filepath.Join("~", ".blaxel", "skills", "state.json") + "  automatic updates off\n",
		"  MCP servers\n    " + filepath.Join("~", ".claude.json") + "  blaxel, blaxel-docs\n    " + filepath.Join("~", ".cursor", "mcp.json") + "  blaxel, blaxel-docs\n",
		"Dry run: nothing was removed.",
		"  MCP updates\n    " + filepath.Join("~", ".blaxel", "mcp", "state.json") + "  automatic updates off\n",
		"To remove the bl CLI as well:\n  rm ~/.local/bin/bl\n  rm -rf ~/.blaxel  # your Blaxel logins and settings\n",
	} {
		assert.Contains(t, text, expected)
	}
	assert.NotContains(t, text, "bl upgrade adds")
}

// Without a terminal and without --yes, uninstall lists what it would remove,
// says how to confirm, and exits with an error instead of waiting.
func TestUninstallWithoutATerminalNeedsYes(t *testing.T) {
	options, out := setUpForUninstall(t)
	before := homeFiles(t, options.home)
	require.ErrorIs(t, runUninstall(context.Background(), options), errUninstallIncomplete)
	assert.Equal(t, before, homeFiles(t, options.home))
	assert.Contains(t, out.String(), "bl uninstall removes:")
	assert.Contains(t, out.String(), "Nothing was removed. To remove these, run: bl uninstall --yes")
	assert.NotContains(t, out.String(), "[y/N]")
}

func TestUninstallAsksInATerminal(t *testing.T) {
	options, out := setUpForUninstall(t)
	options.interactive = true
	before := homeFiles(t, options.home)
	options.in = strings.NewReader("n\n")
	require.NoError(t, runUninstall(context.Background(), options))
	assert.Equal(t, before, homeFiles(t, options.home))
	assert.Contains(t, out.String(), "Remove these? [y/N] Nothing was removed.")

	out.Reset()
	options.in = strings.NewReader("y\n")
	require.NoError(t, runUninstall(context.Background(), options))
	assert.Contains(t, out.String(), "Removed:")
	assert.NoDirExists(t, filepath.Join(options.home, ".agents", "skills", "blaxel-cli"))
}

func TestUninstallYesRemovesEverythingSetupWrote(t *testing.T) {
	options, out := setUpForUninstall(t)
	options.yes = true
	require.NoError(t, runUninstall(context.Background(), options))
	home := options.home
	assert.Equal(t, []string{
		filepath.Join(string(filepath.Separator)+".blaxel", "mcp", "state.json"),
		filepath.Join(string(filepath.Separator)+".blaxel", "skills", "state.json"),
		filepath.Join(string(filepath.Separator)+".blaxel", "skills", "update.lock"),
		filepath.Join(string(filepath.Separator)+".cursor", "mcp.json"),
		filepath.Join(string(filepath.Separator)+".local", "bin", "bl"),
	}, homeFiles(t, home))
	assert.JSONEq(t, `{"mcpServers":{"mine":{"url":"https://example.com/mcp"}}}`, readTestFile(t, filepath.Join(home, ".cursor", "mcp.json")))
	text := out.String()
	assert.Contains(t, text, "Removed:\n  Skills\n")
	assert.Contains(t, text, "Restart your coding agents")
	assert.Contains(t, text, "Automatic skills updates are off; bl setup turns them back on.")
	assert.Contains(t, text, "Upgrades no longer add the MCP servers; run bl setup to add them back.")
	assert.NotContains(t, text, "Not removed")

	out.Reset()
	require.NoError(t, runUninstall(context.Background(), options))
	assert.Contains(t, out.String(), "Nothing to remove")
	assert.Contains(t, out.String(), "To remove the bl CLI as well:")
}

func TestUninstallSkipFlags(t *testing.T) {
	options, out := setUpForUninstall(t)
	options.yes, options.skipSkills = true, true
	require.NoError(t, runUninstall(context.Background(), options))
	assert.DirExists(t, filepath.Join(options.home, ".agents", "skills", "blaxel-cli"))
	assert.NoFileExists(t, filepath.Join(options.home, ".claude.json"))
	assert.NotContains(t, out.String(), "Automatic skills updates")
	assert.Contains(t, out.String(), "run bl setup to add them back")

	options, out = setUpForUninstall(t)
	options.yes, options.skipMCP = true, true
	require.NoError(t, runUninstall(context.Background(), options))
	assert.NoDirExists(t, filepath.Join(options.home, ".agents", "skills", "blaxel-cli"))
	assert.FileExists(t, filepath.Join(options.home, ".claude.json"))
	assert.False(t, agentsetup.MCPAutoUpdateOff(options.home))
	assert.NotContains(t, out.String(), "run bl setup to add them back")
}

// After bl uninstall, upgrades and install scripts leave the skills and MCP
// servers out, even when BL_INSTALL_SKILLS or BL_INSTALL_MCP is true; a bl
// setup you run adds them back and turns their automatic updates on again.
func TestUninstallKeepsSkillsAndMCPOffUntilSetup(t *testing.T) {
	options, _ := setUpForUninstall(t)
	options.yes = true
	require.NoError(t, runUninstall(context.Background(), options))
	home := options.home
	cursorFile := filepath.Join(home, ".cursor", "mcp.json")
	cursor := readTestFile(t, cursorFile)
	require.True(t, agentsetup.MCPAutoUpdateOff(home))
	require.True(t, agentsetup.SkillsAutoUpdateOffByUninstall(home))

	// bl upgrade, Homebrew and the install scripts on an existing install refresh.
	recorder := &setupRecorder{}
	refresh := testSetupOptions(t, home, map[string]string{mcpInstallEnv: "true", agentsetup.SkillsInstallEnv: "true"}, recorder)
	runSetupRefresh(context.Background(), refresh)
	assert.Empty(t, recorder.skillsAgents)
	assert.Equal(t, cursor, readTestFile(t, cursorFile))
	assert.NoFileExists(t, filepath.Join(home, ".claude.json"))
	assert.Contains(t, recorder.text(t), "skills skipped; MCP skipped")

	// The install scripts on a new install run bl setup, and say why the
	// agents get nothing.
	recorder = &setupRecorder{}
	installer := testSetupOptions(t, home, map[string]string{installerEnv: "1", installerShellEnv: "bl on PATH"}, recorder)
	installer.yes = true
	require.NoError(t, runSetup(context.Background(), installer))
	assert.Empty(t, recorder.skillsAgents)
	assert.Equal(t, cursor, readTestFile(t, cursorFile))
	assert.NoFileExists(t, filepath.Join(home, ".claude.json"))
	text := recorder.text(t)
	for _, wrong := range []string{"set up · skills", "already set up", "Restart your agents", "undo this setup"} {
		assert.NotContains(t, text, wrong)
	}
	assert.Contains(t, text, "skills and MCP servers stay removed after bl uninstall")
	assert.Contains(t, text, "add the skills and MCP servers back")

	// A setup that leaves both out keeps their updates off.
	skipped := testSetupOptions(t, home, map[string]string{}, &setupRecorder{})
	skipped.skipMCP, skipped.skipSkills = true, true
	require.NoError(t, runSetup(context.Background(), skipped))
	assert.True(t, agentsetup.MCPAutoUpdateOff(home))
	assert.True(t, agentsetup.SkillsAutoUpdateOffByUninstall(home))

	// A setup whose skills install fails keeps the skills off; the MCP
	// servers that went in come back on.
	failing := testSetupOptions(t, home, map[string]string{}, &setupRecorder{})
	failing.installSkills = func(context.Context, []agentsetup.SkillsAgent) (agentsetup.SkillsInstallResult, error) {
		return agentsetup.SkillsInstallResult{}, errors.New("archive unavailable")
	}
	require.Error(t, runSetup(context.Background(), failing))
	assert.True(t, agentsetup.SkillsAutoUpdateOffByUninstall(home), "a failed skills install keeps them off")
	assert.False(t, agentsetup.MCPAutoUpdateOff(home))
	require.NoError(t, agentsetup.SetMCPAutoUpdate(home, false))

	// bl setup adds them back and turns their automatic updates on.
	recorder = &setupRecorder{}
	setup := testSetupOptions(t, home, map[string]string{}, recorder)
	require.NoError(t, runSetup(context.Background(), setup))
	assert.NotEmpty(t, recorder.skillsAgents)
	assert.NotNil(t, agentsetup.MCPTargets["cursor"].Entry(setup.mcp, "blaxel"))
	assert.NotNil(t, agentsetup.MCPTargets["cursor"].Entry(setup.mcp, "mine"))
	assert.NotNil(t, agentsetup.MCPTargets["claude-code"].Entry(setup.mcp, "blaxel-docs"))
	assert.False(t, agentsetup.MCPAutoUpdateOff(home))
	assert.NoFileExists(t, filepath.Join(home, ".blaxel", "mcp", "state.json"))
	state, err := agentsetup.ReadSkillsUpdateState(home)
	require.NoError(t, err)
	assert.Empty(t, agentsetup.SkillsUpdateDisabled(state, noEnv))
}

// bl skills autoupdate off, before or after bl uninstall, keeps the skills
// removed until bl setup, and bl setup keeps the user's choice.
func TestUninstallKeepsSkillsOffWithTheUsersUpdatesChoice(t *testing.T) {
	disabled := false
	for _, before := range []bool{true, false} {
		name := "autoupdate off after uninstall"
		if before {
			name = "autoupdate off before uninstall"
		}
		t.Run(name, func(t *testing.T) {
			options, _ := setUpForUninstall(t)
			home := options.home
			t.Setenv("HOME", home)
			t.Setenv("USERPROFILE", home)
			if before {
				require.NoError(t, agentsetup.SetSkillsAutoUpdate(context.Background(), false))
			}
			options.yes = true
			require.NoError(t, runUninstall(context.Background(), options))
			if !before {
				require.NoError(t, agentsetup.SetSkillsAutoUpdate(context.Background(), false))
			}
			require.True(t, agentsetup.SkillsAutoUpdateOffByUninstall(home))

			// bl upgrade and Homebrew refresh.
			recorder := &setupRecorder{}
			runSetupRefresh(context.Background(), testSetupOptions(t, home, map[string]string{agentsetup.SkillsInstallEnv: "true"}, recorder))
			assert.Empty(t, recorder.skillsAgents)

			// The install scripts on a new install.
			recorder = &setupRecorder{}
			installer := testSetupOptions(t, home, map[string]string{installerEnv: "1", installerShellEnv: "bl on PATH"}, recorder)
			installer.yes = true
			require.NoError(t, runSetup(context.Background(), installer))
			assert.Empty(t, recorder.skillsAgents)
			assert.Contains(t, recorder.text(t), "skills and MCP servers stay removed after bl uninstall")

			// bl setup adds them back, and only clears the uninstall.
			recorder = &setupRecorder{}
			require.NoError(t, runSetup(context.Background(), testSetupOptions(t, home, map[string]string{}, recorder)))
			assert.NotEmpty(t, recorder.skillsAgents)
			assert.False(t, agentsetup.SkillsAutoUpdateOffByUninstall(home))
			state, err := agentsetup.ReadSkillsUpdateState(home)
			require.NoError(t, err)
			assert.Equal(t, agentsetup.SkillsUpdateState{AutoUpdate: &disabled}, state, "the user's choice stays")
		})
	}
}

// A user's bl skills autoupdate off survives bl setup.
func TestSetupKeepsTheUsersSkillsUpdatesChoice(t *testing.T) {
	home := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0755))
	disabled := false
	require.NoError(t, agentsetup.WriteSkillsUpdateState(home, agentsetup.SkillsUpdateState{AutoUpdate: &disabled}))
	recorder := &setupRecorder{}
	require.NoError(t, runSetup(context.Background(), testSetupOptions(t, home, map[string]string{}, recorder)))
	assert.NotEmpty(t, recorder.skillsAgents, "setup still installs the skills")
	state, err := agentsetup.ReadSkillsUpdateState(home)
	require.NoError(t, err)
	assert.Equal(t, agentsetup.SkillsUpdateState{AutoUpdate: &disabled}, state)
}

func TestUninstallSkipsStartupWork(t *testing.T) {
	assert.Nil(t, homebrewRefreshFor([]string{"uninstall"}, func() {}), "no refresh right before removing")
	assert.Nil(t, homebrewRefreshFor([]string{"--skip-version-warning", "uninstall", "--yes"}, func() {}))
}

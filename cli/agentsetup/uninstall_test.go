package agentsetup

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// snapshot records every file under root with its contents, to show a plan
// changed nothing.
func snapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.Type()&fs.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			files[path] = "link:" + target
			return err
		}
		if !entry.IsDir() {
			data, err := os.ReadFile(path)
			files[path] = string(data)
			return err
		}
		return nil
	}))
	return files
}

// planMCPUninstall plans the MCP part. Every such plan ends by turning
// automatic MCP updates off; that item is checked here and left out, so tests
// see the servers alone.
func planMCPUninstall(t *testing.T, e MCPEnv) UninstallPlan {
	t.Helper()
	plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: e.Home, Env: e.Env, MCP: e, SkipSkills: true})
	require.NoError(t, err)
	require.NotEmpty(t, plan.Remove)
	last := plan.Remove[len(plan.Remove)-1]
	require.Equal(t, UninstallGroupMCPUpdates, last.Group)
	require.Equal(t, mcpSetupStatePath(e.Home), last.Path)
	require.True(t, plan.MCPAutoUpdateOff)
	plan.Remove = plan.Remove[:len(plan.Remove)-1]
	return plan
}

func runUninstallPlan(t *testing.T, plan UninstallPlan) {
	t.Helper()
	items, err := plan.Run(context.Background())
	require.NoError(t, err)
	for _, item := range items {
		require.NoError(t, item.Err, item.Path)
	}
}

func itemDetails(items []UninstallItem) map[string]string {
	details := map[string]string{}
	for _, item := range items {
		details[item.Path] = item.Detail
	}
	return details
}

// For every agent, uninstall removes exactly the servers setup wrote. The
// agent's other servers stay, and a file setup created is deleted again.
func TestUninstallRemovesWhatSetupWroteForEveryAgent(t *testing.T) {
	ids := make([]string, 0, len(MCPTargets))
	for id := range MCPTargets {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	other := MCPServer{Name: "other", Command: []string{"/usr/local/bin/other", "serve"}}
	for _, id := range ids {
		target := MCPTargets[id]
		servers := []MCPServer{testResourceServer, testDocsServer}
		expected := "blaxel, blaxel-docs"
		if target.localOnly {
			servers, expected = servers[:1], "blaxel"
		}
		t.Run(id+"/fresh", func(t *testing.T) {
			var commands []fakeCommand
			e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
			result := ConfigureAgentMCP(context.Background(), e, target, servers)
			require.NoError(t, result.Err)
			require.Len(t, result.Added, len(servers))
			plan := planMCPUninstall(t, e)
			assert.Equal(t, map[string]string{target.File(e): expected}, itemDetails(plan.Remove))
			assert.Empty(t, plan.Keep)
			runUninstallPlan(t, plan)
			assert.NoFileExists(t, target.File(e), "a file setup created is removed when nothing else is in it")
			assert.NoFileExists(t, mcpCreatedPath(e.Home), "and so is its record")
			assert.Empty(t, planMCPUninstall(t, e).Remove)
		})
		t.Run(id+"/shared", func(t *testing.T) {
			var commands []fakeCommand
			e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
			require.NoError(t, target.write(context.Background(), e, other, false))
			require.NoError(t, ConfigureAgentMCP(context.Background(), e, target, servers).Err)
			runUninstallPlan(t, planMCPUninstall(t, e))
			assert.Nil(t, target.Entry(e, "blaxel"))
			assert.Nil(t, target.Entry(e, "blaxel-docs"))
			assert.NotNil(t, target.Entry(e, "other"), "the agent's other servers stay")
			assert.Empty(t, commands)
		})
		t.Run(id+"/yours", func(t *testing.T) {
			var commands []fakeCommand
			e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
			yours := []MCPServer{{Name: "blaxel", Command: []string{"/usr/local/bin/my-wrapper", "mcp"}}}
			if !target.localOnly {
				yours = append(yours, MCPServer{Name: "blaxel-docs", URL: "https://docs.example.com/mcp"})
			}
			for _, server := range yours {
				require.NoError(t, target.write(context.Background(), e, server, false))
			}
			before := readTestFile(t, target.File(e))
			plan := planMCPUninstall(t, e)
			assert.Empty(t, plan.Remove, "servers with the Blaxel names that setup did not write are kept")
			assert.Len(t, plan.Keep, len(yours))
			runUninstallPlan(t, plan)
			assert.Equal(t, before, readTestFile(t, target.File(e)))
		})
	}
}

func TestUninstallKeepsChangedEntries(t *testing.T) {
	for _, fixture := range []struct{ name, target, config string }{
		{"extra argument", "cursor", `{"mcpServers":{"blaxel":{"command":"/opt/blaxel/bin/bl","args":["mcp","--workspace","prod"]}}}`},
		{"env", "cursor", `{"mcpServers":{"blaxel":{"command":"/opt/blaxel/bin/bl","args":["mcp"],"env":{"BL_WORKSPACE":"prod"}}}}`},
		{"relative command", "cursor", `{"mcpServers":{"blaxel":{"command":"bl","args":["mcp"]}}}`},
		{"other binary", "cursor", `{"mcpServers":{"blaxel":{"command":"/opt/blaxel/bin/blx","args":["mcp"]}}}`},
		{"docs headers", "cursor", `{"mcpServers":{"blaxel-docs":{"url":"https://docs.blaxel.ai/mcp","headers":{"X":"1"}}}}`},
		{"disabled", "opencode", `{"mcp":{"blaxel":{"type":"local","command":["/opt/blaxel/bin/bl","mcp"],"enabled":false}}}`},
		{"hosted with query", "cursor", `{"mcpServers":{"blaxel":{"url":"https://api.blaxel.ai/v0/mcp?workspace=prod"}}}`},
		{"docs on desktop", "claude-desktop", `{"mcpServers":{"blaxel-docs":{"url":"https://docs.blaxel.ai/mcp"}}}`},
		{"TOML extra key", "codex", "[mcp_servers.blaxel]\ncommand = \"/opt/blaxel/bin/bl\"\nargs = [\"mcp\"]\nstartup_timeout_sec = 30\n"},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			var commands []fakeCommand
			e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
			writeTestFile(t, MCPTargets[fixture.target].File(e), fixture.config)
			plan := planMCPUninstall(t, e)
			assert.Empty(t, plan.Remove)
			require.Len(t, plan.Keep, 1)
			assert.Contains(t, plan.Keep[0].Detail, "not what setup writes")
		})
	}
}

// Earlier setups added the hosted server, and any installed bl may have
// written the bl mcp entry.
func TestUninstallRemovesEarlierBlaxelEntries(t *testing.T) {
	var commands []fakeCommand
	e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
	cursor := MCPTargets["cursor"].File(e)
	writeTestFile(t, cursor, `{"mcpServers":{"blaxel":{"url":"https://api.blaxel.ai/v0/mcp"},"mine":{"url":"https://example.com"}}}`)
	vscode := MCPTargets["vscode"].File(e)
	writeTestFile(t, vscode, `{"servers":{"blaxel":{"type":"stdio","command":"/home/me/.local/bin/blaxel","args":["mcp"]}}}`)
	plan := planMCPUninstall(t, e)
	assert.Equal(t, map[string]string{cursor: "blaxel", vscode: "blaxel"}, itemDetails(plan.Remove))
	runUninstallPlan(t, plan)
	assert.JSONEq(t, `{"mcpServers":{"mine":{"url":"https://example.com"}}}`, readTestFile(t, cursor))
	// No record says this setup created the file, so it stays, empty.
	assert.JSONEq(t, `{"servers":{}}`, readTestFile(t, vscode))
}

// Setup adds Claude Code's servers with the claude CLI, which also writes an
// empty env; uninstall removes them the same way.
func TestUninstallUsesTheClaudeCLI(t *testing.T) {
	var commands []fakeCommand
	e := testMCPEnv(t.TempDir(), map[string]string{"PATH_HAS_claude": "1"}, &commands, "", nil)
	file := MCPTargets["claude-code"].File(e)
	writeTestFile(t, file, `{"numStartups":3,"mcpServers":{`+
		`"blaxel":{"type":"stdio","command":"/opt/blaxel/bin/bl","args":["mcp"],"env":{}},`+
		`"blaxel-docs":{"type":"http","url":"https://docs.blaxel.ai/mcp"}}}`)
	plan := planMCPUninstall(t, e)
	assert.Equal(t, map[string]string{file: "blaxel, blaxel-docs"}, itemDetails(plan.Remove))
	runUninstallPlan(t, plan)
	assert.Equal(t, []fakeCommand{
		{"/usr/bin/claude", []string{"mcp", "remove", "--scope", "user", "blaxel"}},
		{"/usr/bin/claude", []string{"mcp", "remove", "--scope", "user", "blaxel-docs"}},
	}, commands)
}

// Codex keeps the user's comments and formatting: only the appended tables go.
func TestUninstallRestoresTheCodexConfig(t *testing.T) {
	var commands []fakeCommand
	e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
	file := codexConfigFile(e)
	original := "# my settings\nmodel = \"gpt-5\"\n\n[mcp_servers.other]\nurl = \"https://example.com/mcp\" # mine\n"
	writeTestFile(t, file, original)
	require.NoError(t, ConfigureAgentMCP(context.Background(), e, MCPTargets["codex"], []MCPServer{testResourceServer, testDocsServer}).Err)
	runUninstallPlan(t, planMCPUninstall(t, e))
	assert.Equal(t, original, readTestFile(t, file))

	// A table in the middle of the file takes its trailing blank line along.
	writeTestFile(t, file, "[mcp_servers.blaxel-docs]\nurl = \"https://docs.blaxel.ai/mcp\"\n\n[profile]\nname = \"me\"\n")
	runUninstallPlan(t, planMCPUninstall(t, e))
	assert.Equal(t, "[profile]\nname = \"me\"\n", readTestFile(t, file))
}

func TestUninstallJSONKeepsOtherSettings(t *testing.T) {
	var commands []fakeCommand
	e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
	file := MCPTargets["gemini-cli"].File(e)
	writeTestFile(t, file, `{"theme":"dark","mcpServers":{}}`)
	require.NoError(t, ConfigureAgentMCP(context.Background(), e, MCPTargets["gemini-cli"], []MCPServer{testResourceServer, testDocsServer}).Err)
	runUninstallPlan(t, planMCPUninstall(t, e))
	assert.JSONEq(t, `{"theme":"dark","mcpServers":{}}`, readTestFile(t, file))

	// A symlinked file (from dotfiles) keeps its link.
	if runtime.GOOS != "windows" {
		cursor := MCPTargets["cursor"].File(e)
		dotfiles := filepath.Join(t.TempDir(), "mcp.json")
		writeTestFile(t, dotfiles, `{}`)
		require.NoError(t, os.MkdirAll(filepath.Dir(cursor), 0755))
		require.NoError(t, os.Symlink(dotfiles, cursor))
		require.NoError(t, ConfigureAgentMCP(context.Background(), e, MCPTargets["cursor"], []MCPServer{testResourceServer}).Err)
		runUninstallPlan(t, planMCPUninstall(t, e))
		_, err := os.Readlink(cursor)
		require.NoError(t, err)
		assert.JSONEq(t, `{"mcpServers":{}}`, readTestFile(t, dotfiles))
	}
}

// A file left empty is deleted only when setup created it. One that was there
// before setup, even empty, is the user's and stays, as does any file when the
// record of what setup created is damaged.
func TestUninstallDeletesOnlyConfigFilesSetupCreated(t *testing.T) {
	servers := []MCPServer{testResourceServer, testDocsServer}
	for _, fixture := range []struct{ name, target, before, after string }{
		{"empty JSON object", "cursor", "{}\n", "{\n  \"mcpServers\": {}\n}\n"},
		{"empty TOML file", "codex", "", ""},
	} {
		t.Run("there before setup/"+fixture.name, func(t *testing.T) {
			var commands []fakeCommand
			e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
			target := MCPTargets[fixture.target]
			writeTestFile(t, target.File(e), fixture.before)
			require.NoError(t, ConfigureAgentMCP(context.Background(), e, target, servers).Err)
			assert.NoFileExists(t, mcpCreatedPath(e.Home))
			runUninstallPlan(t, planMCPUninstall(t, e))
			assert.Equal(t, fixture.after, readTestFile(t, target.File(e)))
		})
	}
	t.Run("created by setup", func(t *testing.T) {
		var commands []fakeCommand
		e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
		target := MCPTargets["cursor"]
		require.NoError(t, ConfigureAgentMCP(context.Background(), e, target, servers[:1]).Err)
		// A later setup adds to the file and keeps the record.
		require.NoError(t, ConfigureAgentMCP(context.Background(), e, target, servers).Err)
		assert.True(t, mcpConfigCreated(e.Home, target.File(e)))
		runUninstallPlan(t, planMCPUninstall(t, e))
		assert.NoFileExists(t, target.File(e))
		assert.NoFileExists(t, mcpCreatedPath(e.Home))
	})
	t.Run("damaged record", func(t *testing.T) {
		var commands []fakeCommand
		e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
		target := MCPTargets["cursor"]
		require.NoError(t, ConfigureAgentMCP(context.Background(), e, target, servers).Err)
		writeTestFile(t, mcpCreatedPath(e.Home), "[broken")
		runUninstallPlan(t, planMCPUninstall(t, e))
		assert.JSONEq(t, `{"mcpServers":{}}`, readTestFile(t, target.File(e)))
	})
}

// An entry that changes after the plan is kept.
func TestUninstallRechecksBeforeRemoving(t *testing.T) {
	var commands []fakeCommand
	e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
	target := MCPTargets["cursor"]
	require.NoError(t, ConfigureAgentMCP(context.Background(), e, target, []MCPServer{testResourceServer}).Err)
	plan := planMCPUninstall(t, e)
	changed := `{"mcpServers":{"blaxel":{"command":"/opt/blaxel/bin/bl","args":["mcp","--verbose"]}}}`
	writeTestFile(t, target.File(e), changed)
	items, err := plan.Run(context.Background())
	require.NoError(t, err)
	require.Len(t, items, 1)
	require.ErrorIs(t, items[0].Err, errChangedSincePlan)
	assert.Equal(t, changed, readTestFile(t, target.File(e)))
}

func TestUninstallKeepsASkillEditedAfterThePlan(t *testing.T) {
	home := t.TempDir()
	installTestSkills(t, home)
	plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, SkipMCP: true})
	require.NoError(t, err)
	edited := filepath.Join(home, ".agents", "skills", "blaxel-cli")
	writeTestFile(t, filepath.Join(edited, "notes.md"), "mine")
	items, err := plan.Run(context.Background())
	require.NoError(t, err)
	for _, item := range items {
		if item.Path == edited {
			require.ErrorIs(t, item.Err, errChangedSincePlan)
		} else {
			require.NoError(t, item.Err, item.Path)
		}
	}
	assert.FileExists(t, filepath.Join(edited, "notes.md"))
	assert.NoDirExists(t, filepath.Join(home, ".agents", "skills", "blaxel-sdk"))
	lock := readTestFile(t, SkillsLockPath(home, noEnv))
	assert.Contains(t, lock, `"blaxel-cli"`, "the kept folder keeps its lock entry")
	assert.NotContains(t, lock, `"blaxel-sdk"`)
}

func installTestSkills(t *testing.T, home string) {
	t.Helper()
	for _, dir := range []string{".claude", ".codex", ".pi/agent"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, dir), 0755))
	}
	_, err := InstallSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, detectedSkillsAgents(home, noEnv), time.Now())
	require.NoError(t, err)
}

func TestUninstallRemovesInstalledSkills(t *testing.T) {
	home := t.TempDir()
	// The user's own skills, and a skill another tool recorded, stay.
	writeTestFile(t, filepath.Join(home, ".agents", "skills", "my-skill", "SKILL.md"), skillManifest("my-skill"))
	writeTestFile(t, filepath.Join(home, ".claude", "skills", "mine", "SKILL.md"), skillManifest("mine"))
	writeTestFile(t, SkillsLockPath(home, noEnv), `{"version":3,"skills":{"my-skill":{"source":"me/skills","skillFolderHash":"abc"}},"dismissed":{}}`)
	installTestSkills(t, home)
	before := snapshot(t, home)

	plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, MCP: testMCPEnv(home, nil, nil, "", nil), SkipMCP: true})
	require.NoError(t, err)
	assert.Equal(t, before, snapshot(t, home), "planning changes nothing")
	link := "link"
	if runtime.GOOS == "windows" {
		link = "copy"
	}
	agents, claude, pi := filepath.Join(home, ".agents"), filepath.Join(home, ".claude", "skills"), filepath.Join(home, ".pi", "agent", "skills")
	assert.Equal(t, map[string]string{
		filepath.Join(claude, "blaxel-cli"):           link,
		filepath.Join(claude, "blaxel-sdk"):           link,
		filepath.Join(pi, "blaxel-cli"):               link,
		filepath.Join(pi, "blaxel-sdk"):               link,
		filepath.Join(agents, "skills", "blaxel-cli"): "",
		filepath.Join(agents, "skills", "blaxel-sdk"): "",
		SkillsLockPath(home, noEnv):                   "entries for blaxel-cli and blaxel-sdk",
		skillsUpdateStatePath(home):                   "automatic updates off",
	}, itemDetails(plan.Remove))
	assert.Empty(t, plan.Keep)
	assert.True(t, plan.AutoUpdateOff)

	runUninstallPlan(t, plan)
	assert.ElementsMatch(t, []string{"my-skill"}, dirNames(t, filepath.Join(agents, "skills")))
	assert.ElementsMatch(t, []string{"mine"}, dirNames(t, claude))
	assert.Empty(t, dirNames(t, pi))
	assert.JSONEq(t, `{"version":3,"skills":{"my-skill":{"source":"me/skills","skillFolderHash":"abc"}},"dismissed":{}}`, readTestFile(t, SkillsLockPath(home, noEnv)))
	state, err := ReadSkillsUpdateState(home)
	require.NoError(t, err)
	assert.Equal(t, "disabled by bl uninstall until bl setup", SkillsUpdateDisabled(state, noEnv))

	again, err := PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, SkipMCP: true})
	require.NoError(t, err)
	assert.Empty(t, again.Remove, "a second uninstall finds nothing")
}

// A lock file that held only the Blaxel skills is deleted with them.
func TestUninstallDeletesTheLockFileItCreated(t *testing.T) {
	home := t.TempDir()
	installTestSkills(t, home)
	plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, SkipMCP: true})
	require.NoError(t, err)
	runUninstallPlan(t, plan)
	assert.NoFileExists(t, SkillsLockPath(home, noEnv))
	assert.Empty(t, dirNames(t, filepath.Join(home, ".agents", "skills")))
}

// Edited skills and skills linked from another tool (ENG-5458) are kept,
// with their agent links and lock entries.
func TestUninstallKeepsEditedAndExternallyManagedSkills(t *testing.T) {
	home := t.TempDir()
	installTestSkills(t, home)
	root := filepath.Join(home, ".agents", "skills")
	writeTestFile(t, filepath.Join(root, "blaxel-sdk", "SKILL.md"), skillManifest("blaxel-sdk")+"My notes.\n")
	external := t.TempDir()
	writeTestFile(t, filepath.Join(external, "SKILL.md"), skillManifest("blaxel-cli")+"External fork.\n")
	require.NoError(t, os.RemoveAll(filepath.Join(root, "blaxel-cli")))
	managedSkillLink(t, external, filepath.Join(root, "blaxel-cli"))
	before := snapshot(t, home)

	plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, SkipMCP: true})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		filepath.Join(root, "blaxel-cli"): "externally managed link",
		filepath.Join(root, "blaxel-sdk"): "changed since it was installed",
	}, itemDetails(plan.Keep))
	assert.Equal(t, map[string]string{skillsUpdateStatePath(home): "automatic updates off"}, itemDetails(plan.Remove))
	runUninstallPlan(t, plan)
	after := snapshot(t, home)
	delete(after, skillsUpdateStatePath(home))
	assert.Equal(t, before, after, "only automatic updates were turned off")
	assert.Equal(t, skillManifest("blaxel-cli")+"External fork.\n", readTestFile(t, filepath.Join(external, "SKILL.md")))
}

// An agent folder that is the shared one under another name is not a second
// place to remove from.
func TestUninstallSkipsAgentFoldersLinkedToTheSharedOne(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinked folders")
	}
	home := t.TempDir()
	root := filepath.Join(home, ".agents", "skills")
	require.NoError(t, os.MkdirAll(root, 0755))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0755))
	require.NoError(t, os.Symlink(root, filepath.Join(home, ".claude", "skills")))
	installTestSkills(t, home)
	plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, SkipMCP: true})
	require.NoError(t, err)
	for path := range itemDetails(plan.Remove) {
		assert.False(t, strings.HasPrefix(path, filepath.Join(home, ".claude")), path)
	}
	runUninstallPlan(t, plan)
	_, err = os.Readlink(filepath.Join(home, ".claude", "skills"))
	require.NoError(t, err, "the agent's link to the shared folder stays")
}

// A skill folder deleted by hand leaves only its record and agent links.
func TestUninstallCleansUpAfterADeletedSkill(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("dangling links")
	}
	home := t.TempDir()
	installTestSkills(t, home)
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".agents", "skills", "blaxel-sdk")))
	plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, SkipMCP: true})
	require.NoError(t, err)
	assert.Contains(t, itemDetails(plan.Remove), filepath.Join(home, ".claude", "skills", "blaxel-sdk"))
	runUninstallPlan(t, plan)
	_, err = os.Lstat(filepath.Join(home, ".claude", "skills", "blaxel-sdk"))
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestUninstallTurnsOffAutomaticMCPUpdatesOnce(t *testing.T) {
	var commands []fakeCommand
	e := testMCPEnv(t.TempDir(), map[string]string{}, &commands, "", nil)
	require.NoError(t, ConfigureAgentMCP(context.Background(), e, MCPTargets["cursor"], []MCPServer{testResourceServer, testDocsServer}).Err)
	assert.False(t, MCPAutoUpdateOff(e.Home))

	skipped, err := PlanUninstall(context.Background(), UninstallOptions{Home: e.Home, Env: e.Env, MCP: e, SkipSkills: true, SkipMCP: true})
	require.NoError(t, err)
	assert.Empty(t, skipped.Remove, "--skip-mcp keeps the servers and their updates")

	plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: e.Home, Env: e.Env, MCP: e, SkipSkills: true})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{
		MCPTargets["cursor"].File(e): "blaxel, blaxel-docs",
		mcpSetupStatePath(e.Home):    "automatic updates off",
	}, itemDetails(plan.Remove))
	runUninstallPlan(t, plan)
	assert.True(t, MCPAutoUpdateOff(e.Home))
	assert.JSONEq(t, `{"autoupdate":false}`, readTestFile(t, mcpSetupStatePath(e.Home)))
	again, err := PlanUninstall(context.Background(), UninstallOptions{Home: e.Home, Env: e.Env, MCP: e, SkipSkills: true})
	require.NoError(t, err)
	assert.Empty(t, again.Remove, "a second uninstall finds nothing")

	// bl setup turns them back on, which removes the saved state.
	require.NoError(t, SetMCPAutoUpdate(e.Home, true))
	assert.False(t, MCPAutoUpdateOff(e.Home))
	assert.NoFileExists(t, mcpSetupStatePath(e.Home))
	require.NoError(t, SetMCPAutoUpdate(e.Home, true), "turning on twice is fine")

	// A damaged state never adds servers back, and uninstall saves it again.
	writeTestFile(t, mcpSetupStatePath(e.Home), "{broken")
	assert.True(t, MCPAutoUpdateOff(e.Home))
	plan, err = PlanUninstall(context.Background(), UninstallOptions{Home: e.Home, Env: e.Env, MCP: e, SkipSkills: true})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{mcpSetupStatePath(e.Home): "automatic updates off"}, itemDetails(plan.Remove))
}

func TestUninstallTurnsOffAutomaticUpdatesOnce(t *testing.T) {
	home := t.TempDir()
	enabled := true
	require.NoError(t, WriteSkillsUpdateState(home, SkillsUpdateState{AutoUpdate: &enabled, Revision: strings.Repeat("a", 40)}))
	plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, SkipMCP: true})
	require.NoError(t, err)
	assert.Equal(t, map[string]string{skillsUpdateStatePath(home): "automatic updates off"}, itemDetails(plan.Remove))
	runUninstallPlan(t, plan)
	state, err := ReadSkillsUpdateState(home)
	require.NoError(t, err)
	assert.Equal(t, SkillsUpdateState{AutoUpdate: &enabled, Uninstalled: true}, state, "only the user's preference, and the uninstall, are kept")
	assert.True(t, SkillsAutoUpdateOffByUninstall(home))
	assert.Equal(t, "disabled by bl uninstall until bl setup", SkillsUpdateDisabled(state, noEnv))
	plan, err = PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, SkipMCP: true})
	require.NoError(t, err)
	assert.Empty(t, plan.Remove)
}

// bl setup turns automatic skills updates back on after bl uninstall turned
// them off, but never after the user turned them off, before or after the
// uninstall. Either way the uninstall holds until bl setup.
func TestResumeSkillsAutoUpdateOnlyAfterUninstall(t *testing.T) {
	disabled := false
	t.Run("uninstall", func(t *testing.T) {
		home := t.TempDir()
		plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, SkipMCP: true})
		require.NoError(t, err)
		assert.True(t, plan.AutoUpdateOff)
		runUninstallPlan(t, plan)
		require.NoError(t, ResumeSkillsAutoUpdate(context.Background(), home))
		state, err := ReadSkillsUpdateState(home)
		require.NoError(t, err)
		assert.Empty(t, SkillsUpdateDisabled(state, noEnv))
		assert.False(t, SkillsAutoUpdateOffByUninstall(home))
	})
	t.Run("user before uninstall", func(t *testing.T) {
		home := t.TempDir()
		require.NoError(t, WriteSkillsUpdateState(home, SkillsUpdateState{AutoUpdate: &disabled, Revision: strings.Repeat("a", 40)}))
		plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, SkipMCP: true})
		require.NoError(t, err)
		assert.False(t, plan.AutoUpdateOff, "updates were already off by the user's choice")
		runUninstallPlan(t, plan)
		state, err := ReadSkillsUpdateState(home)
		require.NoError(t, err)
		assert.Equal(t, SkillsUpdateState{AutoUpdate: &disabled, Uninstalled: true}, state)
		assert.True(t, SkillsAutoUpdateOffByUninstall(home), "the uninstall is recorded next to the user's choice")
		require.NoError(t, ResumeSkillsAutoUpdate(context.Background(), home))
		state, err = ReadSkillsUpdateState(home)
		require.NoError(t, err)
		assert.Equal(t, SkillsUpdateState{AutoUpdate: &disabled}, state, "the user's choice stays")
	})
	t.Run("user after uninstall", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("USERPROFILE", home)
		runUninstallPlan(t, func() UninstallPlan {
			plan, err := PlanUninstall(context.Background(), UninstallOptions{Home: home, Env: noEnv, SkipMCP: true})
			require.NoError(t, err)
			return plan
		}())
		require.NoError(t, SetSkillsAutoUpdate(context.Background(), false))
		assert.True(t, SkillsAutoUpdateOffByUninstall(home), "bl skills autoupdate off keeps the uninstall")
		require.NoError(t, ResumeSkillsAutoUpdate(context.Background(), home))
		state, err := ReadSkillsUpdateState(home)
		require.NoError(t, err)
		assert.Equal(t, SkillsUpdateState{AutoUpdate: &disabled}, state, "the user's choice stays")
	})
}

func TestCLIRemovalSteps(t *testing.T) {
	never := func(string) bool { return false }
	t.Run("install script", func(t *testing.T) {
		home := t.TempDir()
		bin := filepath.Join(home, ".local", "bin")
		writeTestFile(t, filepath.Join(bin, "bl"), "")
		writeTestFile(t, filepath.Join(bin, "blaxel"), "")
		writeTestFile(t, filepath.Join(home, ".zsh", "completions", "_bl"), "")
		writeTestFile(t, filepath.Join(home, ".zshrc"), "alias ll='ls -l'\n\n"+installerMarker+"\nexport PATH=\"$HOME/.local/bin:$PATH\"\n")
		writeTestFile(t, filepath.Join(home, ".bashrc"), "# nothing from Blaxel\n")
		steps := CLIRemovalSteps(CLIInstall{Executable: filepath.Join(bin, "bl"), Home: home, GOOS: "linux", Env: noEnv, Sudo: never})
		assert.Equal(t, []CLIRemovalStep{
			{Command: "rm ~/.local/bin/bl ~/.local/bin/blaxel"},
			{Command: "rm ~/.zsh/completions/_bl", Note: "shell completions"},
			{Note: `In ~/.zshrc, delete the lines under "# Added by the Blaxel installer"`},
			{Command: "rm -rf ~/.blaxel", Note: "your Blaxel logins and settings"},
		}, steps)
	})
	t.Run("system folder", func(t *testing.T) {
		home, bin := t.TempDir(), filepath.Join(t.TempDir(), "my bin")
		writeTestFile(t, filepath.Join(bin, "bl"), "")
		steps := CLIRemovalSteps(CLIInstall{Executable: filepath.Join(bin, "bl"), Home: home, GOOS: "darwin", Env: noEnv,
			Sudo: func(string) bool { return true }})
		assert.Equal(t, "sudo rm '"+filepath.Join(bin, "bl")+"'", steps[0].Command)
	})
	t.Run("Homebrew", func(t *testing.T) {
		prefix := t.TempDir()
		keg := filepath.Join(prefix, "Cellar", "blaxel", "1.2.3", "bin", "blaxel")
		writeTestFile(t, keg, "")
		require.NoError(t, os.MkdirAll(filepath.Join(prefix, "bin"), 0755))
		executable := keg
		if runtime.GOOS != "windows" {
			executable = filepath.Join(prefix, "bin", "bl")
			require.NoError(t, os.Symlink(keg, executable))
		}
		steps := CLIRemovalSteps(CLIInstall{Executable: executable, Home: t.TempDir(), GOOS: "darwin", Env: noEnv, Sudo: never})
		assert.Equal(t, []string{"brew uninstall blaxel", "brew untap blaxel-ai/blaxel", "rm -rf ~/.blaxel"},
			[]string{steps[0].Command, steps[1].Command, steps[2].Command})
	})
	t.Run("install.ps1", func(t *testing.T) {
		home, local := t.TempDir(), t.TempDir()
		dir := filepath.Join(local, "blaxel")
		writeTestFile(t, filepath.Join(dir, "bl.exe"), "")
		env := map[string]string{"LOCALAPPDATA": local}
		steps := CLIRemovalSteps(CLIInstall{Executable: filepath.Join(dir, "bl.exe"), Home: home, GOOS: "windows", Env: func(key string) string { return env[key] }, Sudo: never})
		require.Len(t, steps, 3)
		assert.Equal(t, "Remove-Item -Recurse -LiteralPath '"+dir+"'", steps[0].Command)
		assert.Equal(t, "[Environment]::SetEnvironmentVariable('Path', (([Environment]::GetEnvironmentVariable('Path', 'User') -split ';') | Where-Object { $_ -ne '"+dir+"' }) -join ';', 'User')", steps[1].Command)
		assert.Equal(t, "Remove-Item -Recurse -LiteralPath '"+filepath.Join(home, ".blaxel")+"'", steps[2].Command)
	})
}

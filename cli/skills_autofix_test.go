package cli

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInstallSkillsArchiveAutoFixesFlatAndNestedCopies(t *testing.T) {
	home := resolvedTempDir(t)
	root := filepath.Join(home, ".agents", "skills")
	claude := filepath.Join(home, ".claude", "skills")
	managedSkillLink(t, root, filepath.Join(home, ".pi", "agent", "skills"))
	lock := `{"version":3,"skills":{"blaxel-cli":{"source":"blaxel-ai/agent-skills","updatedAt":"old"},"blaxel-sdk":{"source":"blaxel-ai/agent-skills","updatedAt":"old"}},"dismissed":{"mine":true}}`
	writeTestFile(t, skillsLockPath(home, noEnv), lock)
	for _, name := range []string{"blaxel-cli", "blaxel-sdk"} {
		writeTestFile(t, filepath.Join(root, "blaxel", name, "SKILL.md"), skillManifest(name)+"Local fork.\n")
		writeTestFile(t, filepath.Join(root, "blaxel", name, "references", "local.md"), "local reference")
		writeTestFile(t, filepath.Join(root, name, "SKILL.md"), skillManifest(name)+"Old flat copy.\n")
		writeTestFile(t, filepath.Join(root, name, "custom.txt"), "keep flat customization")
		writeTestFile(t, filepath.Join(claude, name, "SKILL.md"), skillManifest(name)+"Old agent copy.\n")
	}
	archive := buildSkillsArchive(t, testSkillsEntries())
	result, err := installSkillsArchive(archive, home, noEnv, detectedSkillsAgents(home, noEnv), time.Now())
	require.NoError(t, err)
	assert.Empty(t, result.skills)
	assert.ElementsMatch(t, []string{"blaxel-cli", "blaxel-sdk"}, result.preserved)
	require.Len(t, result.backups, 4)
	require.Len(t, result.repaired, 4)
	for _, backup := range result.backups {
		assert.FileExists(t, filepath.Join(backup, "SKILL.md"))
		for _, scanRoot := range []string{root, claude} {
			relative, err := filepath.Rel(scanRoot, backup)
			require.NoError(t, err)
			assert.True(t, relative == ".." || len(relative) > 3 && relative[:3] == ".."+string(filepath.Separator), backup)
		}
	}
	for _, name := range []string{"blaxel-cli", "blaxel-sdk"} {
		target, err := evalSkillLinks(filepath.Join(root, name))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(root, "blaxel", name), target)
		assertSkillLink(t, home, claude, name)
		assert.Equal(t, skillManifest(name)+"Local fork.\n", readTestFile(t, filepath.Join(target, "SKILL.md")))
		assert.Equal(t, "local reference", readTestFile(t, filepath.Join(target, "references", "local.md")))
	}
	assert.Equal(t, lock, readTestFile(t, skillsLockPath(home, noEnv)))
	before := dirNames(t, filepath.Join(home, ".agents"))
	repeat, err := installSkillsArchive(archive, home, noEnv, detectedSkillsAgents(home, noEnv), time.Now())
	require.NoError(t, err)
	assert.Empty(t, repeat.repaired)
	assert.Empty(t, repeat.backups)
	assert.Equal(t, before, dirNames(t, filepath.Join(home, ".agents")))
	for _, backup := range result.backups {
		assert.FileExists(t, filepath.Join(backup, "SKILL.md"), "repeat must retain backups")
		if filepath.Dir(filepath.Dir(backup)) == filepath.Dir(root) {
			assert.Equal(t, "keep flat customization", readTestFile(t, filepath.Join(backup, "custom.txt")))
		}
	}
}

func TestInstallSkillsArchiveReconcilesMultipleNestedCopies(t *testing.T) {
	home := resolvedTempDir(t)
	root := filepath.Join(home, ".agents", "skills")
	first := filepath.Join(root, "a", "sdk")
	second := filepath.Join(root, "b", "sdk")
	writeTestFile(t, filepath.Join(first, "SKILL.md"), skillManifest("blaxel-sdk")+"First local copy.\n")
	writeTestFile(t, filepath.Join(second, "SKILL.md"), skillManifest("blaxel-sdk")+"Second local copy.\n")
	result, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, nil, time.Now())
	require.NoError(t, err)
	require.Len(t, result.backups, 1)
	assert.Equal(t, skillManifest("blaxel-sdk")+"Second local copy.\n", readTestFile(t, filepath.Join(result.backups[0], "SKILL.md")))
	for _, path := range []string{second, filepath.Join(root, "blaxel-sdk")} {
		target, err := evalSkillLinks(path)
		require.NoError(t, err)
		assert.Equal(t, first, target)
	}
	found, err := existingSkillFolders(root, map[string]bool{"blaxel-sdk": true})
	require.NoError(t, err)
	assert.Equal(t, []string{first}, found["blaxel-sdk"])
}

func TestRepairSkillLinkRestoresPreviousCopyOnFailure(t *testing.T) {
	home := resolvedTempDir(t)
	root := filepath.Join(home, "skills")
	flat := filepath.Join(root, "blaxel-cli")
	writeTestFile(t, filepath.Join(flat, "SKILL.md"), "keep original")
	_, err := repairSkillLink(skillLinkPlan{target: filepath.Join(root, "missing"), destination: flat, root: root, repair: true})
	require.Error(t, err)
	assert.Equal(t, "keep original", readTestFile(t, filepath.Join(flat, "SKILL.md")))
	assert.Equal(t, []string{"skills"}, dirNames(t, home), "failed repair leaves no backup or staging directory")
}

func TestInstallSkillsArchivePreflightsRepairsBeforeWriting(t *testing.T) {
	home := resolvedTempDir(t)
	root := filepath.Join(home, ".agents", "skills")
	writeTestFile(t, filepath.Join(root, "blaxel", "blaxel-cli", "SKILL.md"), skillManifest("blaxel-cli"))
	writeTestFile(t, filepath.Join(root, "blaxel-cli", "SKILL.md"), "old flat")
	writeTestFile(t, filepath.Join(root, "blaxel-sdk"), "not a directory")
	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, nil, time.Now())
	require.ErrorContains(t, err, "not a directory")
	assert.Equal(t, "old flat", readTestFile(t, filepath.Join(root, "blaxel-cli", "SKILL.md")))
	assert.NoFileExists(t, skillsLockPath(home, noEnv))
	assert.Equal(t, []string{"skills"}, dirNames(t, filepath.Dir(root)))
}

func TestInstallSkillsArchivePreflightsUnavailableRepairLinks(t *testing.T) {
	home := resolvedTempDir(t)
	root := filepath.Join(home, ".agents", "skills")
	nested := filepath.Join(root, "blaxel", "blaxel-sdk")
	flat := filepath.Join(root, "blaxel-sdk")
	manifest := skillManifest("blaxel-sdk") + "Retained local fork.\n"
	writeTestFile(t, filepath.Join(nested, "SKILL.md"), manifest)
	writeTestFile(t, filepath.Join(flat, "custom.txt"), "Retain duplicate")
	previous := skillDirectoryLink
	unavailable := errors.New("directory links are unavailable")
	skillDirectoryLink = func(_, _, _ string) error { return unavailable }
	t.Cleanup(func() { skillDirectoryLink = previous })

	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, func(string) string { return "" }, nil, time.Now())
	require.ErrorIs(t, err, unavailable)
	assert.Contains(t, err.Error(), "no skills were changed")
	assert.NoDirExists(t, filepath.Join(root, "blaxel-cli"), "an earlier upstream skill must not be written")
	assert.Equal(t, manifest, readTestFile(t, filepath.Join(nested, "SKILL.md")))
	assert.Equal(t, "Retain duplicate", readTestFile(t, filepath.Join(flat, "custom.txt")))
	assert.Equal(t, []string{"skills"}, dirNames(t, filepath.Join(home, ".agents")), "no probe, backup, or lock may remain")
}

func TestSetupReportsAutomaticSkillRepair(t *testing.T) {
	home := resolvedTempDir(t)
	root := filepath.Join(home, ".agents", "skills")
	writeTestFile(t, filepath.Join(root, "blaxel", "blaxel-cli", "SKILL.md"), skillManifest("blaxel-cli"))
	writeTestFile(t, filepath.Join(root, "blaxel-cli", "SKILL.md"), "old flat")
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{"LOGGED_IN": "main", "TRACKING_SET": "1"}, recorder)
	options.skipMCP, options.skipLogin = true, true
	options.installSkills = func(_ context.Context, agents []skillsAgent) (skillsInstallResult, error) {
		return installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, agents, time.Now())
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
					assert.Equal(t, "Usage and error reports", item.Label)
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

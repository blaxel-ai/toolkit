package agentsetup

import (
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
	writeTestFile(t, SkillsLockPath(home, noEnv), lock)
	for _, name := range []string{"blaxel-cli", "blaxel-sdk"} {
		writeTestFile(t, filepath.Join(root, "blaxel", name, "SKILL.md"), skillManifest(name)+"Local fork.\n")
		writeTestFile(t, filepath.Join(root, "blaxel", name, "references", "local.md"), "local reference")
		writeTestFile(t, filepath.Join(root, name, "SKILL.md"), skillManifest(name)+"Old flat copy.\n")
		writeTestFile(t, filepath.Join(root, name, "custom.txt"), "keep flat customization")
		writeTestFile(t, filepath.Join(claude, name, "SKILL.md"), skillManifest(name)+"Old agent copy.\n")
	}
	archive := buildSkillsArchive(t, testSkillsEntries())
	result, err := InstallSkillsArchive(archive, home, noEnv, detectedSkillsAgents(home, noEnv), time.Now())
	require.NoError(t, err)
	assert.Empty(t, result.Skills)
	assert.ElementsMatch(t, []string{"blaxel-cli", "blaxel-sdk"}, result.Preserved)
	require.Len(t, result.Backups, 4)
	require.Len(t, result.Repaired, 4)
	for _, backup := range result.Backups {
		assert.FileExists(t, filepath.Join(backup, "SKILL.md"))
		for _, scanRoot := range []string{root, claude} {
			relative, err := filepath.Rel(scanRoot, backup)
			require.NoError(t, err)
			assert.True(t, relative == ".." || len(relative) > 3 && relative[:3] == ".."+string(filepath.Separator), backup)
		}
	}
	for _, name := range []string{"blaxel-cli", "blaxel-sdk"} {
		target, err := EvalSkillLinks(filepath.Join(root, name))
		require.NoError(t, err)
		assert.Equal(t, filepath.Join(root, "blaxel", name), target)
		assertSkillLink(t, home, claude, name)
		assert.Equal(t, skillManifest(name)+"Local fork.\n", readTestFile(t, filepath.Join(target, "SKILL.md")))
		assert.Equal(t, "local reference", readTestFile(t, filepath.Join(target, "references", "local.md")))
	}
	assert.Equal(t, lock, readTestFile(t, SkillsLockPath(home, noEnv)))
	before := dirNames(t, filepath.Join(home, ".agents"))
	repeat, err := InstallSkillsArchive(archive, home, noEnv, detectedSkillsAgents(home, noEnv), time.Now())
	require.NoError(t, err)
	assert.Empty(t, repeat.Repaired)
	assert.Empty(t, repeat.Backups)
	assert.Equal(t, before, dirNames(t, filepath.Join(home, ".agents")))
	for _, backup := range result.Backups {
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
	result, err := InstallSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, nil, time.Now())
	require.NoError(t, err)
	require.Len(t, result.Backups, 1)
	assert.Equal(t, skillManifest("blaxel-sdk")+"Second local copy.\n", readTestFile(t, filepath.Join(result.Backups[0], "SKILL.md")))
	for _, path := range []string{second, filepath.Join(root, "blaxel-sdk")} {
		target, err := EvalSkillLinks(path)
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
	_, err := InstallSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, nil, time.Now())
	require.ErrorContains(t, err, "not a directory")
	assert.Equal(t, "old flat", readTestFile(t, filepath.Join(root, "blaxel-cli", "SKILL.md")))
	assert.NoFileExists(t, SkillsLockPath(home, noEnv))
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

	_, err := InstallSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, func(string) string { return "" }, nil, time.Now())
	require.ErrorIs(t, err, unavailable)
	assert.Contains(t, err.Error(), "no skills were changed")
	assert.NoDirExists(t, filepath.Join(root, "blaxel-cli"), "an earlier upstream skill must not be written")
	assert.Equal(t, manifest, readTestFile(t, filepath.Join(nested, "SKILL.md")))
	assert.Equal(t, "Retain duplicate", readTestFile(t, filepath.Join(flat, "custom.txt")))
	assert.Equal(t, []string{"skills"}, dirNames(t, filepath.Join(home, ".agents")), "no probe, backup, or lock may remain")
}

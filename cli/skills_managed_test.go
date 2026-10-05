package cli

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// resolvedTempDir returns t.TempDir() with links and Windows 8.3 short names
// (C:\Users\RUNNER~1) expanded, matching the resolved paths in error messages.
func resolvedTempDir(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	require.NoError(t, err)
	return dir
}

func managedSkillLink(t *testing.T, target, link string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("directory links need developer mode on Windows")
	}
	require.NoError(t, os.MkdirAll(filepath.Dir(link), 0755))
	require.NoError(t, os.Symlink(target, link))
}

func TestInstallSkillsArchivePreservesManagedProjection(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".agents", "skills")
	lockFile := skillsLockPath(home, noEnv)
	lock := `{"version":3,"skills":{"blaxel-cli":{"source":"blaxel-ai/agent-skills","updatedAt":"old"}},"dismissed":{"mine":true}}`
	writeTestFile(t, lockFile, lock)
	for _, name := range []string{"blaxel-cli", "blaxel-sdk"} {
		target := filepath.Join(root, "blaxel", name)
		writeTestFile(t, filepath.Join(target, "SKILL.md"), skillManifest(name)+"Local fork; do not replace.\n")
		writeTestFile(t, filepath.Join(target, "references", "local.md"), "local reference")
		managedSkillLink(t, filepath.Join("blaxel", name), filepath.Join(root, name))
	}
	managedSkillLink(t, root, filepath.Join(home, ".pi", "agent", "skills"))
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".claude"), 0755))

	for range 2 {
		result, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, detectedSkillsAgents(home, noEnv), time.Now())
		require.NoError(t, err)
		assert.Empty(t, result.skills, "externally managed skills are not reported as upstream installations")
		for _, name := range []string{"blaxel-cli", "blaxel-sdk"} {
			link, err := os.Readlink(filepath.Join(root, name))
			require.NoError(t, err, "the projection must remain a symlink")
			assert.Equal(t, filepath.Join("blaxel", name), link)
			assert.Equal(t, skillManifest(name)+"Local fork; do not replace.\n", readTestFile(t, filepath.Join(root, name, "SKILL.md")))
			assert.Equal(t, "local reference", readTestFile(t, filepath.Join(root, name, "references", "local.md")))
			assertSkillLink(t, home, filepath.Join(home, ".claude", "skills"), name)
		}
		assert.Equal(t, lock, readTestFile(t, lockFile), "do not claim upstream ownership or change its recorded hash/time")
	}
}

func TestInstallSkillsArchivePreservesExternalSkillLink(t *testing.T) {
	home, external := t.TempDir(), t.TempDir()
	root := filepath.Join(home, ".agents", "skills")
	writeTestFile(t, filepath.Join(external, "SKILL.md"), skillManifest("blaxel-cli")+"External fork.\n")
	managedSkillLink(t, external, filepath.Join(root, "blaxel-cli"))

	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, nil, time.Now())
	require.NoError(t, err)
	_, err = os.Readlink(filepath.Join(root, "blaxel-cli"))
	require.NoError(t, err)
	assert.Equal(t, skillManifest("blaxel-cli")+"External fork.\n", readTestFile(t, filepath.Join(external, "SKILL.md")))
	lock := readLock(t, skillsLockPath(home, noEnv))
	entries, ok := lock["skills"].(map[string]any)
	require.True(t, ok)
	assert.NotContains(t, entries, "blaxel-cli", "a kept external skill must not be registered for upstream updates")
	assert.Contains(t, entries, "blaxel-sdk", "unmanaged skills are installed normally")
}

func TestInstallSkillsArchiveRejectsNestedNameCollisionBeforeWriting(t *testing.T) {
	for _, name := range []string{"blaxel-cli", "blaxel-sdk"} {
		t.Run(name, func(t *testing.T) {
			home := resolvedTempDir(t)
			root := filepath.Join(home, ".agents", "skills")
			nested := filepath.Join(root, "blaxel", name)
			writeTestFile(t, filepath.Join(nested, "SKILL.md"), skillManifest(name))
			_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, nil, time.Now())
			require.ErrorContains(t, err, name)
			assert.Contains(t, err.Error(), nested)
			assert.NoDirExists(t, filepath.Join(root, "blaxel-cli"))
			assert.NoDirExists(t, filepath.Join(root, "blaxel-sdk"))
			assert.NoFileExists(t, skillsLockPath(home, noEnv), "preflight must happen before any skill or lock write")
		})
	}
}

func TestInstallSkillsArchiveRejectsBrokenOrMismatchedManagedLink(t *testing.T) {
	for _, manifest := range []string{"", skillManifest("another-skill")} {
		t.Run(manifest, func(t *testing.T) {
			home, target := t.TempDir(), filepath.Join(t.TempDir(), "skill")
			if manifest != "" {
				writeTestFile(t, filepath.Join(target, "SKILL.md"), manifest)
			}
			link := filepath.Join(home, ".agents", "skills", "blaxel-cli")
			managedSkillLink(t, target, link)
			_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, nil, time.Now())
			require.Error(t, err)
			got, err := os.Readlink(link)
			require.NoError(t, err)
			assert.Equal(t, target, got, "never erase a broken or differently named external link")
			assert.NoDirExists(t, filepath.Join(home, ".agents", "skills", "blaxel-sdk"))
		})
	}
}

func TestInstallSkillsArchiveRejectsAgentManagedLinkBeforeWriting(t *testing.T) {
	home, external := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(external, "SKILL.md"), skillManifest("blaxel-sdk")+"Agent-specific fork.\n")
	link := filepath.Join(home, ".claude", "skills", "blaxel-sdk")
	managedSkillLink(t, external, link)
	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, detectedSkillsAgents(home, noEnv), time.Now())
	require.ErrorContains(t, err, "blaxel-sdk")
	got, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, external, got)
	assert.NoDirExists(t, filepath.Join(home, ".agents", "skills"), "a later skill conflict cannot leave an earlier skill installed")
	assert.NoFileExists(t, skillsLockPath(home, noEnv))
}

func TestInstallSkillsArchiveRejectsAgentNestedNameCollision(t *testing.T) {
	home := resolvedTempDir(t)
	nested := filepath.Join(home, ".claude", "skills", "custom", "sdk")
	writeTestFile(t, filepath.Join(nested, "SKILL.md"), skillManifest("blaxel-sdk"))
	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, detectedSkillsAgents(home, noEnv), time.Now())
	require.ErrorContains(t, err, "blaxel-sdk")
	assert.Contains(t, err.Error(), nested)
	assert.NoDirExists(t, filepath.Join(home, ".agents", "skills"))
	assert.NoDirExists(t, filepath.Join(home, ".claude", "skills", "blaxel-sdk"))
}

func TestInstallSkillsArchiveChecksProjectedNamespace(t *testing.T) {
	home, external := t.TempDir(), t.TempDir()
	root := filepath.Join(home, ".agents", "skills")
	writeTestFile(t, filepath.Join(external, "sdk", "SKILL.md"), skillManifest("blaxel-sdk"))
	managedSkillLink(t, external, filepath.Join(root, "vendor"))
	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, nil, time.Now())
	require.ErrorContains(t, err, "blaxel-sdk")
	assert.NoDirExists(t, filepath.Join(root, "blaxel-cli"))
	assert.NoFileExists(t, skillsLockPath(home, noEnv))
}

func TestInstallSkillsArchiveDoesNotTraverseNamespaceCycles(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, ".agents", "skills")
	require.NoError(t, os.MkdirAll(root, 0755))
	managedSkillLink(t, root, filepath.Join(root, "loop"))
	_, err := installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, nil, time.Now())
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(root, "blaxel-sdk", "SKILL.md"))
}

func TestSetupReportsPreservedManagedSkills(t *testing.T) {
	home, external := t.TempDir(), t.TempDir()
	root := filepath.Join(home, ".agents", "skills")
	for _, name := range []string{"blaxel-cli", "blaxel-sdk"} {
		target := filepath.Join(external, name)
		writeTestFile(t, filepath.Join(target, "SKILL.md"), skillManifest(name))
		managedSkillLink(t, target, filepath.Join(root, name))
	}
	require.NoError(t, os.MkdirAll(filepath.Join(home, ".pi", "agent"), 0755))
	recorder := &setupRecorder{}
	options := testSetupOptions(t, home, map[string]string{"LOGGED_IN": "main", "TRACKING_SET": "1"}, recorder)
	options.skipMCP, options.skipLogin = true, true
	options.installSkills = func(_ context.Context, agents []skillsAgent) (skillsInstallResult, error) {
		return installSkillsArchive(buildSkillsArchive(t, testSkillsEntries()), home, noEnv, agents, time.Now())
	}
	require.NoError(t, runSetup(context.Background(), options))
	text := recorder.text(t)
	assert.Contains(t, text, "kept externally managed: blaxel-cli, blaxel-sdk")
	assert.Contains(t, text, "Kept managed")
	assert.Contains(t, text, "links and contents unchanged")
	assert.Contains(t, text, "Pi")
	assert.NoFileExists(t, skillsLockPath(home, noEnv))
}

func TestManagedSkillsInstallMessages(t *testing.T) {
	managed := skillsInstallResult{preserved: []string{"blaxel-cli", "blaxel-sdk"}}
	message := skillsInstalledMessage(managed)
	assert.Contains(t, message, "Kept externally managed")
	assert.NotContains(t, message, "skills installed")
	assert.Equal(t, "kept externally managed: blaxel-cli, blaxel-sdk", skillsInstallDetail(managed))
	managed.skills = []string{"new-skill"}
	assert.Contains(t, skillsInstalledMessage(managed), "Blaxel skills installed")
	assert.Equal(t, "new-skill · kept externally managed: blaxel-cli, blaxel-sdk", skillsInstallDetail(managed))
}

func TestSkillFolderHelpersRejectExternalLinks(t *testing.T) {
	home, external := t.TempDir(), t.TempDir()
	writeTestFile(t, filepath.Join(external, "SKILL.md"), skillManifest("blaxel-cli"))
	link := filepath.Join(home, "blaxel-cli")
	managedSkillLink(t, external, link)
	assert.ErrorContains(t, replaceSkillFolder(link, nil), "left unchanged")
	canonical := filepath.Join(home, "canonical")
	writeTestFile(t, filepath.Join(canonical, "SKILL.md"), skillManifest("blaxel-cli"))
	assert.ErrorContains(t, linkSkillFolder(canonical, link, nil), "left unchanged")
	target, err := os.Readlink(link)
	require.NoError(t, err)
	assert.Equal(t, external, target)
}

func TestInstallSkillsArchiveRejectsArchiveNameCollision(t *testing.T) {
	home := t.TempDir()
	entries := []archiveEntry{
		{name: "skills/a/SKILL.md", body: skillManifest("Blaxel SDK")},
		{name: "skills/b/SKILL.md", body: skillManifest("blaxel-sdk")},
	}
	_, err := installSkillsArchive(buildSkillsArchive(t, entries), home, noEnv, nil, time.Now())
	require.Error(t, err)
	assert.Empty(t, dirNames(t, home))
}

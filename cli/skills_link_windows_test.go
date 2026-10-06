package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWindowsSkillDirectoryJunction(t *testing.T) {
	home := resolvedTempDir(t)
	target := filepath.Join(home, "custom & skills", "日本語")
	writeTestFile(t, filepath.Join(target, "SKILL.md"), skillManifest("blaxel-sdk"))
	staging := filepath.Join(home, "staging")
	require.NoError(t, os.Mkdir(staging, 0755))
	link := filepath.Join(staging, "new")
	// The relative name is deliberately wrong: Windows junctions must keep
	// their absolute target when the staged directory is renamed.
	require.NoError(t, createSkillDirectoryLink(target, "wrong-relative-target", link))
	final := filepath.Join(home, "agent", "blaxel-sdk")
	require.NoError(t, os.MkdirAll(filepath.Dir(final), 0755))
	require.NoError(t, os.Rename(link, final))
	info, err := os.Lstat(final)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&os.ModeSymlink, "a junction must be recognized as a projection")
	resolved, err := filepath.EvalSymlinks(final)
	require.NoError(t, err)
	assert.Equal(t, target, resolved)
	assert.Equal(t, skillManifest("blaxel-sdk"), readTestFile(t, filepath.Join(final, "SKILL.md")))
	// Removing the projection must not traverse or delete the retained skill.
	require.NoError(t, os.RemoveAll(final))
	assert.FileExists(t, filepath.Join(target, "SKILL.md"))
}

func TestWindowsSkillJunctionFailureLeavesNoEmptyFolder(t *testing.T) {
	home := resolvedTempDir(t)
	link := filepath.Join(home, "link")
	err := createSkillDirectoryLink("bad\x00target", "", link)
	require.Error(t, err)
	assert.NoDirExists(t, link)
	assert.Empty(t, dirNames(t, home))
}

package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A deploy rebuilds metadata.labels from scratch, so anything not declared in
// blaxel.toml is dropped on every deploy. That silently defeats labels the
// platform reads at deploy time — a resource labelled through the API loses it
// the next time the CLI runs.
func TestGenerateDeploymentKeepsDeclaredLabels(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "blaxel.toml"), []byte(`
type = "sandbox"
name = "labelled-sandbox"

[labels]
"x-blaxel-builder" = "sandbox"
team = "platform"
`), 0o600))

	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
		core.ResetConfig()
	})

	core.ResetConfig()
	core.ReadConfigToml(".", true)
	require.Equal(t, map[string]string{
		"x-blaxel-builder": "sandbox",
		"team":             "platform",
	}, core.GetConfig().Labels, "labels were not read from blaxel.toml")

	d := &Deployment{name: "labelled-sandbox"}
	meta, ok := d.GenerateDeployment(false).Metadata.(map[string]interface{})
	require.True(t, ok, "metadata is not a map")
	labels, ok := meta["labels"].(map[string]interface{})
	require.True(t, ok, "metadata.labels is missing")

	assert.Equal(t, "sandbox", labels["x-blaxel-builder"])
	assert.Equal(t, "platform", labels["team"])
	// The CLI's own label still gets set alongside them.
	assert.Equal(t, "true", labels["x-blaxel-auto-generated"])
}

// The CLI owns these two, so a manifest must not be able to claim them.
func TestGenerateDeploymentOwnLabelsWin(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "blaxel.toml"), []byte(`
type = "sandbox"
name = "liar"

[labels]
"x-blaxel-auto-generated" = "false"
`), 0o600))

	cwd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
		core.ResetConfig()
	})

	core.ResetConfig()
	core.ReadConfigToml(".", true)

	d := &Deployment{name: "liar"}
	meta := d.GenerateDeployment(false).Metadata.(map[string]interface{})
	labels := meta["labels"].(map[string]interface{})
	assert.Equal(t, "true", labels["x-blaxel-auto-generated"])
}

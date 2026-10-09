package deploy

import (
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveConfigFile(t *testing.T) {
	root := t.TempDir()
	writeDockerfileFixture(t, root, "blaxel-v2.toml", "name = \"v2\"\n")
	writeDockerfileFixture(t, root, "project/blaxel-v2.toml", "name = \"v2\"\n")
	writeDockerfileFixture(t, root, "configs/dev.toml", "name = \"dev\"\n")

	require.NoError(t, ResolveConfigFile(root, "", "configs/dev.toml"))
	require.NoError(t, ResolveConfigFile(root, "project", "blaxel-v2.toml"))
	for _, tt := range []struct{ folder, path, want string }{
		{"", "", "--config must not be empty"},
		{"", "missing.toml", `Config file "missing.toml" not found in `},
		{"project", "configs/dev.toml", "not found in"}, // relative to -d, not the cwd
	} {
		err := ResolveConfigFile(root, tt.folder, tt.path)
		require.ErrorContains(t, err, tt.want, tt.path)
		assert.True(t, core.IsExpectedCLIError(err))
	}
}

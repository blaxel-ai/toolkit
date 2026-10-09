package deploy

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeDockerfileFixture(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o600))
}

func TestDockerfilePrecedence(t *testing.T) {
	root := t.TempDir()
	for _, path := range []string{"Dockerfile", "custom", "flag"} {
		writeDockerfileFixture(t, root, path, path)
	}
	for _, tt := range []struct{ name, flag, toml, want string }{
		{"default", "", "", ""},
		{"toml", "", "custom", "custom"},
		{"flag over toml", "flag", "custom", "flag"},
		{"flag over missing toml", "flag", "missing", "flag"},
		{"explicit default", "Dockerfile", "custom", "Dockerfile"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			config := core.Config{Build: &core.BuildConfig{Dockerfile: tt.toml}}
			selected, err := ResolveDockerfile(root, "", tt.flag, config, true, true)
			require.NoError(t, err)
			if tt.want == "" {
				assert.Nil(t, selected)
			} else {
				assert.Equal(t, tt.want, filepath.Base(selected.Path))
			}
		})
	}
}

func TestDockerfilePaths(t *testing.T) {
	root := t.TempDir()
	writeDockerfileFixture(t, root, "nested dir/custom", "FROM scratch")
	writeDockerfileFixture(t, root, "project/custom", "FROM scratch")
	for _, path := range []string{"nested dir/custom", "./nested dir/../nested dir/custom"} {
		_, err := resolveProjectDockerfile(root, path)
		require.NoError(t, err, path)
	}
	selected, err := ResolveDockerfile(root, "project", "custom", core.Config{}, true, true)
	require.NoError(t, err)
	assert.Contains(t, selected.Path, filepath.Join("project", "custom"), "relative to -d")

	_, err = resolveProjectDockerfile(root, "missing")
	require.EqualError(t, err, fmt.Sprintf("Dockerfile \"missing\" not found in %s", root))

	for _, path := range []string{"missing", "nested dir", "../custom", filepath.Join(root, "custom"), "/Dockerfile", `\Dockerfile`, `C:\Dockerfile`, `\\server\share\Dockerfile`} {
		_, err := resolveProjectDockerfile(root, path)
		require.Error(t, err, path)
		assert.True(t, core.IsExpectedCLIError(err), path)
	}
}

func TestDockerfileSymlinks(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	outside := filepath.Join(parent, "project-sibling")
	writeDockerfileFixture(t, root, "real/custom", "inside")
	writeDockerfileFixture(t, outside, "custom", "outside")
	for _, tt := range []struct {
		name, target, path string
		valid              bool
	}{
		{"inside", filepath.Join(root, "real/custom"), "inside", true},
		{"escape", filepath.Join(outside, "custom"), "escape", false},
		{"ancestor", outside, "ancestor/custom", false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.Symlink(tt.target, filepath.Join(root, tt.name)); err != nil {
				t.Skipf("symlinks unavailable: %v", err)
			}
			_, err := resolveProjectDockerfile(root, tt.path)
			if tt.valid {
				require.NoError(t, err)
			} else {
				require.Error(t, err)
				assert.True(t, core.IsExpectedCLIError(err))
			}
		})
	}
}

// The selected file's .dockerignore companion follows the same containment rules.
func TestDockerfileCompanion(t *testing.T) {
	outside := filepath.Join(t.TempDir(), "ignore")
	require.NoError(t, os.WriteFile(outside, []byte("outside"), 0o600))
	for _, target := range []string{"", "missing", outside} { // directory, broken link, escaping link
		root := t.TempDir()
		writeDockerfileFixture(t, root, "custom", "FROM scratch")
		companion := filepath.Join(root, "custom.dockerignore")
		if target == "" {
			require.NoError(t, os.Mkdir(companion, 0o755))
		} else if err := os.Symlink(target, companion); err != nil {
			t.Skipf("symlinks unavailable: %v", err)
		}
		_, err := ResolveDockerfile(root, "", "custom", core.Config{}, true, false)
		require.ErrorContains(t, err, "custom.dockerignore", target)
		assert.True(t, core.IsExpectedCLIError(err))
	}
}

func TestDockerfileModesAndRecursion(t *testing.T) {
	root := t.TempDir()
	writeDockerfileFixture(t, root, "custom", "FROM scratch")
	// The flag needs a source build; a TOML selector is inert without one.
	_, err := ResolveDockerfile(root, "", "missing", core.Config{}, false, false)
	require.ErrorContains(t, err, "requires a source build")
	selected, err := ResolveDockerfile(root, "", "", core.Config{Build: &core.BuildConfig{Dockerfile: "missing"}}, false, true)
	require.NoError(t, err)
	assert.Nil(t, selected)
	// A recursive deploy of child packages cannot share one selection.
	for _, config := range []core.Config{
		{Agent: map[string]core.Package{"child": {Path: "child"}}},
		{Function: map[string]core.Package{"child": {Path: "child"}}, SkipRoot: true},
	} {
		for _, flag := range []string{"custom", ""} {
			config.Build = &core.BuildConfig{Dockerfile: "custom"}
			_, err := ResolveDockerfile(root, "", flag, config, true, true)
			require.ErrorContains(t, err, "--recursive=false")
			_, err = ResolveDockerfile(root, "", flag, config, true, false)
			require.NoError(t, err)
			_, err = ResolveDockerfile(root, ".", flag, config, true, true)
			require.NoError(t, err)
		}
		config.Build = nil
		selected, err := ResolveDockerfile(root, "", "", config, true, true)
		require.NoError(t, err)
		assert.Nil(t, selected, "default recursion is unchanged")
	}
}

func TestDockerfileUsesServerEnv(t *testing.T) {
	root := t.TempDir()
	for _, pattern := range []string{"HOST", "PORT", "BL_SERVER_HOST", "BL_SERVER_PORT", "nothing"} {
		writeDockerfileFixture(t, root, "blaxel.Dockerfile", "ENV "+pattern+"=value")
		selected, err := ResolveDockerfile(root, "", "blaxel.Dockerfile", core.Config{}, true, false)
		require.NoError(t, err)
		assert.Equal(t, pattern != "nothing", selected.UsesServerEnv(), pattern)
	}
	var unselected *Dockerfile
	assert.False(t, unselected.UsesServerEnv())
}

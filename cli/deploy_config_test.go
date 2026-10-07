package cli

import (
	"cmp"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/deploy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Two configs and two Dockerfiles share one directory and one build context.
func TestDeployConfigNamedLayout(t *testing.T) {
	for _, folder := range []string{"", "project"} {
		root := t.TempDir()
		t.Chdir(root)
		core.ResetConfig()
		t.Cleanup(core.ResetConfig)
		fixture := map[string]string{
			"blaxel.toml":    "name = \"app\"\ntype = \"sandbox\"\n",
			"blaxel-v2.toml": "name = \"app-v2\"\ntype = \"sandbox\"\n[build]\ndockerfile = \"Dockerfile.v2\"\n",
			"Dockerfile":     "FROM scratch\n# v1\n",
			"Dockerfile.v2":  "FROM scratch\n# v2\n",
			"shared.txt":     "shared COPY source",
		}
		for path, content := range fixture {
			writeDockerfileFixture(t, root, filepath.Join(folder, path), content)
		}
		for _, tt := range []struct{ config, name, dockerfile string }{
			{"", "app", "Dockerfile"},
			{"blaxel-v2.toml", "app-v2", "Dockerfile.v2"},
		} {
			core.ResetConfig()
			require.NoError(t, core.ReadConfigTomlFile(folder, tt.config, false))
			cfg := core.GetConfig()
			assert.Equal(t, tt.name, cfg.Name)
			selected, err := deploy.ResolveDockerfile(root, folder, "", cfg, true, false)
			require.NoError(t, err)
			files := dockerfileZipContents(t, &Deployment{cwd: root, folder: folder, dockerfile: selected, configFile: tt.config})
			assert.Equal(t, []string{fixture[tt.dockerfile]}, files["Dockerfile"], "folder=%q config=%q", folder, tt.config)
			assert.Equal(t, []string{fixture["shared.txt"]}, files[filepath.ToSlash(filepath.Join(folder, "shared.txt"))])
			// The builder reads the last blaxel.toml entry, so it must be the config the CLI read.
			configFile := cmp.Or(tt.config, "blaxel.toml")
			assert.Equal(t, fixture[configFile], files["blaxel.toml"][len(files["blaxel.toml"])-1], "folder=%q config=%q", folder, tt.config)
		}
	}
}

func TestDeployConfigVolumeTemplateExcludesConfig(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	core.ResetConfig()
	t.Cleanup(core.ResetConfig)
	writeDockerfileFixture(t, root, "blaxel.toml", "type = \"sandbox\"\n")
	writeDockerfileFixture(t, root, "volume.toml", "type = \"volume-template\"\n")
	writeDockerfileFixture(t, root, "data.txt", "volume data")
	require.NoError(t, core.ReadConfigTomlFile("", "volume.toml", false))
	require.True(t, core.IsVolumeTemplate(core.GetConfig().Type))

	d := Deployment{cwd: root, configFile: "volume.toml"}
	require.NoError(t, d.Tar())
	t.Cleanup(func() { _ = os.Remove(d.archive.Name()) })
	files, err := collectDryRunTarFiles(d.archive.Name())
	require.NoError(t, err)
	assert.Equal(t, []dryRunFile{{Name: "data.txt", Size: 11}}, files)
}

// Hints about where to add settings name the config file that is actually read.
func TestDeployConfigHintNamesFile(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	writeDockerfileFixture(t, root, "requirements.txt", "requests\n")
	d := Deployment{cwd: root, configFile: "blaxel-v2.toml"}
	warning := d.validateDeploymentConfig(core.Config{Type: "agent"})
	assert.Contains(t, warning, "blaxel-v2.toml")
	assert.NotContains(t, warning, "blaxel.toml")
	d.configFile = ""
	assert.Contains(t, d.validateDeploymentConfig(core.Config{Type: "agent"}), "blaxel.toml")
}

// An explicit --config that is empty, missing, outside the project or does not
// parse must fail before any API call or archive, while a broken default
// blaxel.toml keeps warning and deploying.
func TestDeployConfigProcess(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	for _, tt := range []struct {
		name, want string
		args       []string
	}{
		{"empty", "--config must not be empty", []string{"--config", ""}},
		{"missing", `Config file "missing.toml" not found in`, []string{"--config", "missing.toml"}},
		{"outside", "must be a relative path inside the project directory", []string{"--config", "../blaxel.toml"}},
		{"unparsable", "config file bad.toml is not valid: toml:", []string{"--config", "bad.toml"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			writeDockerfileFixture(t, root, "blaxel.toml", "name = \"config-test\"\ntype = \"sandbox\"\n")
			writeDockerfileFixture(t, root, "bad.toml", "name = \"bad\"\ntype = \n")
			writeDockerfileFixture(t, root, "Dockerfile", "FROM ghcr.io/blaxel-ai/sandbox:latest\n")
			requests.Store(0)
			stdout, stderr, code, tmp := runDeployProcess(t, root, server.URL, tt.args...)
			assert.Equal(t, 1, code)
			assert.Empty(t, stdout, "structured stdout must remain empty")
			assert.Contains(t, stderr, tt.want)
			assert.Zero(t, requests.Load(), "validation must precede API calls")
			assert.Empty(t, tmp, "validation must precede archive generation")
		})
	}

	t.Run("broken default blaxel.toml still warns and continues", func(t *testing.T) {
		root := t.TempDir()
		writeDockerfileFixture(t, root, "blaxel.toml", "name = \"default\"\ntype = \n")
		writeDockerfileFixture(t, root, "Dockerfile", "FROM ghcr.io/blaxel-ai/sandbox:latest\n")
		_, stderr, code, _ := runDeployProcess(t, root, server.URL, "--dryrun", "--recursive=false")
		assert.Zero(t, code)
		assert.Contains(t, stderr, "blaxel.toml Configuration Warning")
		assert.NotContains(t, stderr, "is not valid")
	})
}

package cli

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sync/atomic"
	"testing"
	"time"

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

// runDeployProcess runs `bl deploy args...` in root as a subprocess against a stub
// API and returns its output, exit code and the entries left in its private TMPDIR.
func runDeployProcess(t *testing.T, root, apiURL string, args ...string) (stdout, stderr string, code int, tmpEntries []os.DirEntry) {
	t.Helper()
	home, tmp := t.TempDir(), t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestDeployProcessHelper$", "--", "deploy", "--yes", "--skip-version-warning", "-w", "test-workspace", "-o", "json"}, args...)...)
	cmd.Dir = root
	cmd.Env = []string{
		"BLAXEL_TEST_DEPLOY_PROCESS=1", "HOME=" + home, "USERPROFILE=" + home,
		"TMPDIR=" + tmp, "TMP=" + tmp, "TEMP=" + tmp,
		"BL_API_KEY=test-api-key", "BL_API_URL=" + apiURL, "BL_RUN_URL=" + apiURL,
	}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	var exitErr *exec.ExitError
	require.ErrorAs(t, cmd.Run(), &exitErr, "stdout: %s; stderr: %s", &out, &errOut)
	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	return out.String(), errOut.String(), exitErr.ExitCode(), entries
}

func TestDeployProcessHelper(t *testing.T) {
	if os.Getenv("BLAXEL_TEST_DEPLOY_PROCESS") != "1" {
		t.Skip("helper only runs in a subprocess")
	}
	os.Args = append([]string{"bl"}, os.Args[slices.Index(os.Args, "--")+1:]...)
	if err := core.Execute("dev", "", ""); err != nil {
		core.ExitWithError(err)
	}
	os.Exit(0)
}

// An explicitly empty --dockerfile must fail before any API call or archive,
// not silently fall back to the default Dockerfile or the TOML selector.
func TestDeployDockerfileEmptyFlag(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	root := t.TempDir()
	writeDockerfileFixture(t, root, "blaxel.toml", "name = \"empty-flag-test\"\ntype = \"sandbox\"\n[build]\ndockerfile = \"custom\"\n")
	writeDockerfileFixture(t, root, "Dockerfile", "FROM ghcr.io/blaxel-ai/sandbox:latest\n")
	writeDockerfileFixture(t, root, "custom", "FROM ghcr.io/blaxel-ai/sandbox:latest\n")
	for _, form := range [][]string{{"--dockerfile", ""}, {"--dockerfile="}} {
		stdout, stderr, code, tmp := runDeployProcess(t, root, server.URL, append([]string{"--recursive=false"}, form...)...)
		assert.Equal(t, 1, code, form)
		assert.Empty(t, stdout, "structured stdout must remain empty")
		assert.Contains(t, stderr, "--dockerfile must not be empty")
		assert.Zero(t, requests.Load(), "validation must precede API calls")
		assert.Empty(t, tmp, "validation must precede archive generation")
	}
}

func TestDeployDockerfilePrecedence(t *testing.T) {
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
			selected, err := resolveDeployDockerfile(root, "", tt.flag, config, false, true)
			require.NoError(t, err)
			if tt.want == "" {
				assert.Nil(t, selected)
			} else {
				assert.Equal(t, tt.want, filepath.Base(selected.path))
			}
		})
	}
}

func TestDeployDockerfilePaths(t *testing.T) {
	root := t.TempDir()
	writeDockerfileFixture(t, root, "nested dir/custom", "FROM scratch")
	writeDockerfileFixture(t, root, "project/custom", "FROM scratch")
	for _, path := range []string{"nested dir/custom", "./nested dir/../nested dir/custom"} {
		_, err := resolveProjectDockerfile(root, path)
		require.NoError(t, err, path)
	}
	selected, err := resolveDeployDockerfile(root, "project", "custom", core.Config{}, false, true)
	require.NoError(t, err)
	assert.Contains(t, selected.path, filepath.Join("project", "custom"), "relative to -d")

	_, err = resolveProjectDockerfile(root, "missing")
	require.EqualError(t, err, fmt.Sprintf("Dockerfile \"missing\" not found in %s", root))

	for _, path := range []string{"missing", "nested dir", "../custom", filepath.Join(root, "custom"), "/Dockerfile", `\Dockerfile`, `C:\Dockerfile`, `\\server\share\Dockerfile`} {
		_, err := resolveProjectDockerfile(root, path)
		require.Error(t, err, path)
		assert.True(t, core.IsExpectedCLIError(err), path)
	}
}

func TestDeployDockerfileSymlinks(t *testing.T) {
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
func TestDeployDockerfileCompanion(t *testing.T) {
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
		_, err := resolveDeployDockerfile(root, "", "custom", core.Config{}, false, false)
		require.ErrorContains(t, err, "custom.dockerignore", target)
		assert.True(t, core.IsExpectedCLIError(err))
	}
}

func TestDeployDockerfileModesAndRecursion(t *testing.T) {
	root := t.TempDir()
	writeDockerfileFixture(t, root, "custom", "FROM scratch")
	// The flag needs a source build; a TOML selector is inert without one.
	for _, config := range []core.Config{{Image: "registry/image"}, {Type: "volume-template"}, {}} {
		skipBuild := config.Image == "" && config.Type == ""
		_, err := resolveDeployDockerfile(root, "", "missing", config, skipBuild, false)
		require.ErrorContains(t, err, "requires a source build")
		config.Build = &core.BuildConfig{Dockerfile: "missing"}
		selected, err := resolveDeployDockerfile(root, "", "", config, skipBuild, true)
		require.NoError(t, err)
		assert.Nil(t, selected)
	}
	// A recursive deploy of child packages cannot share one selection.
	for _, config := range []core.Config{
		{Agent: map[string]core.Package{"child": {Path: "child"}}},
		{Function: map[string]core.Package{"child": {Path: "child"}}, SkipRoot: true},
	} {
		for _, flag := range []string{"custom", ""} {
			config.Build = &core.BuildConfig{Dockerfile: "custom"}
			_, err := resolveDeployDockerfile(root, "", flag, config, false, true)
			require.ErrorContains(t, err, "--recursive=false")
			_, err = resolveDeployDockerfile(root, "", flag, config, false, false)
			require.NoError(t, err)
			_, err = resolveDeployDockerfile(root, ".", flag, config, false, true)
			require.NoError(t, err)
		}
		config.Build = nil
		selected, err := resolveDeployDockerfile(root, "", "", config, false, true)
		require.NoError(t, err)
		assert.Nil(t, selected, "default recursion is unchanged")
	}
}

func dockerfileZipContents(t *testing.T, d *Deployment) map[string][]string {
	t.Helper()
	require.NoError(t, d.Zip())
	t.Cleanup(func() { _ = os.Remove(d.archive.Name()) })
	reader, err := zip.OpenReader(d.archive.Name())
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()
	files := map[string][]string{}
	for _, entry := range reader.File {
		r, err := entry.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		files[entry.Name] = append(files[entry.Name], string(content))
	}
	return files
}

func TestDeployDockerfileArchive(t *testing.T) {
	for _, tt := range []struct {
		name, folder, path              string
		defaultFile, companion, ignored bool
	}{
		{"no default", "", "custom", false, false, false},
		{"conflicting default", "", "custom", true, true, false},
		{"no companion fallback", "", "custom", true, false, false},
		{"ignored nested selection", "", "nested/custom", true, true, true},
		{"subproject", "project", "nested/custom", true, true, false},
		{"explicit default", "", "Dockerfile", false, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			core.ResetConfig()
			t.Cleanup(core.ResetConfig)
			selectedPath := filepath.Join(tt.folder, tt.path)
			fixture := map[string]string{
				"blaxel.toml":             "name = \"test\"\ntype = \"sandbox\"\n",
				".dockerignore":           "docker-ignored.txt\n",
				"Dockerfile.dockerignore": "wrong-default\n",
				"shared.txt":              "shared COPY source",
				".blaxelignore":           "",
				selectedPath:              "FROM ghcr.io/blaxel-ai/sandbox:latest\nCOPY shared.txt /shared\n",
			}
			if tt.folder != "" {
				fixture[filepath.Join(tt.folder, "blaxel.toml")] = "name = \"subproject\"\ntype = \"sandbox\"\n"
				fixture[filepath.Join(tt.folder, "Dockerfile")] = "wrong-project-default"
			}
			if tt.defaultFile {
				fixture["Dockerfile"] = "wrong-root-default"
			}
			if tt.companion {
				fixture[selectedPath+".dockerignore"] = "selected-ignore\n"
			}
			if tt.ignored {
				fixture[".blaxelignore"] += "nested\n"
			}
			for path, content := range fixture {
				writeDockerfileFixture(t, root, path, content)
			}
			core.ReadConfigToml(tt.folder, false)
			selected, err := resolveDeployDockerfile(root, tt.folder, tt.path, core.GetConfig(), false, false)
			require.NoError(t, err)
			files := dockerfileZipContents(t, &Deployment{cwd: root, folder: tt.folder, dockerfile: selected})

			assert.Equal(t, []string{fixture[selectedPath]}, files["Dockerfile"], "exactly one canonical file with selected bytes")
			if tt.companion {
				assert.Equal(t, []string{fixture[selectedPath+".dockerignore"]}, files["Dockerfile.dockerignore"])
			} else {
				assert.NotContains(t, files, "Dockerfile.dockerignore", "an unrelated default companion must not control the build")
			}
			assert.Equal(t, []string{fixture[".dockerignore"]}, files[".dockerignore"])
			assert.Equal(t, []string{fixture["shared.txt"]}, files["shared.txt"])
			if tt.ignored {
				assert.NotContains(t, files, toArchivePath(selectedPath), "the alias survives .blaxelignore")
			} else if selectedPath != "Dockerfile" {
				assert.Equal(t, []string{fixture[selectedPath]}, files[toArchivePath(selectedPath)], "the original stays eligible")
			}
			if tt.folder != "" {
				assert.Equal(t, fixture[filepath.Join(tt.folder, "blaxel.toml")], files["blaxel.toml"][len(files["blaxel.toml"])-1], "retain legacy manifest promotion")
			}
			for path, content := range fixture {
				actual, err := os.ReadFile(filepath.Join(root, path))
				require.NoError(t, err)
				assert.Equal(t, sha256.Sum256([]byte(content)), sha256.Sum256(actual), "local input changed: %s", path)
			}
		})
	}
}

// A selection resolved before the interactive type picker chose a volume template
// must not put a Dockerfile into the volume content.
func TestDeployDockerfileIgnoredForVolumeTemplate(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	core.ResetConfig()
	t.Cleanup(core.ResetConfig)
	writeDockerfileFixture(t, root, "blaxel.toml", "type = \"volume-template\"\n")
	writeDockerfileFixture(t, root, "custom", "custom")
	writeDockerfileFixture(t, root, "data.txt", "data")
	core.ReadConfigToml("", false)
	files := dockerfileZipContents(t, &Deployment{cwd: root, dockerfile: &deployDockerfile{path: filepath.Join(root, "custom")}})
	assert.NotContains(t, files, "Dockerfile")
	assert.Equal(t, []string{"data"}, files["data.txt"])
}

// A deploy that did not select a file keeps the default Dockerfile and companion
// even when the TOML names a selector, as push does.
func TestDeployDockerfileDefaultArchive(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	core.ResetConfig()
	t.Cleanup(core.ResetConfig)
	writeDockerfileFixture(t, root, "blaxel.toml", "type = \"sandbox\"\n[build]\ndockerfile = \"custom\"\n")
	writeDockerfileFixture(t, root, "Dockerfile", "default")
	writeDockerfileFixture(t, root, "Dockerfile.dockerignore", "default-ignore")
	writeDockerfileFixture(t, root, "custom", "custom")
	core.ReadConfigToml("", false)
	files := dockerfileZipContents(t, &Deployment{cwd: root})
	assert.Equal(t, []string{"default"}, files["Dockerfile"])
	assert.Equal(t, []string{"default-ignore"}, files["Dockerfile.dockerignore"])
}

func TestDeployDockerfileValidationAndServerEnv(t *testing.T) {
	root := t.TempDir()
	for _, tt := range []struct {
		defaultContent, customContent string
		warning                       bool
	}{
		{"FROM ghcr.io/blaxel-ai/sandbox:latest", "FROM debian:bookworm-slim", true},
		{"FROM debian:bookworm-slim", "FROM ghcr.io/blaxel-ai/sandbox:latest", false},
	} {
		writeDockerfileFixture(t, root, "Dockerfile", tt.defaultContent)
		writeDockerfileFixture(t, root, "blaxel.Dockerfile", tt.customContent)
		selected, err := resolveDeployDockerfile(root, "", "blaxel.Dockerfile", core.Config{}, false, false)
		require.NoError(t, err)
		d := Deployment{cwd: root, dockerfile: selected}
		assert.Equal(t, tt.warning, d.validateDeploymentConfig(core.Config{Type: "sandbox"}) != "")
		assert.Equal(t, !tt.warning, ValidateBuildConfig(root, "", core.Config{Type: "sandbox"}) != "", "push still validates the default")
	}
	for _, pattern := range []string{"HOST", "PORT", "BL_SERVER_HOST", "BL_SERVER_PORT", "nothing"} {
		writeDockerfileFixture(t, root, "blaxel.Dockerfile", "ENV "+pattern+"=value")
		selected, err := resolveDeployDockerfile(root, "", "blaxel.Dockerfile", core.Config{}, false, false)
		require.NoError(t, err)
		assert.Equal(t, pattern != "nothing", selected.usesServerEnv(), pattern)
	}
	var unselected *deployDockerfile
	assert.False(t, unselected.usesServerEnv())
}

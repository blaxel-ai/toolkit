package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSkillsInstallerLock(t *testing.T) {
	var lock struct {
		Packages map[string]struct {
			Version   string `json:"version"`
			Resolved  string `json:"resolved"`
			Integrity string `json:"integrity"`
		} `json:"packages"`
	}
	require.NoError(t, json.Unmarshal(skillsPackageLock, &lock))
	require.Len(t, lock.Packages, 9)
	for name, pkg := range lock.Packages {
		if name == "" {
			continue
		}
		assert.Regexp(t, regexp.MustCompile(`^\d+\.\d+\.\d+$`), pkg.Version, name)
		assert.True(t, strings.HasPrefix(pkg.Resolved, "https://registry.npmjs.org/"), name)
		require.True(t, strings.HasPrefix(pkg.Integrity, "sha512-"), name)
		digest, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(pkg.Integrity, "sha512-"))
		require.NoError(t, err)
		assert.Len(t, digest, 64, name)
	}
	assert.Equal(t, "1.7.0", lock.Packages["node_modules/skills"].Version)
	assert.Equal(t, "sha512-OfePnDft+Xt9/tCoHdCUe5fkM8i+Q3QOSQO53hm7mKtsXyvc+CKOAAliVWZ484HS3cWx+6r+ob0AArixs3jYXw==", lock.Packages["node_modules/skills"].Integrity)
}

func TestSkillsInstallerEnvironment(t *testing.T) {
	assert.Equal(t, []string{"HOME=/home/user", "PATH=/bin", "CODEX_HOME=/skills"}, skillsInstallerEnvironment([]string{
		"HOME=/home/user", "NODE_OPTIONS=--require=/tmp/inject.js", "node_path=/tmp/modules",
		"NPM_CONFIG_IGNORE_SCRIPTS=false", "npm_config_registry=https://evil.invalid", "PATH=/bin", "CODEX_HOME=/skills",
	}))
}

func TestPinnedSkillsCommands(t *testing.T) {
	var output bytes.Buffer
	var commands []*exec.Cmd
	err := runPinnedSkills(context.Background(), "/trusted/node", "/trusted/npm-cli.js", &output, func(cmd *exec.Cmd) error {
		commands = append(commands, cmd)
		require.NotEmpty(t, cmd.Dir)
		data, err := os.ReadFile(filepath.Join(cmd.Dir, "package-lock.json"))
		require.NoError(t, err)
		assert.Equal(t, skillsPackageLock, data)
		assert.Same(t, &output, cmd.Stdout)
		assert.Same(t, &output, cmd.Stderr)
		assert.Nil(t, cmd.Stdin)
		return nil
	})
	require.NoError(t, err)
	require.Len(t, commands, 2)
	assert.Equal(t, []string{"/trusted/node", "/trusted/npm-cli.js", "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--engine-strict", "--registry=https://registry.npmjs.org"}, commands[0].Args[:8])
	assert.Equal(t, []string{"/trusted/node", filepath.Join(commands[0].Dir, "node_modules", "skills", "bin", "cli.mjs"), "add", skillsRepo, "-g", "--all"}, commands[1].Args)
	assert.Equal(t, commands[0].Dir, commands[1].Dir)
	_, err = os.Stat(commands[0].Dir)
	assert.True(t, os.IsNotExist(err), "temporary package tree must be removed")
}

func TestPinnedSkillsPreparationFailureStopsExecution(t *testing.T) {
	calls := 0
	err := runPinnedSkills(context.Background(), "node", "npm-cli.js", &bytes.Buffer{}, func(_ *exec.Cmd) error {
		calls++
		return errors.New("EINTEGRITY")
	})
	assert.ErrorContains(t, err, "EINTEGRITY")
	assert.Equal(t, 1, calls)
}

func TestSkillsNPMCLIWindowsLayout(t *testing.T) {
	directory := t.TempDir()
	wrapper := filepath.Join(directory, "npm.cmd")
	cli := filepath.Join(directory, "node_modules", "npm", "bin", "npm-cli.js")
	require.NoError(t, os.MkdirAll(filepath.Dir(cli), 0700))
	require.NoError(t, os.WriteFile(wrapper, []byte("@echo off"), 0600))
	require.NoError(t, os.WriteFile(cli, []byte("// npm"), 0600))
	actual, err := skillsNPMCLI(wrapper)
	require.NoError(t, err)
	resolved, err := filepath.EvalSymlinks(cli)
	require.NoError(t, err)
	assert.Equal(t, resolved, actual)
}

// Exercise npm's actual integrity enforcement using a local tarball server.
// No external registry or installed agent directory is used by this test.
func TestPinnedSkillsRejectsChangedTarball(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is unavailable")
	}
	npm, err := exec.LookPath("npm")
	if err != nil {
		t.Skip("npm is unavailable")
	}
	npm, err = skillsNPMCLI(npm)
	require.NoError(t, err)
	var archive bytes.Buffer
	gz := gzip.NewWriter(&archive)
	tarball := tar.NewWriter(gz)
	manifest := []byte(`{"name":"skills","version":"1.7.0"}`)
	require.NoError(t, tarball.WriteHeader(&tar.Header{Name: "package/package.json", Mode: 0600, Size: int64(len(manifest))}))
	_, err = tarball.Write(manifest)
	require.NoError(t, err)
	require.NoError(t, tarball.Close())
	require.NoError(t, gz.Close())
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(archive.Bytes()) }))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	var output bytes.Buffer
	calls := 0
	err = runPinnedSkills(ctx, node, npm, &output, func(cmd *exec.Cmd) error {
		calls++
		require.Equal(t, 1, calls, "installer must never run after an integrity mismatch")
		fixture := map[string]any{
			"name": "blaxel-skills-installer", "version": "1.0.0", "lockfileVersion": 3,
			"packages": map[string]any{
				"":                    map[string]any{"name": "blaxel-skills-installer", "version": "1.0.0", "dependencies": map[string]string{"skills": "1.7.0"}},
				"node_modules/skills": map[string]string{"version": "1.7.0", "resolved": server.URL + "/skills.tgz", "integrity": "sha512-" + base64.StdEncoding.EncodeToString(make([]byte, 64))},
			},
		}
		data, marshalErr := json.Marshal(fixture)
		require.NoError(t, marshalErr)
		require.NoError(t, os.WriteFile(filepath.Join(cmd.Dir, "package-lock.json"), data, 0600))
		cmd.Args = append(cmd.Args, "--fetch-retries=0")
		return cmd.Run()
	})
	require.Error(t, err)
	assert.Equal(t, 1, calls)
	assert.Contains(t, output.String(), "EINTEGRITY")
}

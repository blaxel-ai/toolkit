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
	"runtime"
	"strconv"
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
	npm := filepath.Join(t.TempDir(), "npm")
	var directory string
	report, err := runPinnedSkills(context.Background(), "/trusted/node", npm, []string{"universal", "claude-code"}, &output, func(cmd *exec.Cmd) error {
		commands = append(commands, cmd)
		assert.Empty(t, cmd.Dir, "preserve caller directory for version-manager shims")
		if directory == "" {
			directory = skillsTestPrefix(t, cmd.Args)
		}
		data, err := os.ReadFile(filepath.Join(directory, "package-lock.json"))
		require.NoError(t, err)
		assert.Equal(t, skillsPackageLock, data)
		assert.Same(t, &output, cmd.Stderr)
		if len(commands) == 1 {
			assert.Same(t, &output, cmd.Stdout)
		} else {
			assert.NotSame(t, &output, cmd.Stdout, "the JSON report is captured separately")
			_, _ = cmd.Stdout.Write([]byte(`[{"name":"blaxel-cli","status":"installed"}]`))
		}
		assert.Nil(t, cmd.Stdin)
		return nil
	})
	require.NoError(t, err)
	assert.JSONEq(t, `[{"name":"blaxel-cli","status":"installed"}]`, string(report))
	require.Len(t, commands, 2)
	assert.Equal(t, []string{npm, "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--engine-strict", "--registry=https://registry.npmjs.org"}, commands[0].Args[:7])
	assert.Equal(t, []string{"/trusted/node", filepath.Join(directory, "node_modules", "skills", "bin", "cli.mjs"), "add", skillsRepo, "-g", "-y", "--skill", "*", "--json", "--agent", "universal", "claude-code"}, commands[1].Args)
	assert.Equal(t, commands[0].Dir, commands[1].Dir)
	_, err = os.Stat(directory)
	assert.True(t, os.IsNotExist(err), "temporary package tree must be removed")
}

func TestPinnedSkillsPreparationFailureStopsExecution(t *testing.T) {
	calls := 0
	_, err := runPinnedSkills(context.Background(), "node", "npm-cli.js", []string{"universal"}, &bytes.Buffer{}, func(_ *exec.Cmd) error {
		calls++
		return errors.New("EINTEGRITY")
	})
	assert.ErrorContains(t, err, "EINTEGRITY")
	assert.Equal(t, 1, calls)
}

func TestSkillsNPMCommandWindowsLayout(t *testing.T) {
	directory := t.TempDir()
	wrapper := filepath.Join(directory, "npm.cmd")
	cli := filepath.Join(directory, "node_modules", "npm", "bin", "npm-cli.js")
	require.NoError(t, os.MkdirAll(filepath.Dir(cli), 0700))
	require.NoError(t, os.WriteFile(wrapper, []byte("@echo off"), 0600))
	require.NoError(t, os.WriteFile(cli, []byte("// npm"), 0600))
	actual, err := skillsNPMCommand("node", wrapper)
	require.NoError(t, err)
	resolved, err := filepath.EvalSymlinks(cli)
	require.NoError(t, err)
	assert.Equal(t, []string{"node", resolved}, actual)
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
	_, err = runPinnedSkills(ctx, node, npm, []string{"universal"}, &output, func(cmd *exec.Cmd) error {
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
		require.NoError(t, os.WriteFile(filepath.Join(skillsTestPrefix(t, cmd.Args), "package-lock.json"), data, 0600))
		cmd.Args = append(cmd.Args, "--fetch-retries=0")
		return cmd.Run()
	})
	require.Error(t, err)
	assert.Equal(t, 1, calls)
	assert.Contains(t, output.String(), "EINTEGRITY")
}

func skillsTestPrefix(t *testing.T, args []string) string {
	t.Helper()
	for _, arg := range args {
		if prefix, found := strings.CutPrefix(arg, "--prefix="); found {
			return prefix
		}
	}
	t.Fatal("npm command is missing its isolated prefix")
	return ""
}

func TestSkillsNPMCommandNativeExecutable(t *testing.T) {
	// The Go test executable stands in for native version-manager launchers.
	executable, err := os.Executable()
	require.NoError(t, err)
	command, err := skillsNPMCommand("unused-node", executable)
	require.NoError(t, err)
	require.Equal(t, []string{executable}, command)
	output, err := exec.Command(command[0], "-test.run=^$").CombinedOutput()
	require.NoError(t, err, string(output))
	command, err = skillsNPMCommand("unused-node", filepath.Join(t.TempDir(), "npm.exe"))
	require.NoError(t, err)
	require.Len(t, command, 1, "native Windows shims must not be interpreted as JavaScript")
}

func TestPinnedSkillsExecutesShellShim(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix shell shim")
	}
	directory := t.TempDir()
	shim := filepath.Join(directory, "npm-shim")
	require.NoError(t, os.WriteFile(shim, []byte("#!/bin/sh\nprintf 'shim:%s' \"$1\"\n"), 0700))
	alias := filepath.Join(directory, "npm")
	require.NoError(t, os.Symlink(shim, alias))
	var output bytes.Buffer
	calls := 0
	_, err := runPinnedSkills(context.Background(), "unused-node", alias, []string{"universal"}, &output, func(cmd *exec.Cmd) error {
		calls++
		if calls == 1 {
			assert.Equal(t, alias, cmd.Path, "preserve shim symlink invocation")
			return cmd.Run()
		}
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, "shim:ci", output.String())
	assert.Equal(t, 2, calls)
}

func TestDetectSkillsAgents(t *testing.T) {
	home := t.TempDir()
	env := map[string]string{}
	lookup := func(key string) string { return env[key] }

	targets, names := detectSkillsAgents(home, lookup)
	assert.Equal(t, []string{"universal"}, targets, "no agents: only the shared directory")
	assert.Empty(t, names)
	entries, err := os.ReadDir(home)
	require.NoError(t, err)
	assert.Empty(t, entries, "detection must not create directories")

	for _, dir := range []string{".claude", ".codex", filepath.Join(".config", "goose"), ".cursor"} {
		require.NoError(t, os.MkdirAll(filepath.Join(home, dir), 0700))
	}
	targets, names = detectSkillsAgents(home, lookup)
	assert.Equal(t, []string{"universal", "claude-code", "goose"}, targets)
	assert.Equal(t, []string{"Claude Code", "Codex", "Cursor", "Goose"}, names)

	// Custom config locations are honored, like the skills installer does.
	other := t.TempDir()
	env["CLAUDE_CONFIG_DIR"] = filepath.Join(other, "claude")
	env["XDG_CONFIG_HOME"] = filepath.Join(other, "xdg")
	require.NoError(t, os.RemoveAll(filepath.Join(home, ".claude")))
	require.NoError(t, os.MkdirAll(env["CLAUDE_CONFIG_DIR"], 0700))
	require.NoError(t, os.MkdirAll(filepath.Join(env["XDG_CONFIG_HOME"], "opencode"), 0700))
	targets, names = detectSkillsAgents(home, lookup)
	assert.Equal(t, []string{"universal", "claude-code"}, targets)
	assert.Equal(t, []string{"Claude Code", "Codex", "Cursor", "OpenCode"}, names)
}

func TestSkillsNodeVersionSupported(t *testing.T) {
	for version, supported := range map[string]bool{
		"v22.20.0": true, "v22.21.1": true, "v23.0.0": true, "v26.3.0": true, "v22.20.0-nightly": true,
		"v22.19.9": false, "v20.11.0": false, "v18.0.0": false, "": false, "garbage": false,
	} {
		assert.Equal(t, supported, skillsNodeVersionSupported(version), version)
	}
}

func TestParseSkillsResult(t *testing.T) {
	skills, err := parseSkillsResult([]byte(`[{"name":"blaxel-cli","status":"installed"},{"name":"blaxel-sdk","status":"installed"}]` + "\n"))
	require.NoError(t, err)
	assert.Equal(t, []string{"blaxel-cli", "blaxel-sdk"}, skills)

	skills, err = parseSkillsResult([]byte("Update available: skills 1.8.0\n" + `[{"name":"blaxel-cli","status":"installed"}]`))
	require.NoError(t, err)
	assert.Equal(t, []string{"blaxel-cli"}, skills)

	_, err = parseSkillsResult([]byte(`[{"name":"blaxel-cli","status":"failed","error":"boom"}]`))
	assert.ErrorContains(t, err, "blaxel-cli: boom")
	_, err = parseSkillsResult([]byte(`[]`))
	assert.ErrorContains(t, err, "no skills were installed")
	_, err = parseSkillsResult([]byte("not json"))
	assert.ErrorContains(t, err, "unexpected installer output")
}

func TestWithSkillsLog(t *testing.T) {
	var log strings.Builder
	for i := 0; i < 30; i++ {
		log.WriteString("\x1b[32mline " + strconv.Itoa(i) + "\x1b[0m\n│\n")
	}
	err := withSkillsLog(errors.New("installing skills: exit status 1"), []byte(log.String()))
	message := err.Error()
	assert.True(t, strings.HasPrefix(message, "installing skills: exit status 1\n"))
	assert.Contains(t, message, "line 29")
	assert.NotContains(t, message, "line 14\n")
	assert.NotContains(t, message, "\x1b")
	assert.Equal(t, errors.New("x").Error(), withSkillsLog(errors.New("x"), nil).Error())
}

func TestSkillsInstalledMessage(t *testing.T) {
	skills := []string{"blaxel-cli", "blaxel-sdk"}
	assert.Equal(t, "Blaxel skills installed to ~/.agents/skills (blaxel-cli, blaxel-sdk). Restart your coding agent to load them.",
		skillsInstalledMessage(skillsInstallResult{skills: skills}))
	assert.Equal(t, "Blaxel skills installed for Claude Code (blaxel-cli, blaxel-sdk). Restart your coding agent to load them.",
		skillsInstalledMessage(skillsInstallResult{skills: skills, agents: []string{"Claude Code"}}))
	assert.Equal(t, "Blaxel skills installed for Claude Code, Codex and Cursor (blaxel-cli, blaxel-sdk). Restart your coding agent to load them.",
		skillsInstalledMessage(skillsInstallResult{skills: skills, agents: []string{"Claude Code", "Codex", "Cursor"}}))
}

func TestIsSkillsCommand(t *testing.T) {
	assert.True(t, isSkillsCommand([]string{"skills", "install"}))
	assert.False(t, isSkillsCommand([]string{"get", "skills"}))
	assert.False(t, isSkillsCommand(nil))
}

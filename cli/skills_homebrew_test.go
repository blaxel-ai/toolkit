package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/agentsetup"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func isolatedSkillsHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(agentsetup.SkillsInstallEnv, "true")
	return home
}

func createSkillsKeg(t *testing.T, prefix, version string) string {
	t.Helper()
	binary := filepath.Join(prefix, "Cellar", "blaxel", version, "bin", "blaxel")
	require.NoError(t, os.MkdirAll(filepath.Dir(binary), 0755))
	require.NoError(t, os.WriteFile(binary, []byte("binary fixture"), 0755))
	return binary
}

func TestSetupHomebrewSkillsOncePerVersion(t *testing.T) {
	home := isolatedSkillsHome(t)
	prefix := t.TempDir()
	binary := createSkillsKeg(t, prefix, "1.2.3")
	link := filepath.Join(prefix, "bl")
	createSkillsSymlink(t, binary, link)
	calls := 0
	install := func() { calls++ }
	setupHomebrewRefresh(link, install)
	setupHomebrewRefresh(link, install)
	require.Equal(t, 1, calls)
	require.FileExists(t, filepath.Join(home, ".blaxel", "setup", "homebrew", "1.2.3"))
	next := createSkillsKeg(t, prefix, "1.2.4")
	require.NoError(t, os.Remove(link))
	createSkillsSymlink(t, next, link)
	setupHomebrewRefresh(link, install)
	setupHomebrewRefresh(link, install)
	require.Equal(t, 2, calls)
	require.FileExists(t, filepath.Join(home, ".blaxel", "setup", "homebrew", "1.2.4"))
}

func TestSetupHomebrewSkillsDisabledDoesNotConsumeAttempt(t *testing.T) {
	for _, mode := range []string{"disabled", "ci"} {
		t.Run(mode, func(t *testing.T) {
			home := isolatedSkillsHome(t)
			binary := createSkillsKeg(t, t.TempDir(), "1.2.3")
			if mode == "disabled" {
				t.Setenv(agentsetup.SkillsInstallEnv, "false")
				t.Setenv(mcpInstallEnv, "false")
			} else {
				t.Setenv(agentsetup.SkillsInstallEnv, "")
				t.Setenv("CI", "true")
			}
			calls := 0
			setupHomebrewRefresh(binary, func() { calls++ })
			require.Zero(t, calls)
			require.NoDirExists(t, filepath.Join(home, ".blaxel"))
			t.Setenv(agentsetup.SkillsInstallEnv, "true")
			t.Setenv(mcpInstallEnv, "true")
			setupHomebrewRefresh(binary, func() { calls++ })
			require.Equal(t, 1, calls)
		})
	}
}

func TestSetupHomebrewSkillsFailedAttemptIsNotRepeated(t *testing.T) {
	home := isolatedSkillsHome(t)
	binary := createSkillsKeg(t, t.TempDir(), "1.2.3")
	calls := 0
	// The callback cannot return errors: a failed installer simply returns.
	failedInstall := func() {
		calls++
		require.FileExists(t, filepath.Join(home, ".blaxel", "setup", "homebrew", "1.2.3"))
	}
	setupHomebrewRefresh(binary, failedInstall)
	setupHomebrewRefresh(binary, failedInstall)
	require.Equal(t, 1, calls)
}

func TestSetupHomebrewSkillsStateFailureSkipsInstall(t *testing.T) {
	home := isolatedSkillsHome(t)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".blaxel"), []byte("not a directory"), 0600))
	binary := createSkillsKeg(t, t.TempDir(), "1.2.3")
	calls := 0
	setupHomebrewRefresh(binary, func() { calls++ })
	require.Zero(t, calls)
}

func TestSetupHomebrewSkillsIgnoresOtherInstallations(t *testing.T) {
	home := isolatedSkillsHome(t)
	binary := filepath.Join(t.TempDir(), "blaxel")
	require.NoError(t, os.WriteFile(binary, nil, 0755))
	setupHomebrewRefresh(binary, func() { t.Fatal("installer called for non-Homebrew binary") })
	setupHomebrewRefresh(binary+"-missing", func() { t.Fatal("installer called for missing binary") })
	require.NoDirExists(t, filepath.Join(home, ".blaxel"))
}

func TestClaimSkillsInstallConcurrent(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "nested", "state", "1.2.3")
	var claims atomic.Int32
	var wg sync.WaitGroup
	errors := make(chan error, 32)
	start := make(chan struct{})
	for i := 0; i < cap(errors); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			claimed, err := claimSkillsInstall(marker)
			if err != nil {
				errors <- err
			}
			if claimed {
				claims.Add(1)
			}
		}()
	}
	close(start)
	wg.Wait()
	close(errors)
	for err := range errors {
		require.NoError(t, err)
	}
	require.EqualValues(t, 1, claims.Load())
	require.FileExists(t, marker)
}

func createSkillsSymlink(t *testing.T, target, link string) {
	t.Helper()
	err := os.Symlink(target, link)
	if err != nil && runtime.GOOS == "windows" {
		t.Skipf("symlinks require Windows privileges: %v", err)
	}
	require.NoError(t, err)
}

func TestIsSkillsCommand(t *testing.T) {
	assert.True(t, isSkillsCommand([]string{"skills", "install"}))
	assert.False(t, isSkillsCommand([]string{"get", "skills"}))
	assert.False(t, isSkillsCommand(nil))
}

func TestHomebrewRefreshFor(t *testing.T) {
	refreshed := 0
	refresh := func() { refreshed++ }
	// bl skills installs skills only, so the MCP refresh stays pending too.
	for _, args := range [][]string{{"mcp"}, {"--workspace", "w", "mcp"}, {"upgrade"}, {"__complete", "get", ""}, {"skills", "install"}} {
		require.Nil(t, homebrewRefreshFor(args, refresh), args)
	}
	// bl setup sets up skills and MCP by itself: record the keg only.
	for _, args := range [][]string{{"setup"}, {"-w", "mcp", "setup", "--yes"}} {
		install := homebrewRefreshFor(args, refresh)
		require.NotNil(t, install, args)
		install()
	}
	require.Zero(t, refreshed)
	// A flag value or argument naming those commands is not the command.
	for _, args := range [][]string{{"get", "mcp"}, {"-w", "upgrade", "get", "agents"}, {"new", "mcp"}, {"get", "agents"}} {
		install := homebrewRefreshFor(args, refresh)
		require.NotNil(t, install, args)
		install()
	}
	require.Equal(t, 4, refreshed)
}

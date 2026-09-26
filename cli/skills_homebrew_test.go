package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHomebrewSkillsLocation(t *testing.T) {
	for _, tt := range []struct{ name, path, prefix, version string }{
		{"apple silicon", "/opt/homebrew/Cellar/blaxel/1.2.3/bin/blaxel", "/opt/homebrew", "1.2.3"},
		{"intel alias", "/usr/local/Cellar/blaxel/1.2.3_1/bin/bl", "/usr/local", "1.2.3_1"},
		{"linux", "/home/linuxbrew/.linuxbrew/Cellar/blaxel/1.2.3/bin/blaxel", "/home/linuxbrew/.linuxbrew", "1.2.3"},
		{"custom prefix", "/tmp/custom brew/Cellar/blaxel/1.2.3/bin/blaxel", "/tmp/custom brew", "1.2.3"},
		{"curl", "/home/user/.local/bin/blaxel", "", ""},
		{"other formula", "/opt/homebrew/Cellar/other/1.2.3/bin/blaxel", "", ""},
		{"lookalike formula", "/opt/homebrew/Cellar/blaxel-extra/1.2.3/bin/blaxel", "", ""},
		{"lookalike cellar", "/opt/homebrew/NotCellar/blaxel/1.2.3/bin/blaxel", "", ""},
		{"wrong directory", "/opt/homebrew/Cellar/blaxel/1.2.3/lib/blaxel", "", ""},
		{"wrong binary", "/opt/homebrew/Cellar/blaxel/1.2.3/bin/other", "", ""},
		{"relative", "Cellar/blaxel/1.2.3/bin/blaxel", "", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path, expectedPrefix := tt.path, tt.prefix
			if runtime.GOOS == "windows" && filepath.IsAbs(filepath.FromSlash("C:"+path)) {
				path = filepath.FromSlash("C:" + path)
				if expectedPrefix != "" {
					expectedPrefix = filepath.FromSlash("C:" + expectedPrefix)
				}
			}
			prefix, version := homebrewSkillsLocation(path)
			require.Equal(t, expectedPrefix, prefix)
			require.Equal(t, tt.version, version)
		})
	}
}

func isolatedSkillsHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	t.Setenv(skillsInstallEnv, "true")
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
	setupHomebrewSkills(link, install)
	setupHomebrewSkills(link, install)
	require.Equal(t, 1, calls)
	require.FileExists(t, filepath.Join(home, ".blaxel", "skills", "homebrew", "1.2.3"))
	next := createSkillsKeg(t, prefix, "1.2.4")
	require.NoError(t, os.Remove(link))
	createSkillsSymlink(t, next, link)
	setupHomebrewSkills(link, install)
	setupHomebrewSkills(link, install)
	require.Equal(t, 2, calls)
	require.FileExists(t, filepath.Join(home, ".blaxel", "skills", "homebrew", "1.2.4"))
}

func TestSetupHomebrewSkillsDisabledDoesNotConsumeAttempt(t *testing.T) {
	for _, mode := range []string{"disabled", "ci"} {
		t.Run(mode, func(t *testing.T) {
			home := isolatedSkillsHome(t)
			binary := createSkillsKeg(t, t.TempDir(), "1.2.3")
			if mode == "disabled" {
				t.Setenv(skillsInstallEnv, "false")
			} else {
				t.Setenv(skillsInstallEnv, "")
				t.Setenv("CI", "true")
			}
			calls := 0
			setupHomebrewSkills(binary, func() { calls++ })
			require.Zero(t, calls)
			require.NoDirExists(t, filepath.Join(home, ".blaxel"))
			t.Setenv(skillsInstallEnv, "true")
			setupHomebrewSkills(binary, func() { calls++ })
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
		require.FileExists(t, filepath.Join(home, ".blaxel", "skills", "homebrew", "1.2.3"))
	}
	setupHomebrewSkills(binary, failedInstall)
	setupHomebrewSkills(binary, failedInstall)
	require.Equal(t, 1, calls)
}

func TestSetupHomebrewSkillsStateFailureSkipsInstall(t *testing.T) {
	home := isolatedSkillsHome(t)
	require.NoError(t, os.WriteFile(filepath.Join(home, ".blaxel"), []byte("not a directory"), 0600))
	binary := createSkillsKeg(t, t.TempDir(), "1.2.3")
	calls := 0
	setupHomebrewSkills(binary, func() { calls++ })
	require.Zero(t, calls)
}

func TestSetupHomebrewSkillsIgnoresOtherInstallations(t *testing.T) {
	home := isolatedSkillsHome(t)
	binary := filepath.Join(t.TempDir(), "blaxel")
	require.NoError(t, os.WriteFile(binary, nil, 0755))
	setupHomebrewSkills(binary, func() { t.Fatal("installer called for non-Homebrew binary") })
	setupHomebrewSkills(binary+"-missing", func() { t.Fatal("installer called for missing binary") })
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

func TestMarkUpgradedHomebrewSkillsWithDeletedOldKeg(t *testing.T) {
	for _, disabled := range []bool{false, true} {
		name := "enabled"
		if disabled {
			name = "disabled"
		}
		t.Run(name, func(t *testing.T) {
			home := isolatedSkillsHome(t)
			prefix := t.TempDir()
			oldBinary := createSkillsKeg(t, prefix, "1.2.3")
			nextBinary := createSkillsKeg(t, prefix, "1.2.4")
			opt := filepath.Join(prefix, "opt", "blaxel")
			require.NoError(t, os.MkdirAll(filepath.Dir(opt), 0755))
			createSkillsSymlink(t, filepath.Dir(filepath.Dir(nextBinary)), opt)
			require.NoError(t, os.RemoveAll(filepath.Dir(filepath.Dir(oldBinary))))
			if disabled {
				t.Setenv(skillsInstallEnv, "false")
			}
			markHomebrewSkillsForExecutable(oldBinary)
			marker := filepath.Join(home, ".blaxel", "skills", "homebrew", "1.2.4")
			if disabled {
				require.NoFileExists(t, marker)
				t.Setenv(skillsInstallEnv, "true")
				calls := 0
				setupHomebrewSkills(nextBinary, func() { calls++ })
				require.Equal(t, 1, calls)
			} else {
				require.FileExists(t, marker)
				setupHomebrewSkills(nextBinary, func() { t.Fatal("upgraded version attempted again") })
			}
			require.NoFileExists(t, filepath.Join(home, ".blaxel", "skills", "homebrew", "1.2.3"))
		})
	}
}

package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

var (
	skillsInstallOnce  sync.Once
	skillsInstallError error
)

const (
	// skillsRepo is the GitHub repository holding the Blaxel agent skills.
	skillsRepo = "blaxel-ai/agent-skills"
	// skillsInstallEnv controls skills installation: "false" disables it, "true" forces it.
	skillsInstallEnv = "BL_INSTALL_SKILLS"
)

// skillsInstallCommand is the shared, integrity-checked installation entry point.
func skillsInstallCommand() string {
	return "bl skills install"
}

// skillsInstallDisabled reports whether skills installation should be skipped:
// BL_INSTALL_SKILLS=false always disables it, and CI environments are skipped
// unless BL_INSTALL_SKILLS=true (mirrors install.sh / install.ps1).
func skillsInstallDisabled(env func(string) string) bool {
	switch strings.ToLower(strings.TrimSpace(env(skillsInstallEnv))) {
	case "false":
		return true
	case "true":
		return false
	}

	for _, ciEnv := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL", "BUILDKITE"} {
		if env(ciEnv) != "" {
			return true
		}
	}
	return false
}

// installSkills installs the Blaxel agent skills after a CLI install/upgrade.
// It is best-effort: failures are reported as warnings and never abort the upgrade.
func installSkills() {
	if skillsInstallDisabled(os.Getenv) {
		return
	}
	// A first invocation of `bl upgrade` also passes through startup setup.
	// Both paths use the same installer, but only one npm process is needed.
	if err := installSkillsOnce(); err != nil {
		fmt.Fprintln(os.Stderr, "Could not install the Blaxel skills:", err)
		fmt.Fprintln(os.Stderr, "You can retry later with:", skillsInstallCommand())
	}
}

func installSkillsOnce() error {
	skillsInstallOnce.Do(func() { skillsInstallError = runSkillsInstall() })
	return skillsInstallError
}

func runSkillsInstall() error {
	fmt.Fprintln(os.Stderr, "Installing Blaxel skills for coding agents (Claude Code, Codex, Cursor, ...)...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if err := installPinnedSkills(ctx, os.Stderr); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "Blaxel skills installed. Restart your coding agent to load them.")
	return nil
}

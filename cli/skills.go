package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

var skillsInstallOnce sync.Once

const (
	// skillsRepo is the GitHub repository holding the Blaxel agent skills.
	skillsRepo = "blaxel-ai/agent-skills"
	// skillsInstallEnv controls skills installation: "false" disables it, "true" forces it.
	skillsInstallEnv = "BL_INSTALL_SKILLS"
)

// skillsInstallArgs returns the npx invocation that installs (or refreshes)
// all Blaxel agent skills globally, so coding agents pick them up.
func skillsInstallArgs() []string {
	return []string{"npx", "-y", "skills", "add", skillsRepo, "-g", "--all"}
}

// skillsInstallCommand returns the install command as a single string for display.
func skillsInstallCommand() string {
	return strings.Join(skillsInstallArgs(), " ")
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
	skillsInstallOnce.Do(runSkillsInstall)
}

func runSkillsInstall() {
	if _, err := exec.LookPath("npx"); err != nil {
		fmt.Fprintln(os.Stderr, "Skipping Blaxel skills installation: npx (Node.js) was not found.")
		fmt.Fprintln(os.Stderr, "Install Node.js (https://nodejs.org) and then run:", skillsInstallCommand())
		return
	}

	fmt.Fprintln(os.Stderr, "Installing Blaxel skills for coding agents (Claude Code, Codex, Cursor, ...)...")

	// A slow registry must not indefinitely delay the user's first command.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	args := skillsInstallArgs()
	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	cmd.WaitDelay = time.Second

	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Could not install the Blaxel skills:", err)
		fmt.Fprintln(os.Stderr, "You can retry later with:", skillsInstallCommand())
		return
	}

	fmt.Fprintln(os.Stderr, "Blaxel skills installed. Restart your coding agent to load them.")
}

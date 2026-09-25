package cli

import (
	"os"
	"os/exec"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
)

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

	if _, err := exec.LookPath("npx"); err != nil {
		core.PrintWarning("Skipping Blaxel skills installation: npx (Node.js) was not found")
		core.PrintInfoWithCommand("Install Node.js (https://nodejs.org) and then run:", skillsInstallCommand())
		return
	}

	core.PrintInfo("Installing Blaxel skills for coding agents (Claude Code, Codex, Cursor, ...)...")

	args := skillsInstallArgs()
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		core.PrintWarning("Could not install the Blaxel skills: " + err.Error())
		core.PrintInfoWithCommand("You can retry later with:", skillsInstallCommand())
		return
	}

	core.PrintSuccess("Blaxel skills installed. Restart your coding agent to load them.")
}

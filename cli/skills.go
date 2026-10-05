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
	skillsReportOnce   sync.Once
)

const (
	// skillsRepo is the GitHub repository holding the Blaxel agent skills.
	skillsRepo = "blaxel-ai/agent-skills"
	// skillsInstallEnv controls skills installation: "false" disables it, "true" forces it.
	skillsInstallEnv = "BL_INSTALL_SKILLS"
)

// skillsInstallCommand is the command that installs or refreshes the skills.
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
	// Both paths use the same installer, but only one download is needed.
	if err := installSkillsOnce(); err != nil {
		// Startup setup and bl upgrade can share one failed attempt; report it once.
		skillsReportOnce.Do(func() {
			fmt.Fprintln(os.Stderr, "Could not install the Blaxel skills:", err)
			fmt.Fprintln(os.Stderr, "You can retry later with:", skillsInstallCommand())
		})
	}
}

func installSkillsOnce() error {
	skillsInstallOnce.Do(func() { skillsInstallError = runSkillsInstall() })
	return skillsInstallError
}

func runSkillsInstall() error {
	fmt.Fprintln(os.Stderr, "Installing Blaxel skills for coding agents...")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	result, err := installDetectedSkills(ctx)
	if err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, skillsInstalledMessage(result))
	return nil
}

func skillsInstalledMessage(result skillsInstallResult) string {
	target := "to ~/.agents/skills"
	if len(result.agents) > 0 {
		target = "for " + joinSkillsNames(result.agents)
	}
	var messages []string
	if len(result.skills) > 0 {
		messages = append(messages, fmt.Sprintf("Blaxel skills installed %s (%s). Restart your coding agent to load them.",
			target, strings.Join(result.skills, ", ")))
	}
	if len(result.preserved) > 0 {
		messages = append(messages, "Kept externally managed Blaxel skills ("+strings.Join(result.preserved, ", ")+"); their links, contents and upstream update records were left unchanged.")
	}
	return strings.Join(messages, "\n")
}

func skillsInstallDetail(result skillsInstallResult) string {
	detail := strings.Join(result.skills, ", ")
	if len(result.preserved) > 0 {
		if detail != "" {
			detail += " · "
		}
		detail += "kept externally managed: " + strings.Join(result.preserved, ", ")
	}
	return detail
}

func joinSkillsNames(names []string) string {
	switch len(names) {
	case 0:
		return ""
	case 1:
		return names[0]
	case 2:
		return names[0] + " and " + names[1]
	}
	return strings.Join(names[:len(names)-1], ", ") + " and " + names[len(names)-1]
}

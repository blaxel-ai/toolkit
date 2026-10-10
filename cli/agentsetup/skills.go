package agentsetup

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
	// SkillsInstallEnv controls skills installation: "false" disables it, "true" forces it.
	SkillsInstallEnv = "BL_INSTALL_SKILLS"
)

// skillsInstallCommand is the command that installs or refreshes the skills.
func skillsInstallCommand() string {
	return "bl skills install"
}

// SkillsInstallDisabled reports whether skills installation should be skipped:
// BL_INSTALL_SKILLS=false always disables it, and CI environments are skipped
// unless BL_INSTALL_SKILLS=true (mirrors install.sh / install.ps1).
func SkillsInstallDisabled(env func(string) string) bool {
	return AutomaticInstallDisabled(env, SkillsInstallEnv)
}

func AutomaticInstallDisabled(env func(string) string, setting string) bool {
	switch strings.ToLower(strings.TrimSpace(env(setting))) {
	case "false":
		return true
	case "true":
		return false
	}

	return CIEnvironment(env)
}

func CIEnvironment(env func(string) string) bool {
	for _, name := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL", "BUILDKITE"} {
		if env(name) != "" {
			return true
		}
	}
	return false
}

func InstallSkillsOnce() error {
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

func skillsInstalledMessage(result SkillsInstallResult) string {
	target := "to ~/.agents/skills"
	if len(result.Agents) > 0 {
		target = "for " + joinSkillsNames(result.Agents)
	}
	var messages []string
	if len(result.Skills) > 0 {
		messages = append(messages, fmt.Sprintf("Blaxel skills installed %s (%s). Restart your coding agent to load them.",
			target, strings.Join(result.Skills, ", ")))
	}
	if len(result.Preserved) > 0 {
		messages = append(messages, "Kept externally managed Blaxel skills ("+strings.Join(result.Preserved, ", ")+"); their contents and upstream update records were left unchanged.")
	}
	if len(result.Repaired) > 0 {
		messages = append(messages, "Automatically linked skill paths to existing copies.")
	}
	for _, backup := range result.Backups {
		messages = append(messages, "Previous copy backed up at "+backup)
	}
	return strings.Join(messages, "\n")
}

func SkillsInstallDetail(result SkillsInstallResult) string {
	detail := strings.Join(result.Skills, ", ")
	if len(result.Preserved) > 0 {
		if detail != "" {
			detail += " · "
		}
		detail += "kept externally managed: " + strings.Join(result.Preserved, ", ")
	}
	if len(result.Repaired) > 0 {
		if detail != "" {
			detail += " · "
		}
		detail += "skill paths auto-fixed"
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

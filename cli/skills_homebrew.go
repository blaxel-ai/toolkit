package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blaxel-ai/toolkit/cli/core"
)

// homebrewSkillsLocation recognizes the resolved keg path without invoking brew
// on every command. It supports custom prefixes and both binary aliases.
func homebrewSkillsLocation(executable string) (prefix, version string) {
	executable = filepath.Clean(executable)
	if !filepath.IsAbs(executable) || (filepath.Base(executable) != "blaxel" && filepath.Base(executable) != "bl") {
		return "", ""
	}
	bin := filepath.Dir(executable)
	keg := filepath.Dir(bin)
	rack := filepath.Dir(keg)
	cellar := filepath.Dir(rack)
	if filepath.Base(bin) != "bin" || filepath.Base(rack) != "blaxel" || filepath.Base(cellar) != "Cellar" {
		return "", ""
	}
	return filepath.Dir(cellar), filepath.Base(keg)
}

func installHomebrewSkills() {
	// bl mcp is started by coding agents, which give it seconds to answer.
	if skillsInstallDisabled(os.Getenv) || core.IsShellCompletionRequest(os.Args[1:]) || isMCPBridgeArgs(os.Args[1:]) {
		return
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	install := func() {
		installSkills()
		// Homebrew cannot ask questions during install, so the first command
		// points new users to the rest of the setup once.
		if !isLoginCommand(os.Args[1:]) && setupLoginState("") == "" {
			fmt.Fprintln(os.Stderr, homebrewSetupHint)
		}
	}
	if isSkillsCommand(os.Args[1:]) {
		// The explicit command installs and reports by itself; only record it.
		install = func() {}
	}
	setupHomebrewSkills(executable, install)
}

const homebrewSetupHint = "Finish setting up Blaxel (MCP servers for your coding agents, then login) with: bl setup"

// isMCPBridgeArgs reports bl mcp, as setup writes it into agent configurations.
func isMCPBridgeArgs(args []string) bool {
	return len(args) > 0 && args[0] == "mcp"
}

func isLoginCommand(args []string) bool {
	return len(args) > 0 && args[0] == "login"
}

// isSkillsCommand reports commands that install the skills themselves.
func isSkillsCommand(args []string) bool {
	return len(args) > 0 && (args[0] == "skills" || args[0] == "setup")
}

// setupHomebrewSkills records an attempt before installing, so concurrent
// launches and failures cannot repeatedly delay commands. A failed attempt can
// be retried explicitly with bl upgrade or bl skills install.
func setupHomebrewSkills(executable string, install func()) {
	if skillsInstallDisabled(os.Getenv) {
		return
	}
	realPath, err := filepath.EvalSymlinks(executable)
	if err != nil {
		return
	}
	_, version := homebrewSkillsLocation(realPath)
	if version == "" {
		return
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	marker := filepath.Join(home, ".blaxel", "skills", "homebrew", version)
	claimed, err := claimSkillsInstall(marker)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not save Blaxel skills setup state:", err)
		fmt.Fprintln(os.Stderr, "You can install the skills with:", skillsInstallCommand())
		return
	}
	if claimed {
		install()
	}
}

func claimSkillsInstall(marker string) (bool, error) {
	file, err := os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(marker), 0700); err != nil {
			return false, err
		}
		file, err = os.OpenFile(marker, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	}
	if errors.Is(err, os.ErrExist) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, file.Close()
}

// The running binary still belongs to the old keg after brew upgrades it.
// Follow Homebrew's stable opt link to mark the newly installed version too.
func markUpgradedHomebrewSkills() {
	executable, err := os.Executable()
	if err != nil {
		return
	}
	markHomebrewSkillsForExecutable(executable)
}

func markHomebrewSkillsForExecutable(executable string) {
	if realPath, err := filepath.EvalSymlinks(executable); err == nil {
		executable = realPath
	}
	prefix, _ := homebrewSkillsLocation(executable)
	if prefix != "" {
		setupHomebrewSkills(filepath.Join(prefix, "opt", "blaxel", "bin", "blaxel"), func() {})
	}
}

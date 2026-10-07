package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"

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

func refreshHomebrewSetup() {
	install := homebrewRefreshFor(os.Args[1:], func() { refreshSetup(setupOptions{}) })
	if install == nil {
		return
	}
	executable, err := os.Executable()
	if err == nil {
		setupHomebrewRefresh(executable, install)
	}
}

// homebrewRefreshFor returns what the first command on a new Homebrew keg does
// about the refresh: nothing yet (nil) for agent handshakes and shell
// completion, which must never wait for a download, for bl upgrade, and for
// bl skills, which installs skills only and leaves the MCP refresh pending;
// only recording the keg for bl setup, which sets up both itself; otherwise
// the refresh. The command is resolved as cobra will, so a flag
// value or argument such as `bl get mcp` does not count.
func homebrewRefreshFor(args []string, refresh func()) func() {
	if core.IsShellCompletionRequest(args) {
		return nil
	}
	if !slices.ContainsFunc(args, func(arg string) bool { return arg == "mcp" || arg == "skills" || arg == "setup" || arg == "upgrade" }) {
		return refresh
	}
	command := core.ResolveStartupCommand(args, "mcp", "skills", "setup", "upgrade")
	switch {
	case isMCPBridgeArgs([]string{command}) || command == "upgrade" || command == "skills":
		return nil
	case command == "setup":
		return func() {}
	}
	return refresh
}

// isMCPBridgeArgs reports bl mcp, as setup writes it into agent configurations.
func isMCPBridgeArgs(args []string) bool {
	return len(args) > 0 && args[0] == "mcp"
}

// isSkillsCommand reports commands that install the skills themselves.
func isSkillsCommand(args []string) bool {
	return len(args) > 0 && (args[0] == "skills" || args[0] == "setup")
}

// setupHomebrewRefresh records an attempt before installing, so concurrent
// launches and failures cannot repeatedly delay commands. A failed attempt can
// be retried explicitly with bl upgrade or bl setup.
func setupHomebrewRefresh(executable string, install func()) {
	if skills, mcp := automaticSetupOffers(os.Getenv); !skills && !mcp {
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
	marker := filepath.Join(home, ".blaxel", "setup", "homebrew", version)
	claimed, err := claimSkillsInstall(marker)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Could not save Blaxel setup refresh state; retry with bl setup.")
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

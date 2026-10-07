package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/blaxel-ai/toolkit/cli/agentsetup"
	"github.com/blaxel-ai/toolkit/cli/core"
)

func refreshHomebrewSetup() {
	// Agent handshakes and shell completion must never wait for a download.
	args := os.Args[1:]
	// Conservatively defer when a refresh command appears after global flags too.
	if core.IsShellCompletionRequest(args) {
		return
	}
	for _, arg := range args {
		if isMCPBridgeArgs([]string{arg}) || isSkillsCommand([]string{arg}) || arg == "upgrade" {
			return
		}
	}
	executable, err := os.Executable()
	if err == nil {
		setupHomebrewRefresh(executable, func() { refreshSetup(setupOptions{}) })
	}
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
	_, version := agentsetup.HomebrewSkillsLocation(realPath)
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

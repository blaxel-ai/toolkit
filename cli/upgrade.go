package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	// Auto-register this command
	core.RegisterCommand("upgrade", func() *cobra.Command {
		return UpgradeCmd()
	})
}

func UpgradeCmd() *cobra.Command {
	var targetVersion string
	var force bool

	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade the Blaxel CLI to the latest version",
		Long: `Upgrade the Blaxel CLI to the latest version.

This command automatically detects your installation method and updates
the CLI in the correct location to avoid version conflicts.

Supported installation methods:
  - Homebrew (brew)
  - Manual installation (install.sh)
  - Direct binary download

Examples:
  # Upgrade to the latest version
  bl upgrade

  # Upgrade to a specific version
  bl upgrade --version v1.2.3

  # Force reinstall even if already on latest version
  bl upgrade --force`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runUpgrade(targetVersion, force)
		},
	}

	cmd.Flags().StringVar(&targetVersion, "version", "", "Target version to upgrade to (e.g., v1.2.3)")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force reinstall even if already on latest version")

	return cmd
}

// detectInstallationMethod determines how the CLI was installed
func detectInstallationMethod() (string, error) {
	// Get the path to the current executable
	execPath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	// Resolve any symlinks to get the actual binary location
	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		// If we can't resolve symlinks, use the original path
		realPath = execPath
	}

	// Check if installed via Homebrew
	// Common Homebrew paths: /opt/homebrew/*, /usr/local/Cellar/*, /usr/local/opt/*
	if strings.Contains(realPath, "/opt/homebrew") ||
		strings.Contains(realPath, "/usr/local/Cellar") ||
		strings.Contains(realPath, "/usr/local/opt") ||
		strings.Contains(realPath, "/home/linuxbrew") {
		return "brew", nil
	}

	// Otherwise, assume curl installation
	return "curl", nil
}

// runUpgrade executes the appropriate upgrade command based on installation method
func runUpgrade(targetVersion string, force bool) error {
	method, err := detectInstallationMethod()
	if err != nil {
		return err
	}

	core.PrintInfo(fmt.Sprintf("Detected installation method: %s", method))

	switch method {
	case "brew":
		return upgradeViaBrew(force)
	case "curl":
		return upgradeViaCurl(targetVersion)
	default:
		return fmt.Errorf("unknown installation method: %s", method)
	}
}

// upgradeViaBrew upgrades the CLI using Homebrew
func upgradeViaBrew(force bool) error {
	core.PrintInfo("Upgrading Blaxel CLI via Homebrew...")

	var cmd *exec.Cmd
	if force {
		// Use reinstall to force update
		cmd = exec.Command("brew", "reinstall", "blaxel")
	} else {
		cmd = exec.Command("brew", "upgrade", "blaxel")
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		// Check if the error is because package is already up-to-date
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			core.PrintInfo("Blaxel CLI is already up to date")
			return nil
		}
		return fmt.Errorf("brew upgrade failed: %w", err)
	}

	core.PrintSuccess("Blaxel CLI upgraded successfully via Homebrew")
	return nil
}

// upgradeViaCurl upgrades the CLI using the install script
func upgradeViaCurl(targetVersion string) error {
	core.PrintInfo("Upgrading Blaxel CLI via install script...")

	// Get the current executable path to determine if we need sudo
	execPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	realPath, err := filepath.EvalSymlinks(execPath)
	if err != nil {
		realPath = execPath
	}

	binDir := filepath.Dir(realPath)

	// Check if we need sudo (if binary is in a system directory)
	needsSudo := needsSudoForPath(binDir)

	// Build the install command
	installScriptURL := "https://raw.githubusercontent.com/blaxel-ai/toolkit/main/install.sh"

	var shellCmd string
	if targetVersion != "" {
		// Upgrade to specific version
		core.PrintInfo(fmt.Sprintf("Upgrading to version %s...", targetVersion))
		if needsSudo {
			shellCmd = fmt.Sprintf("curl -fsSL %s | VERSION=%s BINDIR=%s sudo -E sh", installScriptURL, targetVersion, binDir)
		} else {
			shellCmd = fmt.Sprintf("curl -fsSL %s | VERSION=%s BINDIR=%s sh", installScriptURL, targetVersion, binDir)
		}
	} else {
		// Upgrade to latest version
		core.PrintInfo("Upgrading to latest version...")
		if needsSudo {
			shellCmd = fmt.Sprintf("curl -fsSL %s | BINDIR=%s sudo -E sh", installScriptURL, binDir)
		} else {
			shellCmd = fmt.Sprintf("curl -fsSL %s | BINDIR=%s sh", installScriptURL, binDir)
		}
	}

	if needsSudo {
		core.PrintWarning("This upgrade requires sudo privileges")
		core.Print(fmt.Sprintf("Running: %s", shellCmd))
	}

	// Execute the install script
	cmd := exec.Command("sh", "-c", shellCmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}

	core.PrintSuccess("Blaxel CLI upgraded successfully")
	return nil
}

// needsSudoForPath determines if we need sudo to write to a directory
func needsSudoForPath(path string) bool {
	// Common system directories that require sudo
	systemPaths := []string{
		"/usr/local/bin",
		"/usr/bin",
		"/bin",
		"/usr/sbin",
		"/sbin",
	}

	for _, sysPath := range systemPaths {
		if strings.HasPrefix(path, sysPath) {
			return true
		}
	}

	return false
}

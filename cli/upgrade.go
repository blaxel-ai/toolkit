package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	"github.com/blaxel-ai/toolkit/cli/agentsetup"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

var bareSemverUpgradeVersionPattern = regexp.MustCompile(`^\d+\.\d+\.\d+(?:[-+][0-9A-Za-z.-]+)?$`)

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
  - Manual installation (install.sh, or install.ps1 on Windows)
  - Direct binary download

After upgrading, the newly installed CLI refreshes the Blaxel agent skills and
MCP servers for detected coding agents without setup screens or login. Existing
custom MCP entries, plugin-managed servers, and externally managed skills are
kept. Set BL_INSTALL_SKILLS=false or BL_INSTALL_MCP=false to skip either part.
Automatic refresh is skipped in CI unless the corresponding setting is true.
If the requested release does not support headless refresh, setup is left alone.

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
	if isInstalledViaHomebrew(realPath) {
		return "brew", nil
	}

	// Otherwise, assume curl installation
	return "curl", nil
}

// isInstalledViaHomebrew checks if the CLI was installed via Homebrew
func isInstalledViaHomebrew(execPath string) bool {
	// First, check if brew is installed
	brewPath, err := exec.LookPath("brew")
	if err != nil {
		// brew is not installed
		return false
	}

	// Get the Homebrew prefix
	cmd := exec.Command(brewPath, "--prefix")
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	brewPrefix := strings.TrimSpace(string(output))
	if brewPrefix == "" {
		return false
	}

	// Check if the executable path is under the Homebrew prefix
	if !strings.HasPrefix(execPath, brewPrefix) {
		return false
	}

	// Check if the blaxel-ai/blaxel tap exists
	cmd = exec.Command(brewPath, "tap")
	output, err = cmd.Output()
	if err != nil {
		return false
	}

	taps := strings.Split(string(output), "\n")
	for _, tap := range taps {
		if strings.TrimSpace(tap) == "blaxel-ai/blaxel" {
			return true
		}
	}

	// If tap doesn't exist but binary is in brew prefix, still consider it a brew installation
	// This handles the case where the tap might be removed but the binary still exists
	return true
}

// runUpgrade executes the appropriate upgrade command based on installation method
func runUpgrade(targetVersion string, force bool) error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	if resolved, err := filepath.EvalSymlinks(executable); err == nil {
		executable = resolved
	}
	method, err := detectInstallationMethod()
	if err != nil {
		return err
	}

	core.PrintInfo(fmt.Sprintf("Detected installation method: %s", method))

	switch method {
	case "brew":
		err = upgradeViaBrew(force)
	case "curl":
		err = upgradeViaCurl(targetVersion)
	default:
		return fmt.Errorf("unknown installation method: %s", method)
	}
	if err != nil {
		return err
	}

	refreshUpgradedSetup(upgradedExecutable(executable, method))
	return nil
}

// Resolve the stable opt link after brew has replaced (and possibly removed)
// the running keg. Manual upgrades replace blaxel beside the running binary.
func upgradedExecutable(executable, method string) string {
	if method == "brew" {
		if prefix, _ := agentsetup.HomebrewSkillsLocation(executable); prefix != "" {
			return filepath.Join(prefix, "opt", "blaxel", "bin", "blaxel")
		}
	}
	name := "blaxel"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return filepath.Join(filepath.Dir(executable), name)
}

func refreshUpgradedSetup(executable string) {
	if skills, mcp := automaticSetupOffers(os.Getenv); !skills && !mcp {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), setupRefreshSkillsTimeout+setupRefreshMCPTimeout+10*time.Second)
	defer cancel()
	probeCtx, probeCancel := context.WithTimeout(ctx, 5*time.Second)
	probe := exec.CommandContext(probeCtx, executable, "setup", "--refresh-check")
	probe.Env = append(os.Environ(), "BL_INSTALL_SETUP=false", "BL_INSTALL_SKILLS=false", "BL_INSTALL_MCP=false", "DO_NOT_TRACK=1")
	probe.WaitDelay = time.Second
	output, probeErr := probe.Output()
	probeCancel()
	if probeErr != nil || strings.TrimSpace(string(output)) != setupRefreshCapability {
		fmt.Fprintln(os.Stderr, "The installed CLI does not support headless setup refresh; setup and consent settings were left unchanged.")
		return
	}
	cmd := exec.CommandContext(ctx, executable, "setup", "--yes", "--skip-login")
	cmd.Env = append(os.Environ(), setupRefreshEnv+"=true")
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	cmd.WaitDelay = time.Second
	if err := cmd.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "Could not refresh Blaxel setup with the upgraded CLI; retry with bl setup.")
	}
}

// upgradeViaBrew upgrades the CLI using Homebrew
func upgradeViaBrew(force bool) error {
	core.PrintInfo("Updating Blaxel tap...")

	// Get the tap repository path
	tapPathCmd := exec.Command("brew", "--repository", "blaxel-ai/blaxel")
	tapPathOutput, err := tapPathCmd.Output()
	if err == nil {
		tapPath := strings.TrimSpace(string(tapPathOutput))

		// Checkout main branch
		gitCheckoutCmd := exec.Command("git", "checkout", "main")
		gitCheckoutCmd.Dir = tapPath
		if err := gitCheckoutCmd.Run(); err != nil {
			core.PrintWarning("Failed to checkout main branch, continuing with upgrade...")
		}

		// Pull latest changes from the tap
		gitPullCmd := exec.Command("git", "pull")
		gitPullCmd.Dir = tapPath
		if err := gitPullCmd.Run(); err != nil {
			core.PrintWarning("Failed to update tap, continuing with upgrade...")
		}
	}

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
	targetVersion = normalizeUpgradeVersion(targetVersion)

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
	if runtime.GOOS == "windows" {
		return upgradeViaPowerShell(targetVersion, binDir)
	}

	// Check if we need sudo (if binary is in a system directory)
	needsSudo := needsSudoForPath(binDir)

	// Build the install command
	installScriptURL := "https://raw.githubusercontent.com/blaxel-ai/toolkit/main/install.sh"

	shellCmd := buildCurlUpgradeCommand(installScriptURL, targetVersion, binDir, needsSudo)
	if targetVersion != "" {
		core.PrintInfo(fmt.Sprintf("Upgrading to version %s...", targetVersion))
	} else {
		core.PrintInfo("Upgrading to latest version...")
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

// buildCurlUpgradeCommand builds the shell command that re-runs install.sh.
// Setup is disabled in the script because the new binary refreshes it itself,
// so it runs as the current user even when the
// script needs sudo.
func buildCurlUpgradeCommand(installScriptURL, targetVersion, binDir string, needsSudo bool) string {
	// The installer must not run interactive setup or a second refresh.
	env := "BL_INSTALL_SETUP=false " + agentsetup.SkillsInstallEnv + "=false"
	if targetVersion != "" {
		env += " VERSION=" + targetVersion
	}
	env += " BINDIR=" + binDir

	shell := "sh"
	if needsSudo {
		shell = "sudo -E sh"
	}

	return fmt.Sprintf("curl -fsSL %s | %s %s", installScriptURL, env, shell)
}

func normalizeUpgradeVersion(targetVersion string) string {
	trimmedVersion := strings.TrimSpace(targetVersion)
	if bareSemverUpgradeVersionPattern.MatchString(trimmedVersion) {
		return "v" + trimmedVersion
	}

	return trimmedVersion
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

// installPS1URL is the install script bl upgrade runs on Windows. Test builds
// can point it elsewhere with -ldflags "-X github.com/blaxel-ai/toolkit/cli.installPS1URL=...".
var installPS1URL = "https://raw.githubusercontent.com/blaxel-ai/toolkit/main/install.ps1"

// upgradeViaPowerShell re-runs install.ps1, since Windows has no sh. The script
// installs into the folder bl runs from and moves the running bl.exe aside, so
// it can be replaced; runUpgrade refreshes the skills itself.
func upgradeViaPowerShell(targetVersion, binDir string) error {
	if targetVersion != "" {
		core.PrintInfo(fmt.Sprintf("Upgrading to version %s...", targetVersion))
	} else {
		core.PrintInfo("Upgrading to latest version...")
	}

	// Windows PowerShell comes with every Windows, and the script needs nothing newer.
	cmd := exec.Command("powershell.exe", "-NoProfile", "-Command", buildPowerShellUpgradeCommand(installPS1URL, targetVersion, binDir))
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("upgrade failed: %w", err)
	}

	core.PrintSuccess("Blaxel CLI upgraded successfully")
	return nil
}

// buildPowerShellUpgradeCommand builds the PowerShell command that runs
// install.ps1 with parameters, which irm | iex can't pass. bl upgrade never
// re-runs bl setup. Stop makes a failed download fail the command.
func buildPowerShellUpgradeCommand(scriptURL, targetVersion, binDir string) string {
	command := "$ErrorActionPreference = 'Stop'; " +
		"[Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12; " +
		fmt.Sprintf("& ([scriptblock]::Create((Invoke-RestMethod -UseBasicParsing -Uri %s))) -InstallDir %s -SkipSetup", powerShellQuote(scriptURL), powerShellQuote(binDir))
	if targetVersion != "" {
		command += " -Version " + powerShellQuote(targetVersion)
	}
	// The script reports a failure through LASTEXITCODE.
	return command + "; exit $LASTEXITCODE"
}

// powerShellQuote quotes s as a PowerShell string literal. PowerShell also
// reads the typographic single quotes as quotes, so those are doubled too.
var powerShellQuoteEscaper = strings.NewReplacer("'", "''", "\u2018", "\u2018\u2018", "\u2019", "\u2019\u2019", "\u201a", "\u201a\u201a", "\u201b", "\u201b\u201b")

func powerShellQuote(s string) string {
	return "'" + powerShellQuoteEscaper.Replace(s) + "'"
}

package cli

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

// The reviewed lockfile pins the installer and every transitive dependency.
// npm ci verifies these SHA512 values before the installer is executed.
//
//go:embed skillsinstaller/package.json
var skillsPackageJSON []byte

//go:embed skillsinstaller/package-lock.json
var skillsPackageLock []byte

func init() {
	core.RegisterCommand("skills", SkillsCmd)
}

func SkillsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use: "skills", Short: "Manage Blaxel skills for coding agents",
		PersistentPreRunE: func(_ *cobra.Command, _ []string) error { return nil },
	}
	cmd.AddCommand(&cobra.Command{
		Use: "install", Short: "Install or refresh Blaxel skills for coding agents",
		Long: "Install Blaxel agent skills globally with a version-pinned, integrity-checked installer.\nRequires Node.js 22.20.0 or later and npm. This explicit command runs even when\nautomatic installation is disabled with BL_INSTALL_SKILLS=false or in CI.",
		Args: cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error { return installSkillsOnce() },
	})
	return cmd
}

func installPinnedSkills(ctx context.Context, output io.Writer) error {
	node, err := exec.LookPath("node")
	if err != nil {
		return fmt.Errorf("node (22.20.0 or later) and npm are required: %w", err)
	}
	npm, err := exec.LookPath("npm")
	if err != nil {
		return fmt.Errorf("npm is required: %w", err)
	}
	return runPinnedSkills(ctx, node, npm, output, func(cmd *exec.Cmd) error { return cmd.Run() })
}

// Preserve executable shims and their symlinks on Unix and for native Windows
// executables. Only Windows batch launchers need npm's JS entry point because
// they cannot be executed directly without invoking a command shell.
func skillsNPMCommand(node, npm string) ([]string, error) {
	npm, err := filepath.Abs(npm)
	if err != nil {
		return nil, err
	}
	extension := strings.ToLower(filepath.Ext(npm))
	if extension != ".cmd" && extension != ".bat" {
		return []string{npm}, nil
	}
	resolved, err := filepath.EvalSymlinks(npm)
	if err != nil {
		return nil, err
	}
	entry := filepath.Join(filepath.Dir(resolved), "node_modules", "npm", "bin", "npm-cli.js")
	if _, err := os.Stat(entry); err != nil {
		return nil, fmt.Errorf("could not locate npm CLI: %w", err)
	}
	return []string{node, entry}, nil
}

func runPinnedSkills(ctx context.Context, node, npm string, output io.Writer, run func(*exec.Cmd) error) error {
	npmCommand, err := skillsNPMCommand(node, npm)
	if err != nil {
		return err
	}
	directory, err := os.MkdirTemp("", "blaxel-skills-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(directory) }()
	// npm treats a symlinked prefix (such as macOS /var) as another dependency.
	resolvedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return err
	}
	directory = resolvedDirectory
	for name, data := range map[string][]byte{
		"package.json": skillsPackageJSON, "package-lock.json": skillsPackageLock, "npmrc": {}, "global-npmrc": {},
	} {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			return err
		}
	}
	env := skillsInstallerEnvironment(os.Environ())
	commands := [][]string{
		append(npmCommand, "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--engine-strict", "--registry=https://registry.npmjs.org",
			"--userconfig="+filepath.Join(directory, "npmrc"), "--globalconfig="+filepath.Join(directory, "global-npmrc"),
			"--cache="+filepath.Join(directory, "npm-cache"), "--prefix="+directory),
		{node, filepath.Join(directory, "node_modules", "skills", "bin", "cli.mjs"), "add", skillsRepo, "-g", "--all"},
	}
	for index, args := range commands {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		// Retain the caller's directory so version-manager shims can select Node.
		// --prefix and absolute entry/config paths keep all npm writes isolated.
		cmd.Env = env
		cmd.Stdout, cmd.Stderr = output, output
		cmd.WaitDelay = time.Second
		if err := run(cmd); err != nil {
			if index == 0 {
				return fmt.Errorf("preparing verified skills installer: %w", err)
			}
			return fmt.Errorf("installing skills: %w", err)
		}
	}
	return nil
}

// Do not let inherited Node hooks or npm configuration change the pinned
// installation. Preserve HOME and agent configuration for the intended target.
func skillsInstallerEnvironment(environ []string) []string {
	env := make([]string, 0, len(environ))
	for _, entry := range environ {
		key, _, _ := strings.Cut(entry, "=")
		key = strings.ToUpper(key)
		if key == "NODE_OPTIONS" || key == "NODE_PATH" || strings.HasPrefix(key, "NPM_CONFIG_") {
			continue
		}
		env = append(env, entry)
	}
	return env
}

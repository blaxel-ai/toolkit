package cli

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
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
		Long:         "Install Blaxel agent skills globally with a version-pinned, integrity-checked installer.\nSkills go to ~/.agents/skills and to the coding agents detected on this machine\n(Claude Code, Codex, Cursor, ...). Requires Node.js 22.20.0 or later, npm and git.\nThis explicit command runs even when automatic installation is disabled with\nBL_INSTALL_SKILLS=false or in CI.",
		Args:         cobra.NoArgs,
		RunE:         func(_ *cobra.Command, _ []string) error { return installSkillsOnce() },
		SilenceUsage: true, SilenceErrors: true,
	})
	return cmd
}

// skillsInstallResult summarizes a successful installation for the user.
type skillsInstallResult struct {
	skills []string
	agents []string
}

const skillsMinimumNode = "22.20.0"

func installPinnedSkills(ctx context.Context) (skillsInstallResult, error) {
	node, err := exec.LookPath("node")
	if err != nil {
		return skillsInstallResult{}, fmt.Errorf("requires Node.js %s or later and npm (https://nodejs.org)", skillsMinimumNode)
	}
	npm, err := exec.LookPath("npm")
	if err != nil {
		return skillsInstallResult{}, fmt.Errorf("requires npm alongside Node.js")
	}
	if _, err := exec.LookPath("git"); err != nil {
		return skillsInstallResult{}, fmt.Errorf("requires git to download the skills")
	}
	if err := checkSkillsNodeVersion(ctx, node); err != nil {
		return skillsInstallResult{}, err
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return skillsInstallResult{}, err
	}
	targets, agents := detectSkillsAgents(home, os.Getenv)
	var log bytes.Buffer
	output, err := runPinnedSkills(ctx, node, npm, targets, &log, func(cmd *exec.Cmd) error { return cmd.Run() })
	if err != nil {
		return skillsInstallResult{}, withSkillsLog(err, log.Bytes())
	}
	skills, err := parseSkillsResult(output)
	if err != nil {
		return skillsInstallResult{}, withSkillsLog(err, log.Bytes())
	}
	return skillsInstallResult{skills: skills, agents: agents}, nil
}

func checkSkillsNodeVersion(ctx context.Context, node string) error {
	cmd := exec.CommandContext(ctx, node, "--version")
	cmd.Env = skillsInstallerEnvironment(os.Environ())
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("could not run node --version: %w", err)
	}
	version := strings.TrimSpace(string(output))
	if !skillsNodeVersionSupported(version) {
		return fmt.Errorf("requires Node.js %s or later (found %s)", skillsMinimumNode, version)
	}
	return nil
}

func skillsNodeVersionSupported(version string) bool {
	parse := func(v string) []int {
		parts := strings.SplitN(strings.TrimPrefix(strings.TrimSpace(v), "v"), ".", 3)
		numbers := make([]int, 3)
		for i := 0; i < len(parts); i++ {
			digits := parts[i]
			end := strings.IndexFunc(digits, func(r rune) bool { return r < '0' || r > '9' })
			if end >= 0 {
				digits = digits[:end]
			}
			n, err := strconv.Atoi(digits)
			if err != nil {
				return nil
			}
			numbers[i] = n
		}
		return numbers
	}
	actual, minimum := parse(version), parse(skillsMinimumNode)
	if actual == nil {
		return false
	}
	for i := range minimum {
		if actual[i] != minimum[i] {
			return actual[i] > minimum[i]
		}
	}
	return true
}

// parseSkillsResult reads the installer's --json report.
func parseSkillsResult(output []byte) ([]string, error) {
	var results []struct {
		Name   string `json:"name"`
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.Unmarshal(bytes.TrimSpace(output), &results); err != nil {
		return nil, fmt.Errorf("unexpected installer output: %w", err)
	}
	var skills, failures []string
	for _, result := range results {
		if result.Status == "failed" || result.Error != "" {
			failures = append(failures, result.Name+": "+result.Error)
			continue
		}
		skills = append(skills, result.Name)
	}
	if len(failures) > 0 {
		return nil, fmt.Errorf("installing skills: %s", strings.Join(failures, "; "))
	}
	if len(skills) == 0 {
		return nil, fmt.Errorf("installing skills: no skills were installed")
	}
	return skills, nil
}

var ansiEscape = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

// withSkillsLog appends the end of the installer output to an error so users
// can see why it failed without the full progress output on success.
func withSkillsLog(err error, log []byte) error {
	lines := strings.Split(strings.TrimSpace(ansiEscape.ReplaceAllString(string(log), "")), "\n")
	var kept []string
	for _, line := range lines {
		if line = strings.TrimRight(line, " \r"); strings.Trim(line, " │") != "" {
			kept = append(kept, line)
		}
	}
	if len(kept) == 0 {
		return err
	}
	if len(kept) > 15 {
		kept = kept[len(kept)-15:]
	}
	return fmt.Errorf("%w\n%s", err, strings.Join(kept, "\n"))
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

// runPinnedSkills prepares the locked installer and installs the skills for
// the given agent targets. Progress goes to log; the JSON report is returned.
func runPinnedSkills(ctx context.Context, node, npm string, agents []string, log io.Writer, run func(*exec.Cmd) error) ([]byte, error) {
	npmCommand, err := skillsNPMCommand(node, npm)
	if err != nil {
		return nil, err
	}
	directory, err := os.MkdirTemp("", "blaxel-skills-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(directory) }()
	// npm treats a symlinked prefix (such as macOS /var) as another dependency.
	resolvedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		return nil, err
	}
	directory = resolvedDirectory
	for name, data := range map[string][]byte{
		"package.json": skillsPackageJSON, "package-lock.json": skillsPackageLock, "npmrc": {}, "global-npmrc": {},
	} {
		if err := os.WriteFile(filepath.Join(directory, name), data, 0600); err != nil {
			return nil, err
		}
	}
	env := skillsInstallerEnvironment(os.Environ())
	var report bytes.Buffer
	install := append([]string{node, filepath.Join(directory, "node_modules", "skills", "bin", "cli.mjs"),
		"add", skillsRepo, "-g", "-y", "--skill", "*", "--json", "--agent"}, agents...)
	commands := [][]string{
		append(npmCommand, "ci", "--ignore-scripts", "--no-audit", "--no-fund", "--engine-strict", "--registry=https://registry.npmjs.org",
			"--userconfig="+filepath.Join(directory, "npmrc"), "--globalconfig="+filepath.Join(directory, "global-npmrc"),
			"--cache="+filepath.Join(directory, "npm-cache"), "--prefix="+directory),
		install,
	}
	for index, args := range commands {
		cmd := exec.CommandContext(ctx, args[0], args[1:]...)
		// Retain the caller's directory so version-manager shims can select Node.
		// --prefix and absolute entry/config paths keep all npm writes isolated.
		cmd.Env = env
		cmd.Stdout, cmd.Stderr = log, log
		if index == 1 {
			cmd.Stdout = &report
		}
		cmd.WaitDelay = time.Second
		if err := run(cmd); err != nil {
			if index == 0 {
				return nil, fmt.Errorf("preparing verified skills installer: %w", err)
			}
			return nil, fmt.Errorf("installing skills: %w", err)
		}
	}
	return report.Bytes(), nil
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

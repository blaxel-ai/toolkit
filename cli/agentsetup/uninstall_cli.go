package agentsetup

import (
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CLIRemovalStep is a command that removes part of the bl CLI, or, without
// one, a step to take by hand.
type CLIRemovalStep struct {
	Command, Note string
}

// CLIInstall describes the running bl, to tell how to remove it.
type CLIInstall struct {
	// Executable is the running bl, as os.Executable returns it.
	Executable, Home, GOOS string
	Env                    func(string) string
	// Sudo reports a folder that needs sudo to change.
	Sudo func(string) bool
}

const installerMarker = "# Added by the Blaxel installer"

// CLIRemovalSteps returns how to remove the bl CLI, for the way it was
// installed: Homebrew, install.sh, install.ps1 or a binary copied by hand.
// bl uninstall prints them and never deletes the binary that is running.
func CLIRemovalSteps(install CLIInstall) []CLIRemovalStep {
	var steps []CLIRemovalStep
	resolved, err := filepath.EvalSymlinks(install.Executable)
	if err != nil {
		resolved = install.Executable
	}
	switch prefix, _ := HomebrewSkillsLocation(resolved); {
	case prefix != "":
		steps = append(steps, CLIRemovalStep{Command: "brew uninstall blaxel"}, CLIRemovalStep{Command: "brew untap blaxel-ai/blaxel"})
	case install.GOOS == "windows":
		steps = append(steps, windowsCLIRemoval(install)...)
	default:
		steps = append(steps, unixCLIRemoval(install)...)
	}
	if install.GOOS == "windows" {
		return append(steps, CLIRemovalStep{Command: "Remove-Item -Recurse -LiteralPath " + powerShellQuote(filepath.Join(install.Home, ".blaxel")),
			Note: "your Blaxel logins and settings"})
	}
	return append(steps, CLIRemovalStep{Command: "rm -rf " + shellPath(install.Home, filepath.Join(install.Home, ".blaxel")),
		Note: "your Blaxel logins and settings"})
}

// install.sh puts bl and blaxel in ~/.local/bin, shell completions where each
// shell loads them, and PATH lines in the shell's startup file.
func unixCLIRemoval(install CLIInstall) []CLIRemovalStep {
	var steps []CLIRemovalStep
	dir := filepath.Dir(install.Executable)
	if files := existing(dir, "bl", "blaxel"); len(files) > 0 {
		command := "rm " + shellPaths(install.Home, files)
		if install.Sudo != nil && install.Sudo(dir) {
			command = "sudo " + command
		}
		steps = append(steps, CLIRemovalStep{Command: command})
	}
	envOr := func(key, fallback string) string {
		if value := strings.TrimSpace(install.Env(key)); value != "" {
			return value
		}
		return fallback
	}
	completions := existing("",
		filepath.Join(envOr("ZSH_COMPLETION_DIR", filepath.Join(install.Home, ".zsh", "completions")), "_bl"),
		filepath.Join(envOr("BASH_COMPLETION_USER_DIR", filepath.Join(install.Home, ".local", "share", "bash-completion", "completions")), "bl"),
		filepath.Join(install.Home, ".config", "fish", "completions", "bl.fish"))
	if len(completions) > 0 {
		steps = append(steps, CLIRemovalStep{Command: "rm " + shellPaths(install.Home, completions), Note: "shell completions"})
	}
	for _, name := range []string{".zshrc", ".bashrc", ".bash_profile", ".profile", filepath.Join(".config", "fish", "config.fish")} {
		file := filepath.Join(install.Home, name)
		if data, err := os.ReadFile(file); err == nil && bytes.Contains(data, []byte(installerMarker)) {
			steps = append(steps, CLIRemovalStep{Note: "In " + shellPath(install.Home, file) + ", delete the lines under \"" + installerMarker + "\""})
		}
	}
	return steps
}

// install.ps1 puts bl.exe and blaxel.exe in %LOCALAPPDATA%\blaxel, or the
// folder bl upgrade runs from, and adds that folder to the user's Path.
func windowsCLIRemoval(install CLIInstall) []CLIRemovalStep {
	var steps []CLIRemovalStep
	dir := filepath.Dir(install.Executable)
	installer := strings.TrimSpace(install.Env("LOCALAPPDATA")) != "" && strings.EqualFold(filepath.Clean(dir), filepath.Join(install.Env("LOCALAPPDATA"), "blaxel"))
	if installer {
		steps = append(steps, CLIRemovalStep{Command: "Remove-Item -Recurse -LiteralPath " + powerShellQuote(dir)})
	} else if files := existing(dir, "bl.exe", "blaxel.exe"); len(files) > 0 {
		quoted := make([]string, len(files))
		for i, file := range files {
			quoted[i] = powerShellQuote(file)
		}
		steps = append(steps, CLIRemovalStep{Command: "Remove-Item -LiteralPath " + strings.Join(quoted, ", ")})
	}
	onPath := false
	for _, entry := range filepath.SplitList(install.Env("PATH")) {
		onPath = onPath || strings.EqualFold(filepath.Clean(entry), filepath.Clean(dir))
	}
	if installer || onPath {
		steps = append(steps, CLIRemovalStep{
			Command: "[Environment]::SetEnvironmentVariable('Path', (([Environment]::GetEnvironmentVariable('Path', 'User') -split ';') | Where-Object { $_ -ne " +
				powerShellQuote(dir) + " }) -join ';', 'User')",
			Note: "take it off your Path"})
	}
	return steps
}

// existing returns the files that exist, joined to dir.
func existing(dir string, names ...string) []string {
	var files []string
	for _, name := range names {
		file := filepath.Join(dir, name)
		if _, err := os.Lstat(file); err == nil {
			files = append(files, file)
		}
	}
	return files
}

var shellSafe = regexp.MustCompile(`^[A-Za-z0-9_@%+=:,./-]+$`)

// shellPath writes a path for a POSIX shell: ~/... in the home folder, quoted
// when it needs to be.
func shellPath(home, path string) string {
	if relative, err := filepath.Rel(home, path); err == nil && !strings.HasPrefix(relative, "..") && shellSafe.MatchString(relative) {
		return "~/" + filepath.ToSlash(relative)
	}
	if shellSafe.MatchString(path) {
		return path
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

func shellPaths(home string, paths []string) string {
	quoted := make([]string, len(paths))
	for i, path := range paths {
		quoted[i] = shellPath(home, path)
	}
	return strings.Join(quoted, " ")
}

func powerShellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
)

// FindPythonEntryFile searches for common Python entry files in the given folder
// Returns the relative path to the entry file if found, empty string otherwise
func FindPythonEntryFile(folder string) string {
	files := []string{
		"app.py",
		"main.py",
		"api.py",
		"app/main.py",
		"app/app.py",
		"app/api.py",
		"src/main.py",
		"src/app.py",
		"src/api.py",
	}
	for _, f := range files {
		if _, err := os.Stat(filepath.Join(folder, f)); err == nil {
			return f
		}
	}
	return ""
}

// HasPythonEntryFile checks if common Python entry files exist in the given folder
func HasPythonEntryFile(folder string) bool {
	return FindPythonEntryFile(folder) != ""
}

// FindPythonExecutable checks for python or python3 in PATH and returns the executable name
// Returns an error if neither python nor python3 is available
func FindPythonExecutable() (string, error) {
	// First try "python"
	if _, err := exec.LookPath("python"); err == nil {
		return "python", nil
	}
	// Then try "python3"
	if _, err := exec.LookPath("python3"); err == nil {
		return "python3", nil
	}
	// Neither found
	return "", fmt.Errorf("python is not available on this system")
}

func StartPythonServer(port int, host string, hotreload bool, folder string, config core.Config) *exec.Cmd {
	// Check if Python is available before attempting to start
	pythonExec, err := FindPythonExecutable()
	if err != nil {
		core.PrintError("Serve", err)
		core.PrintInfo("Please install Python:")
		core.PrintInfo("  - macOS: brew install python3")
		core.PrintInfo("  - Linux: sudo apt-get install python3 (or use your distribution's package manager)")
		core.PrintInfo("  - Windows: Download from https://www.python.org/downloads/")
		core.PrintInfo("After installation, ensure Python is in your PATH.")
		core.ExitWithError(err)
	}
	_ = pythonExec // Will be used by findPythonRootCmdAsString
	python, err := FindRootCmd(port, host, hotreload, folder, config)
	if err != nil {
		core.PrintError("Serve", err)
		core.ExitWithError(err)
	}
	// Extract the actual command, hiding "sh -c" wrapper if present
	cmdDisplay := strings.Join(python.Args, " ")
	if len(python.Args) >= 3 && python.Args[0] == "sh" && python.Args[1] == "-c" {
		// Extract just the command after "sh -c"
		cmdDisplay = python.Args[2]
	}
	fmt.Printf("Starting server : %s\n", cmdDisplay)
	if os.Getenv("COMMAND") != "" {
		command := strings.Split(os.Getenv("COMMAND"), " ")
		if len(command) > 1 {
			python = exec.Command(command[0], command[1:]...)
		} else {
			python = exec.Command(command[0])
		}
	}
	python.Stdout = os.Stdout
	python.Stderr = os.Stderr
	python.Dir = folder

	// Set env variables
	envs := GetServerEnvironment(port, host, hotreload, config)
	python.Env = envs.ToEnv()

	err = python.Start()
	if err != nil {
		err = fmt.Errorf("failed to start Python server: %w", err)
		core.PrintError("Serve", err)
		core.ExitWithError(err)
	}

	return python
}

func findPythonRootCmdAsString(cfg RootCmdConfig) ([]string, error) {
	if cfg.Entrypoint.Production != "" || cfg.Entrypoint.Development != "" {
		if cfg.Hotreload && cfg.Entrypoint.Development != "" {
			return strings.Split(cfg.Entrypoint.Development, " "), nil
		}
		return strings.Split(cfg.Entrypoint.Production, " "), nil
	}
	fmt.Println("Entrypoint not found in config, using auto-detection")

	file := FindPythonEntryFile(cfg.Folder)
	if file == "" {
		return nil, fmt.Errorf("app.py or main.py not found in current directory")
	}

	venv := ".venv"
	if _, err := os.Stat(filepath.Join(cfg.Folder, venv)); err == nil {
		// Check if venv python exists, otherwise fall back to system python
		venvPython := filepath.Join(venv, "bin", "python")
		if _, err := os.Stat(filepath.Join(cfg.Folder, venvPython)); err == nil {
			cmd := []string{venvPython, file}
			return cmd, nil
		}
		// Venv exists but python binary not found, try python3
		venvPython3 := filepath.Join(venv, "bin", "python3")
		if _, err := os.Stat(filepath.Join(cfg.Folder, venvPython3)); err == nil {
			cmd := []string{venvPython3, file}
			return cmd, nil
		}
		// Fall through to system python
	}

	// Use system python (python or python3)
	pythonExec, err := FindPythonExecutable()
	if err != nil {
		return nil, err
	}
	return []string{pythonExec, file}, nil
}

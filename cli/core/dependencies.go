package core

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func installPythonDependencies(directory string) error {
	// First, try to use uv
	if isCommandAvailable("uv") {
		uvSyncCmd := exec.Command("uv", "sync", "--refresh")
		uvSyncCmd.Dir = directory

		// Capture both stdout and stderr
		output, err := uvSyncCmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to run uv sync: %w\nOutput: %s", err, string(output))
		}
		return nil
	}

	// If uv is not available, try pip
	if isCommandAvailable("python") {
		// Check for requirements.txt or pyproject.toml
		requirementsPath := filepath.Join(directory, "requirements.txt")
		pyprojectPath := filepath.Join(directory, "pyproject.toml")
		venvPath := filepath.Join(directory, ".venv")

		// Create virtual environment if it doesn't exist
		if _, err := os.Stat(venvPath); os.IsNotExist(err) {
			// Try python3 first, then python
			var venvCreateCmd *exec.Cmd
			if isCommandAvailable("python3") {
				venvCreateCmd = exec.Command("python3", "-m", "venv", ".venv")
			} else if isCommandAvailable("python") {
				venvCreateCmd = exec.Command("python", "-m", "venv", ".venv")
			} else {
				return fmt.Errorf("neither python3 nor python command found")
			}

			venvCreateCmd.Dir = directory

			output, err := venvCreateCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to create virtual environment: %w\nOutput: %s", err, string(output))
			}

			// Verify the virtual environment was created
			if _, err := os.Stat(venvPath); err != nil {
				return fmt.Errorf("virtual environment directory was not created at %s", venvPath)
			}
		}

		// Determine the python executable path in the virtual environment
		var pythonPath string
		// Try multiple possible python executable names and locations
		possiblePaths := []string{
			filepath.Join(venvPath, "bin", "python"),
			filepath.Join(venvPath, "bin", "python3"),
			filepath.Join(venvPath, "Scripts", "python.exe"),
			filepath.Join(venvPath, "Scripts", "python3.exe"),
		}

		for _, path := range possiblePaths {
			if absPath, err := filepath.Abs(path); err == nil {
				if _, err := os.Stat(absPath); err == nil {
					pythonPath = absPath
					break
				}
			}
		}

		if pythonPath == "" {
			return fmt.Errorf("could not find python executable in virtual environment at %s", venvPath)
		}

		if _, err := os.Stat(pyprojectPath); err == nil {
			// If pyproject.toml exists, try pip install -e .
			pipInstallCmd := exec.Command(pythonPath, "-m", "pip", "install", "-e", ".")
			pipInstallCmd.Dir = directory

			output, err := pipInstallCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to run pip install -e . in virtual environment: %w\nOutput: %s", err, string(output))
			}
			return nil
		} else if _, err := os.Stat(requirementsPath); err == nil {
			// If requirements.txt exists, use pip install -r requirements.txt
			pipInstallCmd := exec.Command(pythonPath, "-m", "pip", "install", "-r", "requirements.txt")
			pipInstallCmd.Dir = directory

			output, err := pipInstallCmd.CombinedOutput()
			if err != nil {
				return fmt.Errorf("failed to run pip install -r requirements.txt in virtual environment: %w\nOutput: %s", err, string(output))
			}
			return nil
		} else {
			return fmt.Errorf("neither pyproject.toml nor requirements.txt found in %s", directory)
		}
	}

	// If neither uv nor pip is available, return a clear error
	//nolint:staticcheck
	return fmt.Errorf(`neither 'uv' nor 'pip' is available on your system.

To install uv (recommended):
  curl -LsSf https://astral.sh/uv/install.sh | sh

Or to install pip:
  - On macOS: pip is usually pre-installed with Python
  - On Ubuntu/Debian: sudo apt-get install python3-pip
  - On RHEL/CentOS: sudo yum install python3-pip
  - On Windows: pip is included with Python from python.org

Please install one of these tools and try again.`)
}

// isCommandAvailable checks if a command is available in the system PATH
func isCommandAvailable(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

func installTypescriptDependencies(directory string) error {
	npmInstallCmd := exec.Command("npx", "pnpm", "install")
	npmInstallCmd.Dir = directory

	// Capture both stdout and stderr
	output, err := npmInstallCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run pnpm install: %w\nOutput: %s", err, string(output))
	}
	return nil
}

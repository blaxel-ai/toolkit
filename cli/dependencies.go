package cli

import (
	"fmt"
	"os/exec"
)

func installPythonDependencies(directory string) error {
	uvSyncCmd := exec.Command("uv", "sync", "--refresh")
	uvSyncCmd.Dir = directory

	// Capture both stdout and stderr
	output, err := uvSyncCmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run uv sync: %w\nOutput: %s", err, string(output))
	}
	return nil
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

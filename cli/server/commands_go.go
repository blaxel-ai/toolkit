package server

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
)

// FindGoExecutable checks for go in PATH and returns the executable name
// Returns an error if go is not available
func FindGoExecutable() (string, error) {
	if _, err := exec.LookPath("go"); err == nil {
		return "go", nil
	}
	return "", fmt.Errorf("go is not available on this system")
}

func StartGoServer(port int, host string, hotreload bool, folder string, config core.Config) *exec.Cmd {
	// Check if Go is available before attempting to start
	goExec, err := FindGoExecutable()
	if err != nil {
		core.PrintError("Serve", err)
		core.PrintInfo("Please install Go:")
		core.PrintInfo("  - macOS: brew install go")
		core.PrintInfo("  - Linux: sudo apt-get install golang-go (or use your distribution's package manager)")
		core.PrintInfo("  - Windows: Download from https://go.dev/dl/")
		core.PrintInfo("After installation, ensure Go is in your PATH.")
		os.Exit(1)
	}
	_ = goExec // Will be used by findGoRootCmdAsString

	golang, err := FindRootCmd(port, host, hotreload, folder, config)
	if err != nil {
		core.PrintError("Serve", err)
		os.Exit(1)
	}
	// Extract the actual command, hiding "sh -c" wrapper if present
	cmdDisplay := strings.Join(golang.Args, " ")
	if len(golang.Args) >= 3 && golang.Args[0] == "sh" && golang.Args[1] == "-c" {
		// Extract just the command after "sh -c"
		cmdDisplay = golang.Args[2]
	}
	fmt.Printf("Starting server : %s\n", cmdDisplay)
	if os.Getenv("COMMAND") != "" {
		command := strings.Split(os.Getenv("COMMAND"), " ")
		if len(command) > 1 {
			golang = exec.Command(command[0], command[1:]...)
		} else {
			golang = exec.Command(command[0])
		}
	}
	golang.Stdout = os.Stdout
	golang.Stderr = os.Stderr
	golang.Dir = folder

	// Set env variables
	envs := GetServerEnvironment(port, host, hotreload, config)
	golang.Env = envs.ToEnv()

	err = golang.Start()
	if err != nil {
		core.PrintError("Serve", fmt.Errorf("failed to start Go server: %w", err))
		os.Exit(1)
	}

	return golang
}

func findGoRootCmdAsString(cfg RootCmdConfig) ([]string, error) {
	if cfg.Entrypoint.Production != "" || cfg.Entrypoint.Development != "" {
		if cfg.Hotreload && cfg.Entrypoint.Development != "" {
			return strings.Split(cfg.Entrypoint.Development, " "), nil
		}
		return strings.Split(cfg.Entrypoint.Production, " "), nil
	}
	return nil, fmt.Errorf("entrypoint not found in config")
}

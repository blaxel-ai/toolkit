package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/beamlit/toolkit/cli/dockerfiles"
)

func getPythonDockerfile() (string, error) {
	entrypoint, err := findRootCmdAsString(RootCmdConfig{
		Hotreload:  false,
		Production: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to find root command: %w", err)
	}
	data := map[string]interface{}{
		"BaseImage":      "python:3.12-alpine",
		"LockFile":       findPythonPackageManagerLockFile(),
		"InstallCommand": strings.Join(findPythonPackageManagerCommandAsString(true), " "),
		"Entrypoint":     strings.Join(entrypoint, "\", \""),
	}

	tmpl, err := template.New("dockerfile").Parse(dockerfiles.PythonTemplate)
	if err != nil {
		return "", fmt.Errorf("failed to parse dockerfile template: %w", err)
	}

	var result strings.Builder
	if err := tmpl.Execute(&result, data); err != nil {
		return "", fmt.Errorf("failed to execute dockerfile template: %w", err)
	}

	return result.String(), nil
}

func startPythonServer(port int, host string, hotreload bool) *exec.Cmd {
	py, err := findRootCmd(hotreload)
	fmt.Printf("Starting server : %s\n", strings.Join(py.Args, " "))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if os.Getenv("COMMAND") != "" {
		command := strings.Split(os.Getenv("COMMAND"), " ")
		if len(command) > 1 {
			py = exec.Command(command[0], command[1:]...)
		} else {
			py = exec.Command(command[0])
		}
	}
	py.Stdout = os.Stdout
	py.Stderr = os.Stderr

	// Set env variables
	envs := getServerEnvironment(port, host)
	py.Env = envs.ToEnv()

	err = py.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return py
}

func findPythonRootCmdAsString(config RootCmdConfig) ([]string, error) {
	if _, err := os.Stat("app.py"); err == nil {
		return []string{"python", "app.py"}, nil
	}
	if _, err := os.Stat("main.py"); err == nil {
		return []string{"python", "main.py"}, nil
	}
	return nil, fmt.Errorf("app.py or main.py not found in current directory")
}

func findPythonPackageManager() string {
	lockFile := findPythonPackageManagerLockFile()
	switch lockFile {
	case "uv.lock":
		return "uv"
	default:
		return "pip"
	}
}

func findPythonPackageManagerLockFile() string {
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(currentDir, "uv.lock")); err == nil {
		return "uv.lock"
	}
	return ""
}

func findPythonPackageManagerCommandAsString(production bool) []string {
	// Production mode is not supported for now cause we build in onestage
	packageManager := findPythonPackageManager()
	if packageManager == "uv" {
		baseCmd := []string{"pip", "install", "uv", "&&", "uv", "sync", "--refresh"}
		return baseCmd
	}
	return []string{"pip", "install", "-r", "requirements.txt"}
}

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func startPythonServer(port int, host string, hotreload bool) *exec.Cmd {
	py, err := findRootCmd(port, host, hotreload)
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
	envs := getServerEnvironment(port, host, hotreload)
	py.Env = envs.ToEnv()

	err = py.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return py
}

func findPythonRootCmdAsString(cfg RootCmdConfig) ([]string, error) {
	if config.Entrypoint.Production != "" || config.Entrypoint.Development != "" {
		if cfg.Hotreload && config.Entrypoint.Development != "" {
			return strings.Split(config.Entrypoint.Development, " "), nil
		}
		return strings.Split(config.Entrypoint.Production, " "), nil
	}
	fmt.Println("Entrypoint not found in config, using auto-detection")
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
	file := ""
	for _, f := range files {
		if _, err := os.Stat(f); err == nil {
			file = f
			break
		}
	}
	if file == "" {
		return nil, fmt.Errorf("app.py or main.py not found in current directory")
	}
	venv := ".venv"
	if _, err := os.Stat(venv); err == nil {
		cmd := []string{filepath.Join(venv, "bin", "python"), file}
		return cmd, nil
	}
	return []string{"python", file}, nil
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

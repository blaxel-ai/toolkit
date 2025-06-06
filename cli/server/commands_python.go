package server

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
)

func StartPythonServer(port int, host string, hotreload bool, folder string, config core.Config) *exec.Cmd {
	python, err := FindRootCmd(port, host, hotreload, folder, config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Starting server : %s\n", strings.Join(python.Args, " "))
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
		fmt.Println(err)
		os.Exit(1)
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
		if _, err := os.Stat(filepath.Join(cfg.Folder, f)); err == nil {
			file = f
			break
		}
	}
	if file == "" {
		return nil, fmt.Errorf("app.py or main.py not found in current directory")
	}
	venv := ".venv"
	if _, err := os.Stat(filepath.Join(cfg.Folder, venv)); err == nil {
		cmd := []string{filepath.Join(venv, "bin", "python"), file}
		return cmd, nil
	}
	return []string{"python", file}, nil
}

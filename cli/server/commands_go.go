package server

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
)

func StartGoServer(port int, host string, hotreload bool, folder string, config core.Config) *exec.Cmd {
	golang, err := FindRootCmd(port, host, hotreload, folder, config)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Starting server : %s\n", strings.Join(golang.Args, " "))
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
		fmt.Println(err)
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

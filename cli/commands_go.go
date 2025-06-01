package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func startGoServer(port int, host string, hotreload bool) *exec.Cmd {
	golang, err := findRootCmd(port, host, hotreload)
	fmt.Printf("Starting server : %s\n", strings.Join(golang.Args, " "))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
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
	envs := getServerEnvironment(port, host, hotreload)
	golang.Env = envs.ToEnv()

	err = golang.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return golang
}

func findGoRootCmdAsString(cfg RootCmdConfig) ([]string, error) {
	if config.Entrypoint.Production != "" || config.Entrypoint.Development != "" {
		if cfg.Hotreload && config.Entrypoint.Development != "" {
			return strings.Split(config.Entrypoint.Development, " "), nil
		}
		return strings.Split(config.Entrypoint.Production, " "), nil
	}
	return nil, fmt.Errorf("entrypoint not found in config")
}

package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type PackageJson struct {
	Scripts map[string]string `json:"scripts"`
}

func findTSRootCmd(hotreload bool) (*exec.Cmd, error) {
	rootCmd, err := findTSRootCmdAsString(hotreload)
	if err != nil {
		return nil, fmt.Errorf("error finding ts root cmd: %v", err)
	}
	return exec.Command(rootCmd[0], rootCmd[1:]...), nil
}

func startTypescriptServer(port int, host string, hotreload bool) *exec.Cmd {
	ts, err := findTSRootCmd(hotreload)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	if os.Getenv("COMMAND") != "" {
		command := strings.Split(os.Getenv("COMMAND"), " ")
		if len(command) > 1 {
			ts = exec.Command(command[0], command[1:]...)
		} else {
			ts = exec.Command(command[0])
		}
	}
	ts.Stdout = os.Stdout
	ts.Stderr = os.Stderr

	// Set env variables
	ts.Env = getServerEnvironment(port, host)

	err = ts.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return ts
}

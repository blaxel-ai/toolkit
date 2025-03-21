package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func getPythonDockerfile() (string, error) {
	return `
FROM python:3.11-alpine
WORKDIR /blaxel
COPY pyproject.toml /blaxel/pyproject.toml
RUN pip install -r requirements.txt
`, nil
}

func findPythonRootCmdAsString(config RootCmdConfig) ([]string, error) {
	return []string{"uvicorn", "blaxel.serve.app:app"}, nil
}

func startPythonServer(port int, host string, hotreload bool) *exec.Cmd {
	uvicornCmd := "uvicorn"
	if _, err := os.Stat(".venv"); !os.IsNotExist(err) {
		uvicornCmd = ".venv/bin/uvicorn"
	}

	uvicorn := exec.Command(
		uvicornCmd,
		"blaxel.serve.app:app",
		"--port",
		fmt.Sprintf("%d", port),
		"--host",
		host,
	)
	if hotreload {
		uvicorn.Args = append(uvicorn.Args, "--reload")
	}
	if os.Getenv("COMMAND") != "" {
		command := strings.Split(os.Getenv("COMMAND"), " ")
		if len(command) > 1 {
			uvicorn = exec.Command(command[0], command[1:]...)
		} else {
			uvicorn = exec.Command(command[0])
		}
	}

	uvicorn.Stdout = os.Stdout
	uvicorn.Stderr = os.Stderr

	envs := getServerEnvironment(port, host)
	uvicorn.Env = envs.ToEnv()

	err := uvicorn.Start()
	if err != nil {
		fmt.Printf("Error starting uvicorn server: %v\n", err)
		os.Exit(1)
	}

	return uvicorn
}

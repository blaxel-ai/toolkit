package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func init() {
	if rootCmd.Use == "" {
		fmt.Println("Error: rootCmd not initialized")
		os.Exit(1)
	}
}

type RegisterImpl struct {
}

func findRootCmd(port int, host string, hotreload bool) (*exec.Cmd, error) {
	rootCmd, err := findRootCmdAsString(RootCmdConfig{
		Hotreload:  hotreload,
		Production: false,
		Entrypoint: config.Entrypoint,
		Envs:       getServerEnvironment(port, host, hotreload),
	})
	if err != nil {
		return nil, fmt.Errorf("error finding root cmd: %v", err)
	}
	return exec.Command("sh", "-c", strings.Join(rootCmd, " ")), nil
}

func findJobCommand(task map[string]interface{}) (*exec.Cmd, error) {
	rootCmd, err := findRootCmd(0, "localhost", false)
	if err != nil {
		return nil, fmt.Errorf("error finding root cmd: %v", err)
	}
	for arg := range task {
		jsonencoded, err := json.Marshal(task[arg])
		if err != nil {
			return nil, fmt.Errorf("error marshalling task: %v", err)
		}
		lastArg := rootCmd.Args[len(rootCmd.Args)-1]
		lastArg = strings.Join([]string{lastArg, "--" + arg, string(jsonencoded)}, " ")
		rootCmd.Args[len(rootCmd.Args)-1] = lastArg
	}
	return rootCmd, nil
}

type RootCmdConfig struct {
	Hotreload  bool
	Production bool
	Docker     bool
	Entrypoint Entrypoints
	Envs       CommandEnv
}

func findRootCmdAsString(cfg RootCmdConfig) ([]string, error) {
	language := moduleLanguage()
	switch language {
	case "python":
		return findPythonRootCmdAsString(cfg)
	case "typescript":
		return findTSRootCmdAsString(cfg)
	}
	return nil, fmt.Errorf("language not supported")
}

type CommandEnv map[string]string

func (c *CommandEnv) Set(key, value string) {
	(*c)[key] = value
}

func (c *CommandEnv) AddClientEnv() {
	for _, envVar := range os.Environ() {
		parts := strings.Split(envVar, "=")
		if len(parts) < 2 {
			continue
		}
		key := parts[0]
		value := strings.Join(parts[1:], "=")
		c.Set(key, value)
	}
}

func (c *CommandEnv) ToEnv() []string {
	env := []string{}
	for k, v := range *c {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

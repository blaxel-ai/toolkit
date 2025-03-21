package cli

import (
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

func findRootCmd(hotreload bool) (*exec.Cmd, error) {
	rootCmd, err := findRootCmdAsString(RootCmdConfig{
		Hotreload:  hotreload,
		Production: false,
	})
	if err != nil {
		return nil, fmt.Errorf("error finding root cmd: %v", err)
	}
	return exec.Command("sh", "-c", strings.Join(rootCmd, " ")), nil
}

type RootCmdConfig struct {
	Hotreload  bool
	Production bool
}

func findRootCmdAsString(config RootCmdConfig) ([]string, error) {
	language := moduleLanguage()
	switch language {
	case "python":
		return findPythonRootCmdAsString(config)
	case "typescript":
		return findTSRootCmdAsString(config)
	}
	return nil, fmt.Errorf("language not supported")
}

func getDockerfile() (string, error) {
	language := moduleLanguage()
	switch language {
	case "python":
		return getPythonDockerfile()
	case "typescript":
		return getTSDockerfile()
	}
	return "", fmt.Errorf("language not supported")
}

type CommandEnv map[string]string

func (c *CommandEnv) Set(key, value string) {
	(*c)[key] = value
}

func (c *CommandEnv) AddClientEnv() {
	for _, envVar := range os.Environ() {
		key := strings.Split(envVar, "=")[0]
		value := strings.Split(envVar, "=")[1]
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

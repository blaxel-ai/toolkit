package server

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
)

func FindRootCmd(port int, host string, hotreload bool, folder string, config core.Config) (*exec.Cmd, error) {
	rootCmd, err := FindRootCmdAsString(RootCmdConfig{
		Folder:     folder,
		Hotreload:  hotreload,
		Production: false,
		Entrypoint: config.Entrypoint,
		Envs:       GetServerEnvironment(port, host, hotreload, config),
	})
	if err != nil {
		return nil, fmt.Errorf("error finding root cmd: %v", err)
	}
	return exec.Command("sh", "-c", strings.Join(rootCmd, " ")), nil
}

type RootCmdConfig struct {
	Folder     string
	Hotreload  bool
	Production bool
	Docker     bool
	Entrypoint core.Entrypoints
	Envs       core.CommandEnv
}

func FindRootCmdAsString(cfg RootCmdConfig) ([]string, error) {
	language := core.ModuleLanguage(cfg.Folder)
	switch language {
	case "python":
		return findPythonRootCmdAsString(cfg)
	case "typescript":
		return findTSRootCmdAsString(cfg)
	case "go":
		return findGoRootCmdAsString(cfg)
	}
	return nil, fmt.Errorf("language not supported")
}

func FindJobCommand(task map[string]interface{}, folder string, config core.Config) (*exec.Cmd, error) {
	rootCmd, err := FindRootCmd(0, "localhost", false, folder, config)
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

func GetServerEnvironment(port int, host string, hotreload bool, config core.Config) core.CommandEnv {
	env := core.CommandEnv{}
	// Add all current env variables if not already set
	env.AddClientEnv()
	env.Set("BL_SERVER_PORT", fmt.Sprintf("%d", port))
	env.Set("BL_SERVER_HOST", host)
	env.Set("BL_WORKSPACE", config.Workspace)
	env.Set("PATH", os.Getenv("PATH"))
	if hotreload {
		env.Set("BL_HOTRELOAD", "true")
	}
	secrets := core.GetSecrets()
	for _, secret := range secrets {
		env.Set(secret.Name, secret.Value)
	}
	return env
}

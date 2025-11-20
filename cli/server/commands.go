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
	// First, check if entrypoint is configured
	var useEntrypoint bool
	var entrypoint string

	if cfg.Hotreload {
		// For hotreload, use dev entrypoint if available
		if cfg.Entrypoint.Development != "" {
			useEntrypoint = true
			entrypoint = cfg.Entrypoint.Development
		}
	} else {
		// For production, use prod entrypoint if available
		if cfg.Entrypoint.Production != "" {
			useEntrypoint = true
			entrypoint = cfg.Entrypoint.Production
		}
	}

	if useEntrypoint {
		return strings.Fields(entrypoint), nil
	}

	// Fall back to language detection
	language := core.ModuleLanguage(cfg.Folder)
	switch language {
	case "python":
		return findPythonRootCmdAsString(cfg)
	case "typescript":
		return findTSRootCmdAsString(cfg)
	case "go":
		return findGoRootCmdAsString(cfg)
	default:
		if cfg.Hotreload {
			return nil, fmt.Errorf("no dev entrypoint configured and language not supported")
		}
		return nil, fmt.Errorf("no prod entrypoint configured and language not supported")
	}
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

func StartEntrypoint(port int, host string, hotreload bool, folder string, config core.Config) *exec.Cmd {
	var entrypoint string

	// Choose the appropriate entrypoint based on hotreload flag
	if hotreload {
		if config.Entrypoint.Development != "" {
			entrypoint = config.Entrypoint.Development
		} else {
			core.PrintError("Serve", fmt.Errorf("no dev entrypoint configured in blaxel.toml for hotreload mode"))
			os.Exit(1)
		}
	} else {
		if config.Entrypoint.Production != "" {
			entrypoint = config.Entrypoint.Production
		} else {
			core.PrintError("Serve", fmt.Errorf("no prod entrypoint configured in blaxel.toml"))
			os.Exit(1)
		}
	}

	fmt.Printf("Starting server with entrypoint: %s\n", entrypoint)

	// Parse the entrypoint command
	cmdParts := strings.Fields(entrypoint)
	if len(cmdParts) == 0 {
		core.PrintError("Serve", fmt.Errorf("entrypoint is empty"))
		os.Exit(1)
	}

	var cmd *exec.Cmd
	if len(cmdParts) > 1 {
		cmd = exec.Command(cmdParts[0], cmdParts[1:]...)
	} else {
		cmd = exec.Command(cmdParts[0])
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Dir = folder

	// Set env variables
	envs := GetServerEnvironment(port, host, hotreload, config)
	cmd.Env = envs.ToEnv()

	err := cmd.Start()
	if err != nil {
		core.PrintError("Serve", fmt.Errorf("failed to start entrypoint: %w", err))
		os.Exit(1)
	}

	return cmd
}

func GetServerEnvironment(port int, host string, hotreload bool, config core.Config) core.CommandEnv {
	env := core.CommandEnv{}
	// Add all current env variables if not already set
	env.AddClientEnv()
	env.Set("BL_SERVER_PORT", fmt.Sprintf("%d", port))
	env.Set("BL_SERVER_HOST", host)
	env.Set("HOST", host)
	env.Set("PORT", fmt.Sprintf("%d", port))
	workspace := config.Workspace
	if workspace == "" {
		workspace = core.GetWorkspace()
	}
	env.Set("BL_WORKSPACE", workspace)
	env.Set("PATH", os.Getenv("PATH"))
	if hotreload {
		env.Set("BL_HOTRELOAD", "true")
	}
	secrets := core.GetSecrets()
	for _, secret := range secrets {
		env.Set(secret.Name, secret.Value)
	}
	if config.Env != nil {
		for key, value := range config.Env {
			env.Set(key, value)
		}
	}
	return env
}

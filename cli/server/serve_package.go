package server

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/fatih/color"
)

type PackageCommand struct {
	Name    string
	Cwd     string
	Command string
	Args    []string
	Color   string
	Envs    core.CommandEnv
}

func StartPackageServer(port int, host string, hotreload bool, config core.Config, envFiles []string, secrets []core.Env) bool {
	commands, err := getServeCommands(port, host, hotreload, config, envFiles, secrets)
	if err != nil {
		core.PrintError("Serve", fmt.Errorf("failed to get package commands: %w", err))
		os.Exit(1)
	}
	if len(commands) == 1 {
		if commands[0].Name == "root" {
			return false
		}
		RunCommands(commands, true)
		return true
	}

	RunCommands(commands, false)
	return true
}

func RunCommands(commands []PackageCommand, oneByOne bool) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for _, cmdInfo := range commands {
		cmd := exec.Command(cmdInfo.Command, cmdInfo.Args...)
		cmd.Dir = cmdInfo.Cwd
		cmd.Env = append(os.Environ(), cmdInfo.Envs.ToEnv()...)
		stdoutPipe, _ := cmd.StdoutPipe()
		stderrPipe, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			core.PrintError("Serve", fmt.Errorf("failed to start command '%s': %w", cmdInfo.Name, err))
			continue
		}

		go prefixOutput(stdoutPipe, cmdInfo.Name, cmdInfo.Color)
		go prefixOutput(stderrPipe, cmdInfo.Name, cmdInfo.Color)

		if oneByOne {
			err := cmd.Wait() // Wait for the command to finish before starting the next one
			if err != nil {
				core.PrintError("Serve", fmt.Errorf("error waiting for command '%s': %w", cmdInfo.Name, err))
			}
		} else {
			go func() {
				err := cmd.Wait()
				if err != nil {
					core.PrintError("Serve", fmt.Errorf("error waiting for command '%s': %w", cmdInfo.Name, err))
				}
			}()
		}
	}

	if !oneByOne {
		<-sigChan
	}
}

func prefixOutput(pipe io.ReadCloser, prefix string, color string) {

	// Ensure the prefix is exactly 20 characters long
	if len(prefix) < 20 {
		prefix = fmt.Sprintf("%-20s", prefix) // Left-align and pad with spaces
	} else if len(prefix) > 20 {
		prefix = prefix[:20] // Truncate if longer than 20 characters
	}

	// we colorize the prefix
	prefix = colorize(prefix, color)

	scanner := bufio.NewScanner(pipe)
	for scanner.Scan() {
		fmt.Printf("%s %s\n", prefix, scanner.Text())
	}
}

func colorize(text string, clr string) string {
	switch clr {
	case "red":
		return color.New(color.FgRed).Sprint(text)
	case "green":
		return color.New(color.FgGreen).Sprint(text)
	case "blue":
		return color.New(color.FgBlue).Sprint(text)
	case "yellow":
		return color.New(color.FgYellow).Sprint(text)
	case "purple":
		return color.New(color.FgMagenta).Sprint(text)
	case "cyan":
		return color.New(color.FgCyan).Sprint(text)
	case "white":
		return color.New(color.FgWhite).Sprint(text)
	default:
		return text // Return uncolored text if color is not recognized
	}
}

func GetAllPackages(config core.Config) map[string]core.Package {
	packages := make(map[string]core.Package)
	for functionName := range config.Function {
		pkg := config.Function[functionName]
		pkg.Type = "function"
		packages[functionName] = pkg
	}
	for agentName := range config.Agent {
		pkg := config.Agent[agentName]
		pkg.Type = "agent"
		packages[agentName] = pkg
	}
	for jobName := range config.Job {
		pkg := config.Job[jobName]
		pkg.Type = "job"
		packages[jobName] = pkg
	}
	return packages
}

func getServeCommands(port int, host string, hotreload bool, config core.Config, envFiles []string, secrets []core.Env) ([]PackageCommand, error) {
	packages := GetAllPackages(config)
	usedPorts := make(map[int]bool)
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting current directory: %v", err)
	}
	colors := []string{"red", "green", "blue", "yellow", "purple", "cyan", "white"}
	command := PackageCommand{
		Name:    "root",
		Cwd:     pwd,
		Command: "bl",
		Args:    []string{"serve", "--port", fmt.Sprintf("%d", port), "--host", host, "--recursive=false", "--skip-version-warning"},
		Color:   colors[0],
	}
	if hotreload {
		command.Args = append(command.Args, "--hotreload")
	}
	commands := []PackageCommand{}
	if !config.SkipRoot {
		commands = append(commands, command)
	}
	i := len(commands)
	for name, pkg := range packages {
		if pkg.Type == "job" {
			fmt.Printf("Skipping job %s\n", name)
			continue
		}
		if pkg.Port == 0 {
			return nil, fmt.Errorf("port is not set for %s", name)
		} else {
			if !usedPorts[pkg.Port] {
				usedPorts[pkg.Port] = true
			} else {
				fmt.Printf("Port %d is already in use, please choose another one\n", pkg.Port)
				os.Exit(1)
			}
		}
		command := PackageCommand{
			Name:    name,
			Cwd:     filepath.Join(pwd, pkg.Path),
			Command: "bl",
			Args: []string{
				"serve",
				"--port",
				fmt.Sprintf("%d", pkg.Port),
				"--host",
				host,
				"--recursive=false",
				"--skip-version-warning",
			},
			Color: colors[i%len(colors)],
		}
		if hotreload {
			command.Args = append(command.Args, "--hotreload")
		}
		for _, envFile := range envFiles {
			command.Args = append(command.Args, "--env-file", envFile)
		}
		for _, secret := range secrets {
			command.Args = append(command.Args, "-s", fmt.Sprintf("%s=%s", secret.Name, secret.Value))
		}
		commands = append(commands, command)
		i++
	}

	envs := core.CommandEnv{}
	for name, pkg := range packages {
		if pkg.Type != "" {
			nameUpper := strings.ToUpper(strings.ReplaceAll(name, "-", "_"))
			typeUpper := strings.ToUpper(pkg.Type)
			envs["BL_"+typeUpper+"_"+nameUpper+"_URL"] = "http://localhost:" + fmt.Sprintf("%d", pkg.Port)
		}
	}

	for i := range commands {
		commands[i].Envs = envs
	}
	return commands, nil
}

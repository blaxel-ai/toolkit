package cli

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

	"github.com/fatih/color"
)

type PackageCommand struct {
	Name    string
	Cwd     string
	Command string
	Args    []string
	Color   string
	Envs    CommandEnv
}

func startPackageServer(port int, host string, hotreload bool) bool {
	commands, err := getServeCommands(port, host, hotreload)
	if err != nil {
		fmt.Println("Error getting package commands:", err)
		os.Exit(1)
	}
	if len(commands) == 1 {
		return false
	}

	runCommands(commands, false)
	return true
}

func runCommands(commands []PackageCommand, oneByOne bool) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for _, cmdInfo := range commands {
		cmd := exec.Command(cmdInfo.Command, cmdInfo.Args...)
		cmd.Dir = cmdInfo.Cwd
		cmd.Env = append(os.Environ(), cmdInfo.Envs.ToEnv()...)
		stdoutPipe, _ := cmd.StdoutPipe()
		stderrPipe, _ := cmd.StderrPipe()

		if err := cmd.Start(); err != nil {
			fmt.Printf("Error starting command %s: %v\n", cmdInfo.Name, err)
			continue
		}

		go prefixOutput(stdoutPipe, cmdInfo.Name, cmdInfo.Color)
		go prefixOutput(stderrPipe, cmdInfo.Name, cmdInfo.Color)

		if oneByOne {
			err := cmd.Wait() // Wait for the command to finish before starting the next one
			if err != nil {
				fmt.Printf("Error waiting for command %s: %v\n", cmdInfo.Name, err)
			}
		} else {
			go func() {
				err := cmd.Wait()
				if err != nil {
					fmt.Printf("Error waiting for command %s: %v\n", cmdInfo.Name, err)
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

func getAllPackages() map[string]Package {
	packages := make(map[string]Package)
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
	return packages
}

func getServeCommands(port int, host string, hotreload bool) ([]PackageCommand, error) {
	packages := getAllPackages()
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
	i := 1
	for name, pkg := range packages {
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
		commands = append(commands, command)
		i++
	}

	envs := CommandEnv{}
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

package cli

import (
	"bufio"
	"fmt"
	"io"
	"net"
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

func startPackageServer(port int, host string, hotreload bool) {
	commands, err := getPackageCommands(port, host, hotreload)
	if err != nil {
		fmt.Println("Error getting package commands:", err)
		os.Exit(1)
	}

	runCommands(commands, false)
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

		go prefixOutput(stdoutPipe, fmt.Sprintf("%s", cmdInfo.Name), cmdInfo.Color)
		go prefixOutput(stderrPipe, fmt.Sprintf("%s", cmdInfo.Name), cmdInfo.Color)

		if oneByOne {
			cmd.Wait() // Wait for the command to finish before starting the next one
		} else {
			go func() {
				cmd.Wait()
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

func getFreePort() int {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		fmt.Println("Error getting free port:", err)
		os.Exit(1)
	}
	freePort := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return freePort
}

func getPackageCommands(port int, host string, hotreload bool) ([]PackageCommand, error) {
	usedPorts := make(map[int]bool)
	pwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting current directory: %v", err)
	}
	commands := []PackageCommand{}
	first := true
	i := 0
	colors := []string{"red", "green", "blue", "yellow", "purple", "cyan", "white"}
	for name, pkg := range config.Packages {
		if pkg.Port == 0 {
			if first && port != 0 {
				pkg.Port = port
			} else {
				// We get a free random port
				for {
					freePort := getFreePort()
					if !usedPorts[freePort] {
						pkg.Port = freePort
						usedPorts[freePort] = true
						break
					}
				}
			}
		} else {
			if !usedPorts[pkg.Port] {
				usedPorts[pkg.Port] = true
			} else {
				fmt.Printf("Port %d is already in use, please choose another one\n", pkg.Port)
				os.Exit(1)
			}
		}
		first = false
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
			},
		}
		if hotreload {
			command.Args = append(command.Args, "--hotreload")
		}
		command.Color = colors[i%len(colors)]
		commands = append(commands, command)
		i++
	}

	envs := CommandEnv{}
	for name, pkg := range config.Packages {
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

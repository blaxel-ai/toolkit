package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"

	"github.com/spf13/cobra"
)

func (r *Operations) ServeCmd() *cobra.Command {
	var port int
	var host string
	var hotreload bool
	var module string
	var remote bool

	cmd := &cobra.Command{
		Use:     "serve",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"s", "se"},
		Short:   "Serve a blaxel project",
		Long:    "Serve a blaxel project",
		Example: `  bl serve --remote --hotreload --port 1338`,
		Run: func(cmd *cobra.Command, args []string) {
			var activeProc *exec.Cmd

			// Check for pyproject.toml or package.json
			language := moduleLanguage()
			switch language {
			case "python":
				activeProc = startUvicornServer(port, host, hotreload, module, remote)
			case "typescript":
				activeProc = startTypescriptServer(port, host, hotreload, module, remote)
			default:
				fmt.Println("Error: Neither pyproject.toml nor package.json found in current directory")
				os.Exit(1)
			}

			// Handle graceful shutdown on interrupt
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			go func() {
				<-c
				fmt.Println("\nShutting down server...")
				// Send SIGTERM instead of Kill to allow graceful shutdown
				if err := activeProc.Process.Signal(os.Interrupt); err != nil {
					fmt.Printf("Error sending interrupt signal: %v\n", err)
					// Fall back to Kill if Interrupt fails
					if err := activeProc.Process.Kill(); err != nil {
						fmt.Printf("Error killing process: %v\n", err)
					}
				}
				// Don't exit immediately - let the main Wait() handle the exit
			}()

			// Wait for process to exit
			if err := activeProc.Wait(); err != nil {
				// Only treat as error if we didn't interrupt it ourselves
				if err.Error() != "signal: interrupt" {
					os.Exit(1)
				}
			}
			os.Exit(0)
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 1338, "Bind socket to this host")
	cmd.Flags().StringVarP(&host, "host", "H", "0.0.0.0", "Bind socket to this port. If 0, an available port will be picked")
	cmd.Flags().StringVarP(&module, "module", "m", "", "Module to serve, can be an agent or a function")
	cmd.Flags().BoolVarP(&hotreload, "hotreload", "", false, "Watch for changes in the project")
	cmd.Flags().BoolVarP(&remote, "remote", "r", false, "Serve the project remotely. It will use functions deployed on blaxel cloud")
	return cmd
}

func startUvicornServer(port int, host string, hotreload bool, module string, remote bool) *exec.Cmd {
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

	// Set env variables
	if module == "" {
		module = "agent.main"
	}
	uvicorn.Env = getServerEnvironment(port, host, module, remote)

	err := uvicorn.Start()
	if err != nil {
		fmt.Printf("Error starting uvicorn server: %v\n", err)
		os.Exit(1)
	}

	return uvicorn
}

func startTypescriptServer(port int, host string, hotreload bool, module string, remote bool) *exec.Cmd {
	ts := exec.Command("npm", "run", "start")
	if hotreload {
		ts = exec.Command("npm", "run", "dev")
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
	ts.Env = getServerEnvironment(port, host, module, remote)
	// Check if src directory exists and is a directory
	srcInfo, err := os.Stat("src")
	if module == "" {
		if err == nil && srcInfo.IsDir() {
			ts.Env = append(ts.Env, fmt.Sprintf("BL_SERVER_MODULE=%s", "src.agent.agent"))
		} else {
			ts.Env = append(ts.Env, fmt.Sprintf("BL_SERVER_MODULE=%s", "agent.agent"))
		}
	}
	if os.Getenv("BL_AGENT_FUNCTIONS_DIRECTORY") == "" {
		if err == nil && srcInfo.IsDir() {
			ts.Env = append(ts.Env, fmt.Sprintf("BL_AGENT_FUNCTIONS_DIRECTORY=%s", "src/functions"))
		} else {
			ts.Env = append(ts.Env, fmt.Sprintf("BL_AGENT_FUNCTIONS_DIRECTORY=%s", "functions"))
		}
	}

	// Add src directory to NODE_PATH
	nodePath := "src"
	if currentPath := os.Getenv("NODE_PATH"); currentPath != "" {
		nodePath = fmt.Sprintf("%s:%s", nodePath, currentPath)
	}
	ts.Env = append(ts.Env, fmt.Sprintf("NODE_PATH=%s", nodePath))

	err = ts.Start()
	if err != nil {
		fmt.Printf("Error starting tsx server: %v\n", err)
		os.Exit(1)
	}

	return ts
}

func getServerEnvironment(port int, host string, module string, remote bool) []string {
	env := []string{}
	env = append(env, fmt.Sprintf("BL_WORKSPACE=%s", workspace))
	env = append(env, fmt.Sprintf("BL_REMOTE=%t", remote))
	env = append(env, fmt.Sprintf("BL_SERVER_PORT=%d", port))
	env = append(env, fmt.Sprintf("BL_SERVER_HOST=%s", host))
	env = append(env, fmt.Sprintf("BL_SERVER_MODULE=%s", module))

	if os.Getenv("BL_ENV") != "" {
		env = append(env, fmt.Sprintf("BL_ENV=%s", os.Getenv("BL_ENV")))
	}

	// Add all current env variables if not already set
	env = AddClientEnv(env)
	return env
}

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
				activeProc = startPythonServer(port, host, hotreload)
			case "typescript":
				activeProc = startTypescriptServer(port, host, hotreload)
			default:
				fmt.Println("Error: Neither pyproject.toml nor package.json found in current directory")
				os.Exit(1)
			}

			// Handle graceful shutdown on interrupt
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			go func() {
				<-c
				if err := activeProc.Process.Signal(os.Interrupt); err != nil {
					fmt.Printf("Error sending interrupt signal: %v\n", err)
					// Fall back to Kill if Interrupt fails
					if err := activeProc.Process.Kill(); err != nil {
						fmt.Printf("Error killing process: %v\n", err)
					}
				}
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
	cmd.Flags().BoolVarP(&hotreload, "hotreload", "", false, "Watch for changes in the project")
	return cmd
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

	uvicorn.Env = getServerEnvironment(port, host)

	err := uvicorn.Start()
	if err != nil {
		fmt.Printf("Error starting uvicorn server: %v\n", err)
		os.Exit(1)
	}

	return uvicorn
}

func getServerEnvironment(port int, host string) []string {
	env := []string{}
	env = append(env, fmt.Sprintf("BL_SERVER_PORT=%d", port))
	env = append(env, fmt.Sprintf("BL_SERVER_HOST=%s", host))

	// Add all current env variables if not already set
	env = AddClientEnv(env)
	return env
}

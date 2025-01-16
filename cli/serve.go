package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"

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
		Short:   "Serve a beamlit project",
		Long:    "Serve a beamlit project",
		Example: `  bl serve --remote --hotreload --port 1338`,
		Run: func(cmd *cobra.Command, args []string) {
			uvicornCmd := "uvicorn"
			if _, err := os.Stat(".venv"); !os.IsNotExist(err) {
				uvicornCmd = ".venv/bin/uvicorn"
			}
			uvicorn := exec.Command(
				uvicornCmd,
				"beamlit.serve.app:app",
				"--port",
				fmt.Sprintf("%d", port),
				"--host",
				host,
			)
			if hotreload {
				uvicorn.Args = append(uvicorn.Args, "--reload")
			}

			uvicorn.Stdout = os.Stdout
			uvicorn.Stderr = os.Stderr
			if environment == "" {
				environment = "production"
			}
			uvicorn.Env = append(uvicorn.Env, fmt.Sprintf("BL_ENVIRONMENT=%s", environment))
			uvicorn.Env = append(uvicorn.Env, fmt.Sprintf("BL_WORKSPACE=%s", workspace))
			uvicorn.Env = append(uvicorn.Env, fmt.Sprintf("BL_REMOTE=%t", remote))
			uvicorn.Env = append(uvicorn.Env, fmt.Sprintf("BL_SERVER_PORT=%d", port))
			uvicorn.Env = append(uvicorn.Env, fmt.Sprintf("BL_SERVER_HOST=%s", host))
			uvicorn.Env = append(uvicorn.Env, fmt.Sprintf("BL_SERVER_MODULE=%s", module))
			if os.Getenv("BL_ENV") != "" {
				uvicorn.Env = append(uvicorn.Env, fmt.Sprintf("BL_ENV=%s", os.Getenv("BL_ENV")))
			}
			
			// Add all current environment variables if not already set
			for _, envVar := range os.Environ() {
				found := false
				for _, existingVar := range uvicorn.Env {
					if envVar == existingVar {
						found = true
						break
					}
				}
				if !found {
					uvicorn.Env = append(uvicorn.Env, envVar)
				}
			}

			err := uvicorn.Start()
			if err != nil {
				fmt.Printf("Error starting server: %v\n", err)
				os.Exit(1)
			}

			// Handle graceful shutdown on interrupt
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			go func() {
				<-c
				fmt.Println("\nShutting down server...")
				if err := uvicorn.Process.Kill(); err != nil {
					fmt.Printf("Error killing server process: %v\n", err)
				}
				os.Exit(0)
			}()

			if err := uvicorn.Wait(); err != nil {
				fmt.Printf("Server error: %v\n", err)
				os.Exit(1)
			}
		},
	}

	cmd.Flags().IntVarP(&port, "port", "p", 1338, "Bind socket to this host")
	cmd.Flags().StringVarP(&host, "host", "H", "0.0.0.0", "Bind socket to this port. If 0, an available port will be picked")
	cmd.Flags().StringVarP(&module, "module", "m", "agent.main", "Module to serve, can be an agent or a function")
	cmd.Flags().BoolVarP(&hotreload, "hotreload", "", false, "Watch for changes in the project")
	cmd.Flags().BoolVarP(&remote, "remote", "r", false, "Serve the project remotely. It will use functions deployed on beamlit cloud")
	return cmd
}

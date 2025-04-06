package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

	"github.com/spf13/cobra"
)

func (r *Operations) ServeCmd() *cobra.Command {
	var port int
	var host string
	var hotreload bool
	var recursive bool
	cmd := &cobra.Command{
		Use:     "serve",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"s", "se"},
		Short:   "Serve a blaxel project",
		Long:    "Serve a blaxel project",
		Example: `bl serve --remote --hotreload --port 1338`,
		Run: func(cmd *cobra.Command, args []string) {
			var activeProc *exec.Cmd

			cwd, err := os.Getwd()
			if err != nil {
				fmt.Printf("Error getting current working directory: %v\n", err)
				os.Exit(1)
			}
			err = r.SeedCache(cwd)
			if err != nil {
				fmt.Println("Error seeding cache:", err)
				os.Exit(1)
			}

			// If it's a package, we need to handle it
			if recursive {
				startPackageServer(port, host, hotreload)
				return
			}
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
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", true, "Serve the project recursively")
	return cmd
}

func getServerEnvironment(port int, host string, hotreload bool) CommandEnv {
	env := CommandEnv{}
	// Add all current env variables if not already set
	env.AddClientEnv()
	env.Set("BL_SERVER_PORT", fmt.Sprintf("%d", port))
	env.Set("BL_SERVER_HOST", host)
	env.Set("BL_WORKSPACE", config.Workspace)
	env.Set("PATH", getServerPath())
	if hotreload {
		env.Set("BL_HOTRELOAD", "true")
	}
	return env
}

func getServerPath() string {
	pwd, err := os.Getwd()
	if err != nil {
		fmt.Println("Error getting current directory:", err)
		os.Exit(1)
	}
	language := moduleLanguage()
	switch language {
	case "typescript":
		path := filepath.Join(pwd, "node_modules", ".bin")
		return fmt.Sprintf("%s:%s", path, os.Getenv("PATH"))
	}
	return os.Getenv("PATH")
}

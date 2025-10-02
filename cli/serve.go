package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/server"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("serve", func() *cobra.Command {
		return ServeCmd()
	})
}

func ServeCmd() *cobra.Command {
	var port int
	var host string
	var hotreload bool
	var recursive bool
	var folder string
	var envFiles []string
	var commandSecrets []string
	cmd := &cobra.Command{
		Use:     "serve",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"s", "se"},
		Short:   "Serve a blaxel project",
		Long: `Start a local development server for your Blaxel project.

This runs your agent, MCP server, or job locally on your machine for rapid
development and testing. Perfect for the inner development loop where you
want to iterate quickly without deploying to the cloud.

Supported Languages:
- Python (requires pyproject.toml or requirements.txt)
- TypeScript/JavaScript (requires package.json)
- Go (requires go.mod)

Hot Reload:
Enable --hotreload to automatically restart your server when code changes
are detected. This dramatically speeds up development by eliminating manual
restarts.

Testing Locally:
While your server is running, test it with:
- bl chat agent-name --local   (for agents)
- bl run agent agent-name --local --data '{}'   (for agents/jobs)

Workflow:
1. bl serve --hotreload        Start local server with auto-reload
2. Edit your code               Make changes
3. Test immediately             Server reloads automatically
4. bl deploy                    Deploy when ready`,
		Example: `  # Basic serve with hot reload (recommended)
  bl serve --hotreload

  # Serve on custom port
  bl serve --port 8080

  # Serve specific subdirectory in monorepo
  bl serve -d packages/my-agent

  # Serve with environment variables
  bl serve -e .env.local

  # Serve with secrets (for testing)
  bl serve -s API_KEY=test-key -s DB_PASSWORD=secret

  # Full development workflow
  bl serve --hotreload          # Terminal 1: Run server
  bl chat my-agent --local      # Terminal 2: Test agent`,
		Run: func(cmd *cobra.Command, args []string) {
			var activeProc *exec.Cmd
			core.LoadCommandSecrets(commandSecrets)
			core.ReadSecrets(folder, envFiles)
			if folder != "" {
				core.ReadSecrets("", envFiles)
				core.ReadConfigToml(folder)
			}
			config := core.GetConfig()

			cwd, err := os.Getwd()
			if err != nil {
				core.PrintError("Serve", fmt.Errorf("error getting current working directory: %w", err))
				os.Exit(1)
			}

			err = core.SeedCache(cwd)
			if err != nil {
				core.PrintError("Serve", fmt.Errorf("error seeding cache: %w", err))
				os.Exit(1)
			}

			// If it's a package, we need to handle it
			if recursive {
				if server.StartPackageServer(port, host, hotreload, config, envFiles, core.GetSecrets()) {
					return
				}
			}
			// Check for pyproject.toml or package.json
			language := core.ModuleLanguage(folder)
			switch language {
			case "python":
				activeProc = server.StartPythonServer(port, host, hotreload, folder, config)
			case "typescript":
				activeProc = server.StartTypescriptServer(port, host, hotreload, folder, config)
			case "go":
				activeProc = server.StartGoServer(port, host, hotreload, folder, config)
			default:
				core.PrintError("Serve", fmt.Errorf("neither pyproject.toml nor package.json found in current directory"))
				os.Exit(1)
			}

			// Handle graceful shutdown on interrupt
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt)
			go func() {
				<-c
				if err := activeProc.Process.Signal(os.Interrupt); err != nil {
					core.PrintError("Serve", fmt.Errorf("error sending interrupt signal: %w", err))
					// Fall back to Kill if Interrupt fails
					if err := activeProc.Process.Kill(); err != nil {
						core.PrintError("Serve", fmt.Errorf("error killing process: %w", err))
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
	cmd.Flags().StringVarP(&folder, "directory", "d", "", "Serve the project from a sub directory")
	cmd.Flags().StringSliceVarP(&envFiles, "env-file", "e", []string{".env"}, "Environment file to load")
	cmd.Flags().StringSliceVarP(&commandSecrets, "secrets", "s", []string{}, "Secrets to deploy")
	return cmd
}

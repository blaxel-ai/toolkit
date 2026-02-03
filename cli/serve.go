package cli

import (
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"

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

This runs your agent or MCP server locally on your machine for rapid
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
- bl run agent agent-name --local --data '{}'   (for agents)

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
				core.ReadConfigToml(folder, true)
			}
			config := core.GetConfig()

			// Volume templates cannot be served - they are data volumes, not services
			if core.IsVolumeTemplate(config.Type) {
				err := fmt.Errorf("volume templates cannot be served. Use 'bl deploy' to upload volume template files")
				core.PrintError("Serve", err)
				core.ExitWithError(err)
			}

			// Jobs cannot be served - they are batch processes, not persistent servers
			if config.Type == "job" {
				// Get job name from config or derive from directory
				jobName := config.Name
				if jobName == "" {
					cwd, _ := os.Getwd()
					jobName = filepath.Base(filepath.Join(cwd, folder))
				}
				jobName = core.Slugify(jobName)

				err := fmt.Errorf("jobs cannot be served as they are batch processes, not persistent servers")
				core.PrintError("Serve", err)
				fmt.Println()
				core.PrintInfo("To run a job locally:")
				fmt.Printf("  bl run job %s --local --file batches/sample-batch.json\n", jobName)
				fmt.Println("  Learn more: https://docs.blaxel.ai/Jobs/Develop-a-job")
				fmt.Println()
				core.PrintInfo("To run on Blaxel:")
				fmt.Println("  bl deploy")
				fmt.Printf("  bl run job %s --data '{\"tasks\": [{\"name\": \"John\"}]}'\n", jobName)
				fmt.Println("  Learn more: https://docs.blaxel.ai/Jobs/Deploy-a-job")
				fmt.Println()
				core.ExitWithError(err)
			}

			// Check if agent/function code uses HOST/PORT environment variables (warning only for serve)
			// Agent is the default type, so check if type is "", "agent", or "function"
			if config.Type == "" || config.Type == "agent" || config.Type == "function" {
				language := core.ModuleLanguage(folder)
				if !core.CheckServerEnvUsage(folder, language) {
					// Use "agent" as default type for documentation URL if type is empty
					resourceType := config.Type
					if resourceType == "" {
						resourceType = "agent"
					}
					serverEnvWarning := core.BuildServerEnvWarning(language, resourceType)
					fmt.Println(serverEnvWarning)
					fmt.Println()
				}
			}

			cwd, err := os.Getwd()
			if err != nil {
				err = fmt.Errorf("error getting current working directory: %w", err)
				core.PrintError("Serve", err)
				core.ExitWithError(err)
			}

			err = core.SeedCache(cwd)
			if err != nil {
				err = fmt.Errorf("error seeding cache: %w", err)
				core.PrintError("Serve", err)
				core.ExitWithError(err)
			}

			// If it's a package, we need to handle it
			if recursive {
				if server.StartPackageServer(port, host, hotreload, config, envFiles, core.GetSecrets()) {
					return
				}
			}

			// First, check if entrypoint is configured
			useEntrypoint := (config.Entrypoint.Production != "" && !hotreload) || (config.Entrypoint.Development != "" && hotreload)

			if useEntrypoint {
				activeProc = server.StartEntrypoint(port, host, hotreload, folder, config)
			} else {
				// Fall back to language detection
				language := core.ModuleLanguage(folder)
				switch language {
				case "python":
					activeProc = server.StartPythonServer(port, host, hotreload, folder, config)
				case "typescript":
					activeProc = server.StartTypescriptServer(port, host, hotreload, folder, config)
				case "go":
					activeProc = server.StartGoServer(port, host, hotreload, folder, config)
				default:
					// Neither entrypoint nor language detected
					// Check if blaxel.toml exists
					blaxelTomlPath := filepath.Join(cwd, folder, "blaxel.toml")
					blaxelTomlExists := false
					if _, err := os.Stat(blaxelTomlPath); err == nil {
						blaxelTomlExists = true
					}

					if hotreload {
						core.PrintError("Serve", fmt.Errorf("cannot start server with hotreload: no dev entrypoint configured and no language detected"))
						if blaxelTomlExists {
							core.PrintInfo("To fix this issue, configure a dev entrypoint in blaxel.toml:")
							core.Print("[entrypoint]")
							core.Print("dev = \"your-command\"")
						} else {
							core.PrintInfo("To fix this issue, create a blaxel.toml file with a dev entrypoint:")
							core.Print("[entrypoint]")
							core.Print("dev = \"your-command\"")
						}
						core.PrintInfo("\nOr execute this command:")
						if blaxelTomlExists {
							core.Print("echo '[entrypoint]\\ndev = \"your-command\"' >> blaxel.toml")
						} else {
							core.Print("echo '[entrypoint]\\ndev = \"your-command\"' > blaxel.toml")
						}
					} else {
						core.PrintError("Serve", fmt.Errorf("cannot start server: no prod entrypoint configured and no language detected"))
						if blaxelTomlExists {
							core.PrintInfo("To fix this issue, configure a prod entrypoint in blaxel.toml:")
							core.Print("[entrypoint]")
							core.Print("prod = \"your-command\"")
						} else {
							core.PrintInfo("To fix this issue, create a blaxel.toml file with a prod entrypoint:")
							core.Print("[entrypoint]")
							core.Print("prod = \"your-command\"")
						}
						core.PrintInfo("\nOr execute this command:")
						if blaxelTomlExists {
							core.Print("echo '[entrypoint]\\nprod = \"your-command\"' >> blaxel.toml")
						} else {
							core.Print("echo '[entrypoint]\\nprod = \"your-command\"' > blaxel.toml")
						}
					}
					core.ExitWithError(fmt.Errorf("cannot start server: no entrypoint configured and no language detected"))
				}
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

	cmd.Flags().IntVarP(&port, "port", "p", 1338, "Bind socket to this port")
	cmd.Flags().StringVarP(&host, "host", "H", "0.0.0.0", "Bind socket to this host. If 0.0.0.0, listens on all interfaces")
	cmd.Flags().BoolVarP(&hotreload, "hotreload", "", false, "Watch for changes in the project")
	cmd.Flags().BoolVarP(&recursive, "recursive", "r", true, "Serve the project recursively")
	cmd.Flags().StringVarP(&folder, "directory", "d", "", "Serve the project from a sub directory")
	cmd.Flags().StringSliceVarP(&envFiles, "env-file", "e", []string{".env"}, "Environment file to load")
	cmd.Flags().StringSliceVarP(&commandSecrets, "secrets", "s", []string{}, "Secrets to deploy")
	return cmd
}

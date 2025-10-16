package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/server"
	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("run", func() *cobra.Command {
		return RunCmd()
	})
}

func RunCmd() *cobra.Command {
	var data string
	var path string
	var method string
	var params []string
	var debug bool
	var local bool
	var headerFlags []string
	var uploadFilePath string
	var filePath string
	var envFiles []string
	var commandSecrets []string
	var folder string
	cmd := &cobra.Command{
		Use:   "run resource-type resource-name",
		Args:  cobra.ExactArgs(2),
		Short: "Run a resource on blaxel",
		Long: `Execute a Blaxel resource with custom input data.

Different resource types behave differently when run:

- agent: Send a single request (non-interactive, unlike 'bl chat')
         Returns agent response for the given input

- model: Make an inference request to an AI model
         Calls the model's API endpoint with your data

- job: Start a job execution with batch input
       Processes multiple tasks defined in JSON batch file

- function/mcp: Invoke an MCP server function
                Calls a specific tool or method

Local vs Remote:
- Remote (default): Runs against deployed resources in your workspace
- Local (--local): Runs against locally served resources (requires 'bl serve')

Input Formats:
- Inline JSON with --data '{"key": "value"}'
- From file with --file path/to/input.json

Advanced Usage:
Use --path, --method, and --params for custom HTTP requests to your resources.
This is useful for testing specific endpoints or non-standard API calls.`,
		Example: `  # Run agent with inline data
  bl run agent my-agent --data '{"inputs": "Summarize this text"}'

  # Run agent with file input
  bl run agent my-agent --file request.json

  # Run job with batch file
  bl run job my-job --file batches/process-users.json

  # Run job locally for testing (requires 'bl serve' in another terminal)
  bl run job my-job --local --file batch.json

  # Run model with custom endpoint
  bl run model my-model --path /v1/chat/completions --data '{"messages": [...]}'

  # Run with query parameters
  bl run agent my-agent --data '{}' --params "stream=true" --params "max_tokens=100"

  # Run with custom headers
  bl run agent my-agent --data '{}' --header "X-User-ID: 123"

  # Debug mode (see full request/response details)
  bl run agent my-agent --data '{}' --debug`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 || len(args) == 1 {
				core.PrintError("Run", fmt.Errorf("resource type and name are required"))
				os.Exit(1)
			}
			core.LoadCommandSecrets(commandSecrets)
			core.ReadSecrets("", envFiles)

			resourceType := args[0]
			resourceName := args[1]
			headers := make(map[string]string)

			// Parse header flags into map
			for _, header := range headerFlags {
				parts := strings.SplitN(header, ":", 2)
				if len(parts) != 2 {
					core.PrintError("Run", fmt.Errorf("invalid header format '%s'. Must be 'Key: Value'", header))
					os.Exit(1)
				}
				key := strings.TrimSpace(parts[0])
				value := strings.TrimSpace(parts[1])
				headers[key] = value
			}

			if filePath != "" {
				fileContent, err := os.ReadFile(filePath)
				if err != nil {
					core.PrintError("Run", fmt.Errorf("error reading file: %w", err))
				}
				data = string(fileContent)
			}

			// Handle file upload if specified
			if uploadFilePath != "" {
				fileContent, err := os.ReadFile(uploadFilePath)
				if err != nil {
					core.PrintError("Run", fmt.Errorf("error reading file: %w", err))
					os.Exit(1)
				}
				data = string(fileContent)
			}
			if (resourceType == "model" || resourceType == "models") && path == "" {
				path = getModelDefaultPath(resourceName)
			}

			if (resourceType == "job" || resourceType == "jobs") && local {
				runJobLocally(data, folder, core.GetConfig())
				os.Exit(0)
			}

			if (resourceType == "job" || resourceType == "jobs") && path == "" {
				path = "/executions"
			}
			if resourceType == "mcp" {
				resourceType = "functions"
			}
			if resourceType == "sbx" {
				resourceType = "sandbox"
			}

			client := core.GetClient()
			workspace := core.GetWorkspace()
			res, err := client.Run(
				context.Background(),
				workspace,
				resourceType,
				resourceName,
				method,
				path,
				headers,
				params,
				data,
				debug,
				local,
			)
			if err != nil {
				core.PrintError("Run", fmt.Errorf("error making request: %w", err))
				os.Exit(1)
			}
			defer func() { _ = res.Body.Close() }()

			// Read response body
			body, err := io.ReadAll(res.Body)
			if err != nil {
				core.PrintError("Run", fmt.Errorf("error reading response: %w", err))
				os.Exit(1)
			}
			// Only print status code if it's an error
			if res.StatusCode >= 400 {
				core.PrintError("Run", fmt.Errorf("response status: %s", res.Status))
			}

			if debug {
				core.Print("Response Headers:")
				for key, values := range res.Header {
					for _, value := range values {
						core.Print(fmt.Sprintf("  %s: %s", key, value))
					}
				}
				fmt.Println()
			}

			// Try to pretty print JSON response
			var prettyJSON bytes.Buffer
			if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
				core.Print(prettyJSON.String())
			} else {
				// If not JSON, print as string
				core.Print(string(body))
			}
		},
	}

	cmd.Flags().StringVarP(&filePath, "file", "f", "", "Input from a file")
	cmd.Flags().StringVarP(&data, "data", "d", "", "JSON body data for the inference request")
	cmd.Flags().StringVar(&path, "path", "", "path for the inference request")
	cmd.Flags().StringVar(&method, "method", "POST", "HTTP method for the inference request")
	cmd.Flags().StringSliceVar(&params, "params", []string{}, "Query params sent to the inference request")
	cmd.Flags().StringVar(&uploadFilePath, "upload-file", "", "This transfers the specified local file to the remote URL")
	cmd.Flags().StringArrayVar(&headerFlags, "header", []string{}, "Request headers in 'Key: Value' format. Can be specified multiple times")
	cmd.Flags().BoolVar(&debug, "debug", false, "Debug mode")
	cmd.Flags().BoolVar(&local, "local", false, "Run locally")
	cmd.Flags().StringSliceVarP(&envFiles, "env-file", "e", []string{".env"}, "Environment file to load")
	cmd.Flags().StringSliceVarP(&commandSecrets, "secrets", "s", []string{}, "Secrets to deploy")
	cmd.Flags().StringVar(&folder, "directory", "", "Directory to run the command from")
	return cmd
}

func getModelDefaultPath(resourceName string) string {
	client := core.GetClient()
	res, err := client.GetModel(context.Background(), resourceName)
	if err != nil {
		return ""
	}
	defer func() { _ = res.Body.Close() }()

	if res.StatusCode == 200 {
		var model sdk.Model
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return ""
		}
		err = json.Unmarshal(body, &model)
		if err != nil {
			return ""
		}

		integrationName := model.Spec.Runtime.Type
		if integrationName != nil {
			res, err := client.GetIntegration(context.Background(), *integrationName)
			if err == nil {
				defer func() { _ = res.Body.Close() }()
				if res.StatusCode == 200 {
					var integrationResponse sdk.Integration
					body, err := io.ReadAll(res.Body)
					if err != nil {
						return ""
					}

					err = json.Unmarshal(body, &integrationResponse)
					if err != nil {
						return ""
					}
					endpoints := integrationResponse.Endpoints
					if endpoints != nil {
						key := ""
						for path := range *endpoints {
							if key == "" || strings.Contains(path, "completions") {
								key = path
							}
						}
						if key != "" {
							core.PrintInfo(fmt.Sprintf("Using default path: %s, you can change it by specifying it with --path PATH", key))
						}
						return key
					}
				}
			}
		}
	}
	return ""
}

func runJobLocally(data string, folder string, config core.Config) {
	// Load .env if it exists and merge into command environment
	var dotenvVars map[string]string
	if _, err := os.Stat(".env"); err == nil {
		dotenvVars, err = godotenv.Read()
		if err != nil {
			core.PrintError("Run", fmt.Errorf("could not load .env file: %w", err))
		}
	}
	batch := sdk.Batch{}
	err := json.Unmarshal([]byte(data), &batch)
	if err != nil {
		core.PrintError("Run", fmt.Errorf("invalid JSON: %w", err))
		os.Exit(1)
	}

	for i, task := range batch.Tasks {
		core.PrintInfo(fmt.Sprintf("Task %d:", i+1))
		jsonencoded, err := json.Marshal(task)
		if err != nil {
			core.PrintError("Run", fmt.Errorf("error marshalling task: %w", err))
			os.Exit(1)
		}
		core.PrintInfo(fmt.Sprintf("Arguments: %s", string(jsonencoded)))
		cmd, err := server.FindJobCommand(task, folder, config)
		if err != nil {
			core.PrintError("Run", fmt.Errorf("error finding root cmd: %w", err))
			os.Exit(1)
		}

		// Merge .env variables into the command's environment
		if dotenvVars != nil {
			envMap := map[string]string{}
			for _, env := range os.Environ() {
				parts := strings.SplitN(env, "=", 2)
				if len(parts) >= 2 {
					envMap[parts[0]] = strings.Join(parts[1:], "=")
				}
			}
			for k, v := range dotenvVars {
				envMap[k] = v
			}
			cmd.Env = []string{}
			for k, v := range envMap {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
		}

		// Capture stdout and stderr
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		// Run the command and wait for it to complete
		err = cmd.Run()
		if err != nil {
			core.PrintError("Run", fmt.Errorf("error executing task %d: %w", i+1, err))
			os.Exit(1)
		}
		core.Print("")
	}
}

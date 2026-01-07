package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/server"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	blaxel "github.com/stainless-sdks/blaxel-go"
)

func init() {
	core.RegisterCommand("run", func() *cobra.Command {
		return RunCmd()
	})
}

// Batch represents a batch of tasks for job execution
type Batch struct {
	Tasks []map[string]interface{} `json:"tasks"`
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
- Inline JSON with --data json-object
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
				err := fmt.Errorf("resource type and name are required")
				core.PrintError("Run", err)
				core.ExitWithError(err)
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
					err := fmt.Errorf("invalid header format '%s'. Must be 'Key: Value'", header)
					core.PrintError("Run", err)
					core.ExitWithError(err)
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
					err = fmt.Errorf("error reading file: %w", err)
					core.PrintError("Run", err)
					core.ExitWithError(err)
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

			workspace := core.GetWorkspace()
			res, err := runRequest(
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
				err = fmt.Errorf("error making request: %w", err)
				core.PrintError("Run", err)
				core.ExitWithError(err)
			}
			defer func() { _ = res.Body.Close() }()

			// Read response body
			body, err := io.ReadAll(res.Body)
			if err != nil {
				err = fmt.Errorf("error reading response: %w", err)
				core.PrintError("Run", err)
				core.ExitWithError(err)
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

// runRequest executes a request to a blaxel resource
func runRequest(
	ctx context.Context,
	workspace string,
	resourceType string,
	resourceName string,
	method string,
	path string,
	headers map[string]string,
	params []string,
	data string,
	debug bool,
	local bool,
) (*http.Response, error) {
	var baseURL string
	if local {
		baseURL = "http://localhost:1338"
	} else {
		baseURL = blaxel.GetRunURL()
	}

	// Build the URL
	url := fmt.Sprintf("%s/%s/%s/%s", baseURL, workspace, resourceType, resourceName)
	if path != "" {
		if !strings.HasPrefix(path, "/") {
			path = "/" + path
		}
		url += path
	}

	// Add query params
	if len(params) > 0 {
		url += "?" + strings.Join(params, "&")
	}

	if debug {
		core.Print(fmt.Sprintf("Request URL: %s", url))
		core.Print(fmt.Sprintf("Request Method: %s", method))
		if data != "" {
			core.Print(fmt.Sprintf("Request Body: %s", data))
		}
	}

	// Create request
	var bodyReader io.Reader
	if data != "" {
		bodyReader = strings.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return nil, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Add authentication headers from credentials
	credentials, _ := blaxel.LoadCredentials(workspace)
	if credentials.APIKey != "" {
		req.Header.Set("X-Blaxel-Authorization", fmt.Sprintf("Bearer %s", credentials.APIKey))
	} else if credentials.AccessToken != "" {
		req.Header.Set("X-Blaxel-Authorization", fmt.Sprintf("Bearer %s", credentials.AccessToken))
	}

	// Add workspace header
	if workspace != "" {
		req.Header.Set("X-Blaxel-Workspace", workspace)
	}

	// Make request
	httpClient := &http.Client{}
	return httpClient.Do(req)
}

func getModelDefaultPath(resourceName string) string {
	client := core.GetClient()
	model, err := client.Models.Get(context.Background(), resourceName)
	if err != nil {
		return ""
	}

	integrationName := string(model.Spec.Runtime.Type)
	if integrationName != "" {
		integration, err := client.Integrations.Get(context.Background(), integrationName)
		if err == nil {
			endpoints := integration.Endpoints
			if endpoints != nil {
				key := ""
				for path := range endpoints {
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
	batch := Batch{}
	err := json.Unmarshal([]byte(data), &batch)
	if err != nil {
		err = fmt.Errorf("invalid JSON: %w", err)
		core.PrintError("Run", err)
		core.ExitWithError(err)
	}

	for i, task := range batch.Tasks {
		core.PrintInfo(fmt.Sprintf("Task %d:", i+1))
		jsonencoded, err := json.Marshal(task)
		if err != nil {
			err = fmt.Errorf("error marshalling task: %w", err)
			core.PrintError("Run", err)
			core.ExitWithError(err)
		}
		core.PrintInfo(fmt.Sprintf("Arguments: %s", string(jsonencoded)))
		cmd, err := server.FindJobCommand(task, folder, config)
		if err != nil {
			err = fmt.Errorf("error finding root cmd: %w", err)
			core.PrintError("Run", err)
			core.ExitWithError(err)
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
			err = fmt.Errorf("error executing task %d: %w", i+1, err)
			core.PrintError("Run", err)
			core.ExitWithError(err)
		}
		core.Print("")
	}
}

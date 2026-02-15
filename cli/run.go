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
	"sync"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/server"
	"github.com/joho/godotenv"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
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
	var concurrent int
	var outputFormat string
	cmd := &cobra.Command{
		Use:               "run resource-type resource-name",
		Args:              cobra.ExactArgs(2),
		Short:             "Run a resource on blaxel",
		ValidArgsFunction: GetRunValidArgsFunction(),
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

  # Run job locally with 4 concurrent workers
  bl run job my-job --local --file batch.json --concurrent 4

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
					core.ExitWithError(err)
				}

				// Check if file is YAML and convert to JSON
				if strings.HasSuffix(strings.ToLower(filePath), ".yaml") || strings.HasSuffix(strings.ToLower(filePath), ".yml") {
					var yamlData interface{}
					if err := yaml.Unmarshal(fileContent, &yamlData); err != nil {
						core.PrintError("Run", fmt.Errorf("error parsing YAML file: %w", err))
						core.ExitWithError(err)
					}
					jsonBytes, err := json.Marshal(yamlData)
					if err != nil {
						core.PrintError("Run", fmt.Errorf("error converting YAML to JSON: %w", err))
						core.ExitWithError(err)
					}
					data = string(jsonBytes)
				} else {
					data = string(fileContent)
				}
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

			isJob := resourceType == "job" || resourceType == "jobs"
			isRawOutput := outputFormat == "json" || outputFormat == "yaml"

			if isJob && local {
				runJobLocally(data, folder, core.GetConfig(), concurrent)
				os.Exit(0)
			}

			if isJob && path == "" {
				path = "/executions"
			}

			// Print descriptive info for job execution (skip if raw output format)
			if isJob && !isRawOutput {
				core.PrintInfo(fmt.Sprintf("Starting job execution for '%s'...", resourceName))
				// Parse and display batch info if available
				var batch Batch
				if err := json.Unmarshal([]byte(data), &batch); err == nil && len(batch.Tasks) > 0 {
					core.PrintInfo(fmt.Sprintf("Batch contains %d task(s)", len(batch.Tasks)))
				}
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

			// Handle job-specific success output (skip if raw output format)
			if isJob && res.StatusCode < 400 && !isRawOutput {
				// Try to extract execution_id from response for the log command hint
				var responseData map[string]interface{}
				executionID := ""
				if err := json.Unmarshal(body, &responseData); err == nil {
					if id, ok := responseData["execution_id"].(string); ok {
						executionID = id
					} else if id, ok := responseData["executionId"].(string); ok {
						executionID = id
					} else if id, ok := responseData["id"].(string); ok {
						executionID = id
					}
				}

				// Print monitor logs hint with command in white
				if executionID != "" {
					// Show short execution ID (first 8 chars) for readability
					shortID := executionID
					if len(shortID) > 8 {
						shortID = shortID[:8]
					}
					core.PrintInfoWithCommand("Logs:", fmt.Sprintf("bl logs job %s %s -f", resourceName, shortID))
				} else {
					core.PrintInfoWithCommand("Logs:", fmt.Sprintf("bl logs job %s -f", resourceName))
				}
				core.PrintSuccess(fmt.Sprintf("Job '%s' execution started successfully!", resourceName))
				fmt.Println()
			}

			// Output based on format
			switch outputFormat {
			case "json":
				// Raw JSON output
				fmt.Println(string(body))
			case "yaml":
				// Convert JSON to YAML
				var jsonData interface{}
				if err := json.Unmarshal(body, &jsonData); err == nil {
					yamlBytes, err := yaml.Marshal(jsonData)
					if err == nil {
						fmt.Print(string(yamlBytes))
					} else {
						fmt.Println(string(body))
					}
				} else {
					fmt.Println(string(body))
				}
			default:
				// Pretty print JSON response
				var prettyJSON bytes.Buffer
				if err := json.Indent(&prettyJSON, body, "", "  "); err == nil {
					core.Print(prettyJSON.String())
				} else {
					// If not JSON, print as string
					core.Print(string(body))
				}
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
	cmd.Flags().StringSliceVarP(&commandSecrets, "secrets", "s", []string{}, "Secrets to pass to the execution")
	cmd.Flags().StringVar(&folder, "directory", "", "Directory to run the command from")
	cmd.Flags().IntVarP(&concurrent, "concurrent", "c", 1, "Number of concurrent workers for local job execution")
	cmd.Flags().StringVarP(&outputFormat, "output", "o", "", "Output format: json, yaml")
	return cmd
}

// runRequest executes a request to a blaxel resource using the SDK client
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
	// Build request options
	opts := []option.RequestOption{}

	// Add query params
	for _, param := range params {
		parts := strings.SplitN(param, "=", 2)
		if len(parts) == 2 {
			opts = append(opts, option.WithQueryAdd(parts[0], parts[1]))
		}
	}

	// Add custom headers
	for k, v := range headers {
		opts = append(opts, option.WithHeader(k, v))
	}

	// Ensure path starts with /
	if path != "" && !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	if debug {
		baseURL := blaxel.GetRunURL()
		if local {
			baseURL = "http://localhost:1338"
		}
		fullURL := fmt.Sprintf("%s/%s/%s/%s%s", baseURL, workspace, resourceType, resourceName, path)
		if len(params) > 0 {
			fullURL += "?" + strings.Join(params, "&")
		}
		core.Print(fmt.Sprintf("Request URL: %s", fullURL))
		core.Print(fmt.Sprintf("Request Method: %s", method))
		if data != "" {
			core.Print(fmt.Sprintf("Request Body: %s", data))
		}
	}

	// Parse request body if provided
	var body []byte
	if data != "" {
		body = []byte(data)
	}

	// Use SDK client to make the request
	client := core.GetClient()

	if local {
		// For local, build the full path manually
		return client.RunLocal(ctx, method, path, body, opts...)
	}

	// Use RunWithMetadata for remote execution - it fetches the resource's metadata URL first
	// and uses that if available (for agent, function/mcp, sandbox), otherwise falls back to default URL
	return client.RunWithMetadata(ctx, workspace, resourceType, resourceName, method, path, body, opts...)
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

func runJobLocally(data string, folder string, config core.Config, concurrent int) {
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

	// Ensure concurrent is at least 1
	if concurrent < 1 {
		concurrent = 1
	}

	// Cap concurrent to number of tasks
	if concurrent > len(batch.Tasks) {
		concurrent = len(batch.Tasks)
	}

	// If concurrent is 1, run sequentially (original behavior)
	if concurrent == 1 {
		for i, task := range batch.Tasks {
			runSingleTask(i, task, folder, config, dotenvVars)
		}
		return
	}

	// Concurrent execution
	core.PrintInfo(fmt.Sprintf("Running %d tasks with %d concurrent workers", len(batch.Tasks), concurrent))

	type taskJob struct {
		index int
		task  map[string]interface{}
	}

	taskChan := make(chan taskJob, len(batch.Tasks))
	var wg sync.WaitGroup
	var errMu sync.Mutex
	var firstErr error
	var outputMu sync.Mutex

	// Start workers
	for w := 0; w < concurrent; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range taskChan {
				err := runSingleTaskParallel(job.index, job.task, folder, config, dotenvVars, &outputMu)
				if err != nil {
					errMu.Lock()
					if firstErr == nil {
						firstErr = err
					}
					errMu.Unlock()
				}
			}
		}()
	}

	// Send all tasks to the channel
	for i, task := range batch.Tasks {
		taskChan <- taskJob{index: i, task: task}
	}
	close(taskChan)

	// Wait for all workers to complete
	wg.Wait()

	if firstErr != nil {
		core.PrintError("Run", firstErr)
		core.ExitWithError(firstErr)
	}
}

func runSingleTask(i int, task map[string]interface{}, folder string, config core.Config, dotenvVars map[string]string) {
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

// prefixedWriter writes each line with a prefix, using a mutex for thread-safe output
type prefixedWriter struct {
	prefix string
	mu     *sync.Mutex
	buf    []byte
}

func newPrefixedWriter(prefix string, mu *sync.Mutex) *prefixedWriter {
	return &prefixedWriter{
		prefix: prefix,
		mu:     mu,
		buf:    make([]byte, 0),
	}
}

func (w *prefixedWriter) Write(p []byte) (n int, err error) {
	n = len(p)
	w.buf = append(w.buf, p...)

	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := w.buf[:idx]
		w.buf = w.buf[idx+1:]

		w.mu.Lock()
		fmt.Printf("%s %s\n", w.prefix, string(line))
		w.mu.Unlock()
	}
	return n, nil
}

func (w *prefixedWriter) Flush() {
	if len(w.buf) > 0 {
		w.mu.Lock()
		fmt.Printf("%s %s\n", w.prefix, string(w.buf))
		w.mu.Unlock()
		w.buf = w.buf[:0]
	}
}

func runSingleTaskParallel(i int, task map[string]interface{}, folder string, config core.Config, dotenvVars map[string]string, outputMu *sync.Mutex) error {
	prefix := fmt.Sprintf("[Task %d]", i+1)

	jsonencoded, err := json.Marshal(task)
	if err != nil {
		return fmt.Errorf("error marshalling task %d: %w", i+1, err)
	}

	outputMu.Lock()
	core.PrintInfo(fmt.Sprintf("%s Starting - Arguments: %s", prefix, string(jsonencoded)))
	outputMu.Unlock()

	cmd, err := server.FindJobCommand(task, folder, config)
	if err != nil {
		return fmt.Errorf("error finding root cmd for task %d: %w", i+1, err)
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

	// Start the command with merged stdout+stderr output.
	// On Unix this uses a PTY so child processes use line-buffered output.
	// On Windows this falls back to an os.Pipe.
	reader, err := startCmdWithOutput(cmd)
	if err != nil {
		return fmt.Errorf("error starting task %d: %w", i+1, err)
	}

	// Use prefixed writer for real-time streaming output
	outputWriter := newPrefixedWriter(prefix, outputMu)

	// io.Copy will return when the command exits and the read end gets EOF
	_, _ = io.Copy(outputWriter, reader)

	// Close the reader to release resources
	reader.Close()

	// Flush any remaining buffered output
	outputWriter.Flush()

	// Wait for the command to complete
	err = cmd.Wait()

	if err != nil {
		outputMu.Lock()
		core.PrintError("Run", fmt.Errorf("%s failed: %w", prefix, err))
		outputMu.Unlock()
		return fmt.Errorf("error executing task %d: %w", i+1, err)
	}

	outputMu.Lock()
	core.PrintInfo(fmt.Sprintf("%s Completed", prefix))
	outputMu.Unlock()

	return nil
}

package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"reflect"
	"syscall"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("get", func() *cobra.Command {
		return GetCmd()
	})
}

func GetCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get a resource",
		Long: `Retrieve information about Blaxel resources in your workspace.

A "resource" in Blaxel refers to any deployable or manageable entity:
- agents: AI agent applications
- functions/mcp: Model Context Protocol servers (tool providers)
- jobs: Batch processing tasks
- sandboxes: Isolated execution environments
- models: AI model configurations
- policies: Access control policies
- volumes: Persistent storage
- integrationconnections: External service integrations

Output Formats:
Use -o flag to control output format:
- pretty: Human-readable colored output (default)
- json: Machine-readable JSON (for scripting)
- yaml: YAML format
- table: Tabular format with columns

Watch Mode:
Use --watch to continuously monitor a resource and see updates in real-time.
Useful for tracking deployment status or watching for changes.

The command can list all resources of a type or get details for a specific one.`,
		Example: `  # List all agents
  bl get agents

  # Get specific agent details
  bl get agent my-agent

  # Get in JSON format (useful for scripting)
  bl get agent my-agent -o json

  # Watch agent status in real-time
  bl get agent my-agent --watch

  # List all resources with table output
  bl get agents -o table

  # Get MCP servers (also called functions)
  bl get functions
  bl get mcp

  # List jobs
  bl get jobs

  # Get specific job
  bl get job my-job

  # List executions for a job (nested resource)
  bl get job my-job executions

  # Get specific execution for a job
  bl get job my-job execution <execution-id>

  # Monitor sandbox status
  bl get sandbox my-sandbox --watch

  # List processes in a sandbox
  bl get sandbox my-sandbox process
  bl get sbx my-sandbox ps

  # Get specific process in a sandbox
  bl get sandbox my-sandbox process my-process`,
	}
	var watch bool
	resources := core.GetResources()
	for _, resource := range resources {
		aliases := []string{resource.Singular, resource.Short}
		if len(resource.Aliases) > 0 {
			aliases = append(aliases, resource.Aliases...)
		}

		// Special handling for images - use custom command
		if resource.Kind == "Image" {
			imageCmd := GetImagesCmd()
			// Add both singular and plural
			cmd.AddCommand(imageCmd)
			continue
		}

		// Capture resource in closure for ValidArgsFunction
		resourceKind := resource.Kind

		subcmd := &cobra.Command{
			Use:               resource.Plural,
			Aliases:           aliases,
			Short:             fmt.Sprintf("Get a %s", resource.Kind),
			ValidArgsFunction: GetResourceValidArgsFunction(resourceKind),
			Run: func(cmd *cobra.Command, args []string) {
				// Special handling for nested resources (e.g., job executions, sandbox processes)
				if resource.Kind == "Job" && len(args) >= 2 {
					// Check if this is a nested resource request
					if HandleJobNestedResource(args) {
						return
					}
				}

				if resource.Kind == "Sandbox" && len(args) >= 2 {
					// Check if this is a nested resource request (e.g., processes)
					if HandleSandboxNestedResource(args) {
						return
					}
				}

				if watch {
					seconds := 2
					duration := time.Duration(seconds) * time.Second

					// Execute immediately before starting the ticker
					executeAndDisplayWatch(args, *resource, seconds)

					// Create a ticker to periodically fetch updates
					ticker := time.NewTicker(duration)
					defer ticker.Stop()

					// Handle Ctrl+C gracefully
					sigChan := make(chan os.Signal, 1)
					signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

					for {
						select {
						case <-ticker.C:
							executeAndDisplayWatch(args, *resource, seconds)
						case <-sigChan:
							fmt.Println("\nStopped watching.")
							return
						}
					}
				} else {
					if len(args) == 0 {
						ListFn(resource)
						return
					}
					if len(args) == 1 {
						GetFn(resource, args[0])
					}
				}
			},
		}

		cmd.AddCommand(subcmd)
	}
	cmd.PersistentFlags().BoolVarP(&watch, "watch", "", false, "After listing/getting the requested object, watch for changes.")
	return cmd
}

func GetFn(resource *core.Resource, name string) {
	ctx := context.Background()
	formattedError := fmt.Sprintf("Resource %s:%s error: ", resource.Kind, name)

	// Use reflect to call the function
	funcValue := reflect.ValueOf(resource.Get)
	if funcValue.Kind() != reflect.Func {
		err := fmt.Errorf("%s%s", formattedError, "fn is not a valid function")
		core.PrintError("Get", err)
		core.ExitWithError(err)
	}

	// Build arguments: (ctx, name, ...opts)
	fnargs := []reflect.Value{reflect.ValueOf(ctx), reflect.ValueOf(name)}

	// Check if the function expects more arguments (e.g., params struct)
	// Some SDK methods like Jobs.Get require (ctx, name, params, ...opts)
	// Others like Policies.Get only have (ctx, name, ...opts)
	// We need to add zero values for any non-variadic params between name and the variadic opts
	funcType := funcValue.Type()
	if funcType.NumIn() > 2 {
		// For variadic functions, the last param is the variadic (e.g., ...option.RequestOption)
		// We need to add parameters between index 2 and the variadic parameter
		lastNonVariadicIdx := funcType.NumIn()
		if funcType.IsVariadic() {
			lastNonVariadicIdx = funcType.NumIn() - 1
		}
		for i := 2; i < lastNonVariadicIdx; i++ {
			paramsType := funcType.In(i)
			fnargs = append(fnargs, reflect.Zero(paramsType))
		}
	}

	// Call the function with the arguments
	results := funcValue.Call(fnargs)

	// Handle the results based on your needs
	if len(results) <= 1 {
		return
	}

	if err, ok := results[1].Interface().(error); ok && err != nil {
		fmt.Printf("%s%v\n", formattedError, err)
		core.ExitWithError(err)
	}

	// The new SDK returns typed responses, not *http.Response
	// Convert result to interface{} for output
	result := results[0].Interface()
	if result == nil {
		err := fmt.Errorf("%s%s", formattedError, "no result returned")
		fmt.Println(err)
		core.ExitWithError(err)
	}

	// Convert to JSON and back to interface{} for consistent handling
	jsonData, err := json.Marshal(result)
	if err != nil {
		fmt.Printf("%s%v\n", formattedError, err)
		core.ExitWithError(err)
	}

	var res interface{}
	if err := json.Unmarshal(jsonData, &res); err != nil {
		fmt.Printf("%s%v\n", formattedError, err)
		core.ExitWithError(err)
	}

	core.Output(*resource, []interface{}{res}, core.GetOutputFormat())
}

func ListFn(resource *core.Resource) {
	slices, err := ListExec(resource)
	if err != nil {
		fmt.Println(err)
		core.ExitWithError(err)
	}
	// Check the output format
	core.Output(*resource, slices, core.GetOutputFormat())
}

func ListExec(resource *core.Resource) ([]interface{}, error) {
	formattedError := fmt.Sprintf("Resource %s error: ", resource.Kind)

	ctx := context.Background()
	// Use reflect to call the function
	funcValue := reflect.ValueOf(resource.List)
	if funcValue.Kind() != reflect.Func {
		return nil, fmt.Errorf("fn is not a valid function")
	}

	// Build arguments: (ctx, ...opts)
	fnargs := []reflect.Value{reflect.ValueOf(ctx)}

	// Call the function with the arguments
	results := funcValue.Call(fnargs)

	// Handle the results based on your needs
	if len(results) <= 1 {
		return nil, nil
	}

	if err, ok := results[1].Interface().(error); ok && err != nil {
		return nil, fmt.Errorf("%s%v", formattedError, err)
	}

	// The new SDK returns typed responses (e.g., *[]Agent), not *http.Response
	result := results[0].Interface()
	if result == nil {
		return nil, errors.New("no result returned")
	}

	// Convert to JSON and back to []interface{} for consistent handling
	jsonData, err := json.Marshal(result)
	if err != nil {
		return nil, err
	}

	var slices []interface{}
	if err := json.Unmarshal(jsonData, &slices); err != nil {
		return nil, err
	}

	return slices, nil
}

// Helper function to execute and display results
func executeAndDisplayWatch(args []string, resource core.Resource, seconds int) {
	// Create a pipe to capture output
	r, w, _ := os.Pipe()
	// Save the original stdout
	stdout := os.Stdout
	// Set stdout to our pipe
	os.Stdout = w

	// Execute the resource function
	if len(args) == 0 {
		ListFn(&resource)
	} else if len(args) == 1 {
		GetFn(&resource, args[0])
	}

	// Close the write end of the pipe
	_ = w.Close()

	// Read the output from the pipe
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)

	// Restore stdout
	os.Stdout = stdout

	// Clear the screen
	fmt.Print("\033[2J\033[H")

	// Print the timestamp and output
	fmt.Printf("Every %ds: %s\n", seconds, time.Now().Format("Mon Jan 2 15:04:05 2006"))
	fmt.Print(buf.String())
}

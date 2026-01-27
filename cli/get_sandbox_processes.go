package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"gopkg.in/yaml.v3"
)

// HandleSandboxNestedResource handles nested resources for sandboxes (like processes)
// Returns true if a nested resource was handled, false if this is a regular get
func HandleSandboxNestedResource(args []string) bool {
	if len(args) < 2 {
		return false
	}

	sandboxName := args[0]
	nestedResource := args[1]

	switch nestedResource {
	case "processes", "process", "proc", "procs", "ps":
		// Check if process name is provided
		if len(args) >= 3 {
			processName := args[2]
			// Get specific process
			getSandboxProcess(sandboxName, processName)
		} else {
			// List all processes for this sandbox
			listSandboxProcesses(sandboxName)
		}
		return true

	default:
		// Not a nested resource, let the normal get handler process it
		return false
	}
}

func listSandboxProcesses(sandboxName string) {
	ctx := context.Background()
	client := core.GetClient()

	// Get the sandbox instance
	sandboxInstance, err := client.Sandboxes.GetInstance(ctx, sandboxName)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to get sandbox instance '%s': %w", sandboxName, err))
		os.Exit(1)
	}

	// List processes
	processes, err := sandboxInstance.Process.List(ctx)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to list processes for sandbox '%s': %w", sandboxName, err))
		os.Exit(1)
	}

	if processes == nil || len(*processes) == 0 {
		core.PrintInfo(fmt.Sprintf("No processes found in sandbox '%s'", sandboxName))
		return
	}

	// Check output format
	outputFormat := core.GetOutputFormat()
	if outputFormat == "json" || outputFormat == "yaml" {
		outputProcessData(processes, outputFormat)
		return
	}

	// For table output, convert to maps
	jsonData, err := json.Marshal(processes)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to marshal processes: %w", err))
		os.Exit(1)
	}

	var slices []interface{}
	if err := json.Unmarshal(jsonData, &slices); err != nil {
		core.PrintError("Get", fmt.Errorf("failed to unmarshal processes: %w", err))
		os.Exit(1)
	}

	// Create a pseudo-resource for output formatting
	resource := core.Resource{
		Kind:     "Process",
		Plural:   "processes",
		Singular: "process",
		Fields: []core.Field{
			{Key: "NAME", Value: "name"},
			{Key: "PID", Value: "pid"},
			{Key: "STATUS", Value: "status"},
			{Key: "COMMAND", Value: "command"},
			{Key: "EXIT_CODE", Value: "exitCode"},
			{Key: "STARTED_AT", Value: "startedAt", Special: "datetime"},
		},
	}

	core.Output(resource, slices, outputFormat)
}

func getSandboxProcess(sandboxName, processName string) {
	ctx := context.Background()
	client := core.GetClient()

	// Get the sandbox instance
	sandboxInstance, err := client.Sandboxes.GetInstance(ctx, sandboxName)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to get sandbox instance '%s': %w", sandboxName, err))
		os.Exit(1)
	}

	// Get specific process
	process, err := sandboxInstance.Process.Get(ctx, processName)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to get process '%s' in sandbox '%s': %w", processName, sandboxName, err))
		os.Exit(1)
	}

	if process == nil {
		core.PrintError("Get", fmt.Errorf("no process data returned"))
		os.Exit(1)
	}

	// Check output format
	outputFormat := core.GetOutputFormat()
	if outputFormat == "json" || outputFormat == "yaml" {
		outputProcessData(process, outputFormat)
		return
	}

	// For table output, convert to map
	jsonData, err := json.Marshal(process)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to marshal process: %w", err))
		os.Exit(1)
	}

	var processMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &processMap); err != nil {
		core.PrintError("Get", fmt.Errorf("failed to unmarshal process: %w", err))
		os.Exit(1)
	}

	// Create a pseudo-resource for output formatting
	resource := core.Resource{
		Kind:     "Process",
		Plural:   "processes",
		Singular: "process",
		Fields: []core.Field{
			{Key: "NAME", Value: "name"},
			{Key: "PID", Value: "pid"},
			{Key: "STATUS", Value: "status"},
			{Key: "COMMAND", Value: "command"},
			{Key: "EXIT_CODE", Value: "exitCode"},
			{Key: "STARTED_AT", Value: "startedAt", Special: "datetime"},
			{Key: "COMPLETED_AT", Value: "completedAt", Special: "datetime"},
		},
	}

	core.Output(resource, []interface{}{processMap}, outputFormat)
}

func getSandboxProcessLogs(sandboxName, processName string) {
	ctx := context.Background()
	client := core.GetClient()

	// Get the sandbox instance
	sandboxInstance, err := client.Sandboxes.GetInstance(ctx, sandboxName)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to get sandbox instance '%s': %w", sandboxName, err))
		os.Exit(1)
	}

	// Get process logs
	logs, err := sandboxInstance.Process.GetLogs(ctx, processName)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to get logs for process '%s' in sandbox '%s': %w", processName, sandboxName, err))
		os.Exit(1)
	}

	if logs == nil {
		core.PrintError("Get", fmt.Errorf("no logs data returned"))
		os.Exit(1)
	}

	// Check output format
	outputFormat := core.GetOutputFormat()
	if outputFormat == "json" || outputFormat == "yaml" {
		// Convert ProcessLogs struct to map for output formatting
		jsonData, err := json.Marshal(logs)
		if err != nil {
			core.PrintError("Get", fmt.Errorf("failed to marshal logs: %w", err))
			os.Exit(1)
		}

		var logsMap map[string]interface{}
		if err := json.Unmarshal(jsonData, &logsMap); err != nil {
			core.PrintError("Get", fmt.Errorf("failed to unmarshal logs: %w", err))
			os.Exit(1)
		}

		// Create a pseudo-resource for output formatting
		resource := core.Resource{
			Kind:     "ProcessLogs",
			Plural:   "logs",
			Singular: "log",
			Fields: []core.Field{
				{Key: "LOGS", Value: "logs"},
				{Key: "STDOUT", Value: "stdout"},
				{Key: "STDERR", Value: "stderr"},
			},
		}

		core.Output(resource, []interface{}{logsMap}, outputFormat)
	} else {
		// For pretty/default output, just print the logs directly
		if logs.Logs != "" {
			fmt.Print(logs.Logs)
		} else {
			// Fallback to stdout/stderr if logs field is empty
			if logs.Stdout != "" {
				fmt.Print(logs.Stdout)
			}
			if logs.Stderr != "" {
				fmt.Fprint(os.Stderr, logs.Stderr)
			}
		}
	}
}

// outputProcessData outputs process data in JSON or YAML format
func outputProcessData(data interface{}, format string) {
	// First convert to JSON to handle unexported fields in SDK structs
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to marshal process data: %w", err))
		os.Exit(1)
	}

	if format == "json" {
		fmt.Println(string(jsonData))
		return
	}

	// For YAML, unmarshal JSON to generic type first to avoid reflection issues
	var genericData interface{}
	if err := json.Unmarshal(jsonData, &genericData); err != nil {
		core.PrintError("Get", fmt.Errorf("failed to unmarshal process data: %w", err))
		os.Exit(1)
	}

	yamlData, err := yaml.Marshal(genericData)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to marshal process data to YAML: %w", err))
		os.Exit(1)
	}
	fmt.Print(string(yamlData))
}

// streamSandboxProcessLogs streams process logs in real-time using SDK's StreamLogs
func streamSandboxProcessLogs(sandboxName, processName string) {
	ctx := context.Background()
	client := core.GetClient()

	// Get the sandbox instance
	sandboxInstance, err := client.Sandboxes.GetInstance(ctx, sandboxName)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to get sandbox instance '%s': %w", sandboxName, err))
		os.Exit(1)
	}

	// Handle Ctrl+C gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Start streaming logs using SDK's StreamLogs
	streamControl := sandboxInstance.Process.StreamLogs(ctx, processName, blaxel.ProcessStreamOptions{
		OnLog: func(log string) {
			printWithNewline(log)
		},
		OnStdout: func(stdout string) {
			printWithNewline(stdout)
		},
		OnStderr: func(stderr string) {
			printWithNewlineStderr(stderr)
		},
		OnError: func(err error) {
			core.PrintError("Stream", err)
		},
	})

	// Wait for interrupt signal
	<-sigChan
	streamControl.Close()
	fmt.Println("\nStopped streaming logs.")
}

// printWithNewline prints a string and ensures it ends with a newline
func printWithNewline(s string) {
	fmt.Print(s)
	if !strings.HasSuffix(s, "\n") {
		fmt.Println()
	}
}

// printWithNewlineStderr prints a string to stderr and ensures it ends with a newline
func printWithNewlineStderr(s string) {
	fmt.Fprint(os.Stderr, s)
	if !strings.HasSuffix(s, "\n") {
		fmt.Fprintln(os.Stderr)
	}
}

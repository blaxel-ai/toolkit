package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
)

// HandleJobNestedResource handles nested resources for jobs (like executions)
// Returns true if a nested resource was handled, false if this is a regular get
func HandleJobNestedResource(args []string) bool {
	if len(args) < 2 {
		return false
	}

	jobName := args[0]
	nestedResource := args[1]

	switch nestedResource {
	case "executions":
		// Check if execution ID is provided
		if len(args) >= 3 {
			// Get specific execution
			executionID := args[2]
			getJobExecution(jobName, executionID)
		} else {
			// List all executions for this job
			listJobExecutions(jobName)
		}
		return true

	case "execution":
		// Get specific execution
		if len(args) < 3 {
			core.PrintError("Get", fmt.Errorf("execution ID required: bl get job %s execution <execution-id>", jobName))
			os.Exit(1)
		}
		executionID := args[2]
		getJobExecution(jobName, executionID)
		return true

	default:
		// Not a nested resource, let the normal get handler process it
		return false
	}
}

func listJobExecutions(jobName string) {
	ctx := context.Background()
	client := core.GetClient()

	executions, err := client.Jobs.Executions.List(ctx, jobName, blaxel.JobExecutionListParams{})
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to list job executions: %w", err))
		os.Exit(1)
	}

	if executions == nil || len(*executions) == 0 {
		core.PrintError("Get", fmt.Errorf("no executions found"))
		os.Exit(1)
	}

	// Convert JobExecution structs to maps for output formatting
	// Marshal to JSON and unmarshal back to []interface{} to get map representation
	jsonData, err := json.Marshal(executions)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to marshal executions: %w", err))
		os.Exit(1)
	}

	var slices []interface{}
	if err := json.Unmarshal(jsonData, &slices); err != nil {
		core.PrintError("Get", fmt.Errorf("failed to unmarshal executions: %w", err))
		os.Exit(1)
	}

	// Create a pseudo-resource for output formatting
	resource := core.Resource{
		Kind:     "JobExecution",
		Plural:   "executions",
		Singular: "execution",
		Fields: []core.Field{
			{Key: "WORKSPACE", Value: "metadata.workspace"},
			{Key: "JOB", Value: "metadata.job"},
			{Key: "ID", Value: "metadata.id"},
			{Key: "TASKS", Value: "spec.tasks", Special: "count"},
			{Key: "STATUS", Value: "status"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "datetime"},
		},
	}

	core.Output(resource, slices, core.GetOutputFormat())
}

func getJobExecution(jobName, executionID string) {
	ctx := context.Background()
	client := core.GetClient()

	execution, err := client.Jobs.Executions.Get(ctx, executionID, blaxel.JobExecutionGetParams{JobID: jobName})
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to get job execution: %w", err))
		os.Exit(1)
	}

	if execution == nil {
		core.PrintError("Get", fmt.Errorf("no execution data returned"))
		os.Exit(1)
	}

	// Convert JobExecution struct to map for output formatting
	// Marshal to JSON and unmarshal back to map[string]interface{}
	jsonData, err := json.Marshal(execution)
	if err != nil {
		core.PrintError("Get", fmt.Errorf("failed to marshal execution: %w", err))
		os.Exit(1)
	}

	var executionMap map[string]interface{}
	if err := json.Unmarshal(jsonData, &executionMap); err != nil {
		core.PrintError("Get", fmt.Errorf("failed to unmarshal execution: %w", err))
		os.Exit(1)
	}

	// Create a pseudo-resource for output formatting
	resource := core.Resource{
		Kind:     "JobExecution",
		Plural:   "executions",
		Singular: "execution",
		Fields: []core.Field{
			{Key: "WORKSPACE", Value: "metadata.workspace"},
			{Key: "JOB", Value: "metadata.job"},
			{Key: "ID", Value: "metadata.id"},
			{Key: "TASKS", Value: "spec.tasks", Special: "count"},
			{Key: "STATUS", Value: "status"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "datetime"},
		},
	}

	core.Output(resource, []interface{}{executionMap}, core.GetOutputFormat())
}

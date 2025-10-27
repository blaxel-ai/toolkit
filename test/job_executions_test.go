package test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
)

func TestJobExecutions(t *testing.T) {
	// Use dev environment and charles workspace
	os.Setenv("BL_ENV", "dev")
	core.SetEnvs()

	workspace := "charles"
	jobName := "mk3"

	// Get credentials
	credentials := sdk.LoadCredentials(workspace)
	if !credentials.IsValid() {
		t.Skip("No valid credentials for workspace charles")
	}

	// Create API client
	client, err := sdk.NewClientWithCredentials(sdk.RunClientWithCredentials{
		ApiURL:      core.GetBaseURL(),
		RunURL:      core.GetRunURL(),
		Credentials: credentials,
		Workspace:   workspace,
	})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	// Create job execution helper
	helper := sdk.NewJobExecutionHelper(client, workspace, jobName)

	ctx := context.Background()

	// Test listing executions
	t.Run("List Executions", func(t *testing.T) {
		executions, err := helper.List(ctx)
		if err != nil {
			t.Errorf("Failed to list executions: %v", err)
			return
		}
		t.Logf("Found %d executions", len(executions))
	})

	// Test creating an execution
	t.Run("Create Execution", func(t *testing.T) {
		request := sdk.CreateJobExecutionRequest{
			Id: stringPtr("test-exec-" + time.Now().Format("20060102-150405")),
			Tasks: &[]map[string]interface{}{
				{
					"message": "Test task 1",
				},
			},
		}

		executionID, err := helper.Create(ctx, request)
		if err != nil {
			t.Errorf("Failed to create execution: %v", err)
			return
		}

		t.Logf("Created execution: %s", executionID)

		// Get the execution
		execution, err := helper.Get(ctx, executionID)
		if err != nil {
			t.Errorf("Failed to get execution: %v", err)
			return
		}

		t.Logf("Execution status: %s", *execution.Status)

		// Get status
		status, err := helper.GetStatus(ctx, executionID)
		if err != nil {
			t.Errorf("Failed to get status: %v", err)
			return
		}

		t.Logf("Status via helper: %s", status)

		// Clean up - cancel the execution
		if err := helper.Cancel(ctx, executionID); err != nil {
			t.Logf("Failed to cancel execution (may be expected): %v", err)
		} else {
			t.Log("Execution cancelled successfully")
		}
	})
}

func stringPtr(s string) *string {
	return &s
}

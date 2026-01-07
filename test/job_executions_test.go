package test

import (
	"context"
	"os"
	"testing"
	"time"

	blaxel "github.com/stainless-sdks/blaxel-go"
	"github.com/stainless-sdks/blaxel-go/option"
)

func TestJobExecutions(t *testing.T) {
	// Use dev environment and charles workspace
	os.Setenv("BL_ENV", "dev")
	blaxel.SetEnvironment(blaxel.EnvDevelopment)
	blaxel.ApplyEnvironmentOverrides()

	workspace := "charles"
	jobName := "mk3"

	// Get credentials
	credentials, _ := blaxel.LoadCredentials(workspace)
	if !credentials.IsValid() {
		t.Skip("No valid credentials for workspace charles")
	}

	// Create API client
	opts := []option.RequestOption{
		option.WithBaseURL(blaxel.GetBaseURL()),
		option.WithWorkspace(workspace),
	}

	if credentials.APIKey != "" {
		opts = append(opts, option.WithAPIKey(credentials.APIKey))
	} else if credentials.AccessToken != "" {
		opts = append(opts, option.WithAccessToken(credentials.AccessToken))
	} else if credentials.ClientCredentials != "" {
		opts = append(opts, option.WithClientCredentials(credentials.ClientCredentials))
	}

	client, err := blaxel.NewDefaultClient(opts...)
	if err != nil {
		t.Errorf("Failed to create client: %v", err)
		return
	}

	ctx := context.Background()

	// Test listing executions
	t.Run("List Executions", func(t *testing.T) {
		executions, err := client.Jobs.Executions.List(ctx, jobName, blaxel.JobExecutionListParams{})
		if err != nil {
			t.Errorf("Failed to list executions: %v", err)
			return
		}
		t.Logf("Found %d executions", len(*executions))
	})

	// Test creating an execution
	t.Run("Create Execution", func(t *testing.T) {
		execID := "test-exec-" + time.Now().Format("20060102-150405")
		request := blaxel.JobExecutionNewParams{
			CreateJobExecutionRequest: blaxel.CreateJobExecutionRequestParam{
				ID: blaxel.String(execID),
				Tasks: []any{
					map[string]interface{}{
						"message": "Test task 1",
					},
				},
			},
		}

		execution, err := client.Jobs.Executions.New(ctx, jobName, request)
		if err != nil {
			t.Errorf("Failed to create execution: %v", err)
			return
		}

		executionID := execution.Metadata.ID
		t.Logf("Created execution: %s", executionID)

		// Get the execution
		getExecution, err := client.Jobs.Executions.Get(ctx, executionID, blaxel.JobExecutionGetParams{JobID: jobName})
		if err != nil {
			t.Errorf("Failed to get execution: %v", err)
			return
		}

		t.Logf("Execution status: %s", getExecution.Status)

		// Clean up - cancel the execution
		_, cancelErr := client.Jobs.Executions.Delete(ctx, executionID, blaxel.JobExecutionDeleteParams{JobID: jobName})
		if cancelErr != nil {
			t.Logf("Failed to cancel execution (may be expected): %v", cancelErr)
		} else {
			t.Log("Execution cancelled successfully")
		}
	})
}

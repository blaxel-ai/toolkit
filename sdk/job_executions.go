package sdk

import (
	"context"
	"encoding/json"
	"fmt"
)

// JobExecutionHelper provides convenient methods for working with job executions
type JobExecutionHelper struct {
	client    *ClientWithResponses
	workspace string
	jobName   string
}

// NewJobExecutionHelper creates a new job execution helper
func NewJobExecutionHelper(client *ClientWithResponses, workspace, jobName string) *JobExecutionHelper {
	return &JobExecutionHelper{
		client:    client,
		workspace: workspace,
		jobName:   jobName,
	}
}

// Create creates a new job execution and returns the execution ID
func (h *JobExecutionHelper) Create(ctx context.Context, request CreateJobExecutionRequest) (string, error) {
	response, err := h.client.CreateJobExecutionWithResponse(ctx, h.jobName, request)
	if err != nil {
		return "", fmt.Errorf("failed to create job execution: %w", err)
	}

	if response.StatusCode() != 200 {
		return "", fmt.Errorf("failed to create job execution: %s", response.Status())
	}

	// Parse the response body to extract executionId
	var result struct {
		ExecutionId string `json:"executionId"`
	}
	if err := json.Unmarshal(response.Body, &result); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if result.ExecutionId == "" {
		return "", fmt.Errorf("no execution ID returned")
	}

	return result.ExecutionId, nil
}

// Get retrieves a specific job execution by ID
func (h *JobExecutionHelper) Get(ctx context.Context, executionID string) (*JobExecution, error) {
	response, err := h.client.GetJobExecutionWithResponse(ctx, h.jobName, executionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get job execution: %w", err)
	}

	if response.StatusCode() == 404 {
		return nil, fmt.Errorf("execution '%s' not found for job '%s'", executionID, h.jobName)
	}

	if response.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to get job execution: %s", response.Status())
	}

	if response.JSON200 == nil {
		return nil, fmt.Errorf("no execution data returned")
	}

	return response.JSON200, nil
}

// List retrieves all executions for the job
func (h *JobExecutionHelper) List(ctx context.Context) ([]JobExecution, error) {
	response, err := h.client.ListJobExecutionsWithResponse(ctx, h.jobName, &ListJobExecutionsParams{})
	if err != nil {
		return nil, fmt.Errorf("failed to list job executions: %w", err)
	}

	if response.StatusCode() != 200 {
		return nil, fmt.Errorf("failed to list job executions: %s", response.Status())
	}

	if response.JSON200 == nil {
		return []JobExecution{}, nil
	}

	return *response.JSON200, nil
}

// GetStatus returns the status of a specific execution
func (h *JobExecutionHelper) GetStatus(ctx context.Context, executionID string) (string, error) {
	execution, err := h.Get(ctx, executionID)
	if err != nil {
		return "", err
	}

	if execution.Status == nil {
		return "UNKNOWN", nil
	}

	return *execution.Status, nil
}

// Cancel cancels a specific job execution
func (h *JobExecutionHelper) Cancel(ctx context.Context, executionID string) error {
	response, err := h.client.DeleteJobExecutionWithResponse(ctx, h.jobName, executionID)
	if err != nil {
		return fmt.Errorf("failed to cancel job execution: %w", err)
	}

	if response.StatusCode() != 200 {
		return fmt.Errorf("failed to cancel job execution: %s", response.Status())
	}

	return nil
}

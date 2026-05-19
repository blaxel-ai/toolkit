package core

import (
	"context"
	"fmt"

	blaxel "github.com/blaxel-ai/sdk-go"
)

const listAllPageLimit int64 = 200

func listAllPages[T any](fetch func(cursor string) ([]T, bool, string, error)) (*[]T, error) {
	var all []T
	cursor := ""
	seenCursors := map[string]struct{}{}

	for {
		data, hasMore, nextCursor, err := fetch(cursor)
		if err != nil {
			return nil, err
		}

		all = append(all, data...)

		if !hasMore || nextCursor == "" {
			return &all, nil
		}

		if _, seen := seenCursors[nextCursor]; seen {
			return nil, fmt.Errorf("pagination cursor loop detected")
		}
		seenCursors[nextCursor] = struct{}{}
		cursor = nextCursor
	}
}

func ListAllAgents(ctx context.Context, c *blaxel.Client) (*[]blaxel.Agent, error) {
	return listAllPages(func(cursor string) ([]blaxel.Agent, bool, string, error) {
		params := blaxel.AgentListParams{Limit: blaxel.Int(listAllPageLimit)}
		if cursor != "" {
			params.Cursor = blaxel.String(cursor)
		}
		page, err := c.Agents.List(ctx, params)
		if err != nil || page == nil {
			return nil, false, "", err
		}
		return page.Data, page.Meta.HasMore, page.Meta.NextCursor, nil
	})
}

func ListAllPolicies(ctx context.Context, c *blaxel.Client) (*[]blaxel.Policy, error) {
	return listAllPages(func(cursor string) ([]blaxel.Policy, bool, string, error) {
		params := blaxel.PolicyListParams{Limit: blaxel.Int(listAllPageLimit)}
		if cursor != "" {
			params.Cursor = blaxel.String(cursor)
		}
		page, err := c.Policies.List(ctx, params)
		if err != nil || page == nil {
			return nil, false, "", err
		}
		return page.Data, page.Meta.HasMore, page.Meta.NextCursor, nil
	})
}

func ListAllModels(ctx context.Context, c *blaxel.Client) (*[]blaxel.Model, error) {
	return listAllPages(func(cursor string) ([]blaxel.Model, bool, string, error) {
		params := blaxel.ModelListParams{Limit: blaxel.Int(listAllPageLimit)}
		if cursor != "" {
			params.Cursor = blaxel.String(cursor)
		}
		page, err := c.Models.List(ctx, params)
		if err != nil || page == nil {
			return nil, false, "", err
		}
		return page.Data, page.Meta.HasMore, page.Meta.NextCursor, nil
	})
}

func ListAllFunctions(ctx context.Context, c *blaxel.Client) (*[]blaxel.Function, error) {
	return listAllPages(func(cursor string) ([]blaxel.Function, bool, string, error) {
		params := blaxel.FunctionListParams{Limit: blaxel.Int(listAllPageLimit)}
		if cursor != "" {
			params.Cursor = blaxel.String(cursor)
		}
		page, err := c.Functions.List(ctx, params)
		if err != nil || page == nil {
			return nil, false, "", err
		}
		return page.Data, page.Meta.HasMore, page.Meta.NextCursor, nil
	})
}

func ListAllSandboxes(ctx context.Context, c *blaxel.Client) (*[]blaxel.Sandbox, error) {
	return listAllPages(func(cursor string) ([]blaxel.Sandbox, bool, string, error) {
		params := blaxel.SandboxListParams{Limit: blaxel.Int(listAllPageLimit)}
		if cursor != "" {
			params.Cursor = blaxel.String(cursor)
		}
		page, err := c.Sandboxes.List(ctx, params)
		if err != nil || page == nil {
			return nil, false, "", err
		}
		return page.Data, page.Meta.HasMore, page.Meta.NextCursor, nil
	})
}

func ListAllJobs(ctx context.Context, c *blaxel.Client) (*[]blaxel.Job, error) {
	return listAllPages(func(cursor string) ([]blaxel.Job, bool, string, error) {
		params := blaxel.JobListParams{Limit: blaxel.Int(listAllPageLimit)}
		if cursor != "" {
			params.Cursor = blaxel.String(cursor)
		}
		page, err := c.Jobs.List(ctx, params)
		if err != nil || page == nil {
			return nil, false, "", err
		}
		return page.Data, page.Meta.HasMore, page.Meta.NextCursor, nil
	})
}

func ListAllVolumes(ctx context.Context, c *blaxel.Client) (*[]blaxel.VolumeListResponseData, error) {
	return listAllPages(func(cursor string) ([]blaxel.VolumeListResponseData, bool, string, error) {
		params := blaxel.VolumeListParams{Limit: blaxel.Int(listAllPageLimit)}
		if cursor != "" {
			params.Cursor = blaxel.String(cursor)
		}
		page, err := c.Volumes.List(ctx, params)
		if err != nil || page == nil {
			return nil, false, "", err
		}
		return page.Data, page.Meta.HasMore, page.Meta.NextCursor, nil
	})
}

func ListAllDrives(ctx context.Context, c *blaxel.Client) (*[]blaxel.DriveListResponseData, error) {
	return listAllPages(func(cursor string) ([]blaxel.DriveListResponseData, bool, string, error) {
		params := blaxel.DriveListParams{Limit: blaxel.Int(listAllPageLimit)}
		if cursor != "" {
			params.Cursor = blaxel.String(cursor)
		}
		page, err := c.Drives.List(ctx, params)
		if err != nil || page == nil {
			return nil, false, "", err
		}
		return page.Data, page.Meta.HasMore, page.Meta.NextCursor, nil
	})
}

func ListAllJobExecutions(ctx context.Context, c *blaxel.Client, jobName string) (*[]blaxel.JobExecution, error) {
	return listAllPages(func(cursor string) ([]blaxel.JobExecution, bool, string, error) {
		params := blaxel.JobExecutionListParams{Limit: blaxel.Int(listAllPageLimit)}
		if cursor != "" {
			params.Cursor = blaxel.String(cursor)
		}
		page, err := c.Jobs.Executions.List(ctx, jobName, params)
		if err != nil || page == nil {
			return nil, false, "", err
		}
		return page.Data, page.Meta.HasMore, page.Meta.NextCursor, nil
	})
}

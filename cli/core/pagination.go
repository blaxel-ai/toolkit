package core

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"golang.org/x/term"
)

// blaxelVersionPaginated is the minimum API version that enables cursor-paginated
// listing responses ({data, meta}) instead of bare arrays.
const blaxelVersionPaginated = "2026-04-28"

// DefaultPageLimit is the per-page size sent with paginated requests. Matches
// the API maximum of 200.
const DefaultPageLimit = 200

// PaginationMeta mirrors the controlplane's PaginationMeta shape.
type PaginationMeta struct {
	NextCursor string `json:"nextCursor"`
	HasMore    bool   `json:"hasMore"`
	Total      int    `json:"total"`
}

// PaginatedResult holds one page of items together with pagination metadata.
type PaginatedResult struct {
	Items []any
	Meta  PaginationMeta
}

// paginatedResponse is the envelope returned by paginated listing endpoints.
type paginatedResponse struct {
	Data json.RawMessage `json:"data"`
	Meta PaginationMeta  `json:"meta"`
}

// fetchPage fetches a single page from a paginated listing endpoint.
func fetchPage(c *blaxel.Client, apiPath string, limit int, cursor string) (PaginatedResult, error) {
	ctx := context.Background()
	path := fmt.Sprintf("%s?limit=%d", apiPath, limit)
	if cursor != "" {
		path += "&cursor=" + cursor
	}

	var page paginatedResponse
	err := c.Get(ctx, path, nil, &page,
		option.WithHeader("Blaxel-Version", blaxelVersionPaginated),
		option.WithHeader("Accept", "application/json"),
	)
	if err != nil {
		return PaginatedResult{}, fmt.Errorf("paginated list %s: %w", apiPath, err)
	}

	var items []any
	if err := json.Unmarshal(page.Data, &items); err != nil {
		return PaginatedResult{}, fmt.Errorf("paginated list %s: decode data: %w", apiPath, err)
	}
	return PaginatedResult{Items: items, Meta: page.Meta}, nil
}

// ListPaginated fetches a single page of items for the given resource. It
// sends the Blaxel-Version header so the API returns the cursor-paginated
// response shape. The returned PaginatedResult contains the items and
// pagination metadata (nextCursor, hasMore, total).
func ListPaginated(resource *Resource, limit int, cursor string) (PaginatedResult, error) {
	c := GetClient()
	if c == nil {
		return PaginatedResult{}, fmt.Errorf("client not initialized")
	}
	if !resource.Paginated || resource.APIPath == "" {
		return PaginatedResult{}, fmt.Errorf("resource %s does not support pagination", resource.Kind)
	}
	if limit <= 0 || limit > DefaultPageLimit {
		limit = DefaultPageLimit
	}
	return fetchPage(c, resource.APIPath, limit, cursor)
}

// ListAllPaginated fetches every page from a paginated listing endpoint,
// showing a progress indicator on stderr when the output is a terminal.
// Returns all collected items.
func ListAllPaginated(resource *Resource) ([]any, error) {
	c := GetClient()
	if c == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	if !resource.Paginated || resource.APIPath == "" {
		return nil, fmt.Errorf("resource %s does not support pagination", resource.Kind)
	}

	isTTY := term.IsTerminal(int(os.Stderr.Fd()))
	var all []any
	cursor := ""

	for {
		result, err := fetchPage(c, resource.APIPath, DefaultPageLimit, cursor)
		if err != nil {
			return nil, err
		}
		all = append(all, result.Items...)

		if isTTY {
			if result.Meta.Total > 0 {
				fmt.Fprintf(os.Stderr, "\rFetching %s... %d/%d", resource.Plural, len(all), result.Meta.Total)
			} else {
				fmt.Fprintf(os.Stderr, "\rFetching %s... %d", resource.Plural, len(all))
			}
		}

		if !result.Meta.HasMore || result.Meta.NextCursor == "" {
			break
		}
		cursor = result.Meta.NextCursor
	}

	// Clear the progress line.
	if isTTY {
		fmt.Fprintf(os.Stderr, "\r\033[K")
	}

	return all, nil
}

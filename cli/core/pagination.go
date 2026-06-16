package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"golang.org/x/term"
)

// blaxelVersionPaginated is the minimum API version that enables cursor-paginated
// listing responses ({data, meta}) instead of bare arrays.
const blaxelVersionPaginated = "2026-04-28"

// DefaultPageLimit is the default number of items returned when no --limit or
// --all flag is provided. Also the maximum per-page size the API accepts.
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
		path += "&cursor=" + url.QueryEscape(cursor)
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

// ListPaginated fetches a single page of items starting from the given cursor.
// Used when --cursor is supplied for explicit page-by-page navigation.
func ListPaginated(resource *Resource, pageSize int, cursor string) (PaginatedResult, error) {
	c := GetClient()
	if c == nil {
		return PaginatedResult{}, fmt.Errorf("client not initialized")
	}
	if !resource.Paginated || resource.APIPath == "" {
		return PaginatedResult{}, fmt.Errorf("resource %s does not support pagination", resource.Kind)
	}
	if pageSize <= 0 || pageSize > DefaultPageLimit {
		pageSize = DefaultPageLimit
	}
	return fetchPage(c, resource.APIPath, pageSize, cursor)
}

// ListWithLimit fetches up to maxItems items, auto-paginating in pages of up to
// 200. A progress indicator is shown on stderr when the output is a terminal
// and more than one page is needed. The returned PaginatedResult.Meta reflects
// the last page fetched (so HasMore/NextCursor tell the caller whether there
// are items beyond the requested limit).
func ListWithLimit(resource *Resource, maxItems int) (PaginatedResult, error) {
	c := GetClient()
	if c == nil {
		return PaginatedResult{}, fmt.Errorf("client not initialized")
	}
	if !resource.Paginated || resource.APIPath == "" {
		return PaginatedResult{}, fmt.Errorf("resource %s does not support pagination", resource.Kind)
	}

	isTTY := term.IsTerminal(int(os.Stderr.Fd()))
	var all []any
	cursor := ""
	var lastMeta PaginationMeta

	for {
		remaining := maxItems - len(all)
		pageSize := DefaultPageLimit
		if remaining < pageSize {
			pageSize = remaining
		}

		result, err := fetchPage(c, resource.APIPath, pageSize, cursor)
		if err != nil {
			return PaginatedResult{}, err
		}
		all = append(all, result.Items...)
		lastMeta = result.Meta

		if isTTY && maxItems > DefaultPageLimit {
			if lastMeta.Total > 0 {
				fmt.Fprintf(os.Stderr, "\rFetching %s... %d/%d", resource.Plural, len(all), lastMeta.Total)
			} else {
				fmt.Fprintf(os.Stderr, "\rFetching %s... %d", resource.Plural, len(all))
			}
		}

		if len(all) >= maxItems {
			break
		}
		if !result.Meta.HasMore || result.Meta.NextCursor == "" {
			break
		}
		cursor = result.Meta.NextCursor
	}

	if isTTY && maxItems > DefaultPageLimit {
		fmt.Fprintf(os.Stderr, "\r\033[K")
	}

	return PaginatedResult{Items: all, Meta: lastMeta}, nil
}

// ListAllPaginated fetches every page from a paginated listing endpoint,
// showing a progress indicator on stderr when the output is a terminal.
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

	if isTTY {
		fmt.Fprintf(os.Stderr, "\r\033[K")
	}

	return all, nil
}

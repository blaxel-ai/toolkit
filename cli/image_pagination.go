package cli

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
)

const imageAPIVersion = "2026-09-22"
const imagePageLimit = 100

type imagePage struct {
	Data []any               `json:"data"`
	Meta core.PaginationMeta `json:"meta"`
}

type imageListOptions struct {
	limit                       int
	cursor, sort, query, source string
	all, latest                 bool
}

func imagePath(kind, name string) string {
	return "images/" + url.PathEscape(kind) + "/" + url.PathEscape(name)
}

// Use the SDK HTTP client, like core pagination, to retain the CLI's generic
// rendering data while reusing authentication, retries and connections.
func fetchImagePage(ctx context.Context, client *blaxel.Client, path string, query url.Values) (imagePage, error) {
	page := imagePage{Data: []any{}}
	if client == nil {
		return page, fmt.Errorf("client not initialized")
	}
	err := client.Get(ctx, path+"?"+query.Encode(), nil, &page,
		option.WithHeader("Blaxel-Version", imageAPIVersion), option.WithHeader("Accept", "application/json"))
	if err != nil {
		return page, err
	}
	if page.Meta.HasMore && page.Meta.NextCursor == "" {
		return page, fmt.Errorf("image pagination returned hasMore without a cursor")
	}
	return page, nil
}

func imageQuery(opts imageListOptions) url.Values {
	q := url.Values{"limit": {strconv.Itoa(opts.limit)}}
	if opts.cursor != "" {
		q.Set("cursor", opts.cursor)
	}
	if opts.sort != "" {
		q.Set("sort", opts.sort)
	}
	if opts.query != "" {
		q.Set("q", opts.query)
	}
	if opts.source != "" {
		q.Set("sourceWorkspace", opts.source)
	}
	return q
}

// walkImagePages follows cursors even through empty filtered pages. Keeping the
// visitor separate lets --latest inspect all tags without storing their collection.
func walkImagePages(ctx context.Context, client *blaxel.Client, path string, query url.Values, all bool, visit func([]any) error) (core.PaginationMeta, error) {
	seen := map[string]bool{query.Get("cursor"): true}
	for {
		page, err := fetchImagePage(ctx, client, path, query)
		if err != nil {
			return core.PaginationMeta{}, err
		}
		if err := visit(page.Data); err != nil {
			return core.PaginationMeta{}, err
		}
		if !all || !page.Meta.HasMore {
			return page.Meta, nil
		}
		if seen[page.Meta.NextCursor] {
			return core.PaginationMeta{}, fmt.Errorf("image pagination returned a repeated cursor")
		}
		seen[page.Meta.NextCursor] = true
		query.Set("cursor", page.Meta.NextCursor)
	}
}

func collectImagePages(ctx context.Context, client *blaxel.Client, path string, query url.Values, all bool) (imagePage, error) {
	result := imagePage{Data: []any{}}
	meta, err := walkImagePages(ctx, client, path, query, all, func(items []any) error {
		result.Data = append(result.Data, items...)
		return nil
	})
	result.Meta = meta
	return result, err
}

func fetchImageSummary(ctx context.Context, client *blaxel.Client, kind, name, source string) (map[string]any, error) {
	if client == nil {
		return nil, fmt.Errorf("client not initialized")
	}
	path := imagePath(kind, name)
	if source != "" {
		path += "?" + url.Values{"sourceWorkspace": {source}}.Encode()
	}
	var image map[string]any
	err := client.Get(ctx, path, nil, &image, option.WithHeader("Blaxel-Version", imageAPIVersion), option.WithHeader("Accept", "application/json"))
	return image, err
}

func latestImageTag(ctx context.Context, client *blaxel.Client, kind, name, source string) (string, error) {
	var latest string
	var newest time.Time
	query := imageQuery(imageListOptions{limit: imagePageLimit, sort: "name:asc", source: source})
	_, err := walkImagePages(ctx, client, imagePath(kind, name)+"/tags", query, true, func(items []any) error {
		for _, item := range items {
			tag, ok := item.(map[string]any)
			if !ok {
				return fmt.Errorf("invalid image tag response")
			}
			tagName, _ := tag["name"].(string)
			stamp, _ := tag["createdAt"].(string)
			created, err := time.Parse(time.RFC3339Nano, stamp)
			if err != nil || tagName == "" {
				return fmt.Errorf("image tag is missing a valid name or creation timestamp")
			}
			if latest == "" || created.After(newest) || (created.Equal(newest) && tagName > latest) {
				latest, newest = tagName, created
			}
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if latest == "" {
		return "", core.MarkExpectedError(fmt.Errorf("no tags found for image %s/%s", kind, name), core.CLIErrorNotFound)
	}
	return latest, nil
}

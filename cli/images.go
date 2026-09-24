package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

// Note: Image commands are integrated directly into get.go and delete.go
// No init() registration needed here

// parseImageRef parses image references in the format:
// - "resourceType/imageName" (e.g., "agent/my-image")
// - "resourceType/imageName:tag" (e.g., "agent/my-image:v1.0")
func parseImageRef(ref string) (resourceType, imageName, tag string, err error) {
	// Check if there's a tag
	parts := strings.SplitN(ref, ":", 2)
	imageRef := parts[0]
	if len(parts) == 2 {
		tag = parts[1]
	}

	// Split resourceType/imageName
	imageParts := strings.SplitN(imageRef, "/", 2)
	if len(imageParts) != 2 {
		return "", "", "", core.MarkExpectedError(
			fmt.Errorf("invalid image reference format. Expected 'resourceType/imageName' or 'resourceType/imageName:tag', got '%s'", ref),
			core.CLIErrorValidation,
		)
	}

	resourceType = imageParts[0]
	imageName = imageParts[1]

	return resourceType, imageName, tag, nil
}

func GetImagesCmd() *cobra.Command {
	opts := imageListOptions{}
	cmd := &cobra.Command{
		Use: "image [resourceType/imageName[:tag]]", Aliases: []string{"images", "img"},
		Short:             "List image summaries or image tags with cursor pagination",
		ValidArgsFunction: GetImageValidArgsFunction(), Args: cobra.MaximumNArgs(1),
		Long: `List one page of image repository summaries, or one page of tags for a named image.
Use --cursor to continue a listing and --all to fetch every page. Empty pages
may still have a next cursor. Repository summaries retain total size, tag count,
status and last deployment time without downloading their tags.

Search uses a case-sensitive name prefix. Search results are ordered by name
ascending. Tag pages support name:asc and name:desc only.

--latest inspects every tag page and prints the most recently created tag reference.`,
		Example: `  bl get images --limit 100
  bl get images --q base --sort name:asc
  bl get image sandbox/base --limit 20
  bl get image sandbox/base --cursor CURSOR
  bl get image sandbox/base:v1
  bl get image sandbox/base --all
  bl get image sandbox/base --latest`,
		RunE: func(cmd *cobra.Command, args []string) error { return runGetImages(cmd, args, opts) },
	}
	cmd.Flags().IntVar(&opts.limit, "limit", imagePageLimit, "Maximum items per page (1-100)")
	cmd.Flags().StringVar(&opts.cursor, "cursor", "", "Cursor from the previous page; keep the same search and sort")
	cmd.Flags().BoolVar(&opts.all, "all", false, "Fetch all pages instead of a single page")
	cmd.Flags().StringVar(&opts.sort, "sort", "", "Sort: name:asc, name:desc, createdAt:asc or createdAt:desc (tags: name only)")
	cmd.Flags().StringVar(&opts.query, "q", "", "Filter by a case-sensitive image or tag name prefix")
	cmd.Flags().StringVar(&opts.source, "source-workspace", "", "Owner workspace of a shared image (named images only)")
	cmd.Flags().BoolVar(&opts.latest, "latest", false, "Return the most recent tag by creation time (reads all tag pages)")
	return cmd
}

func runGetImages(cmd *cobra.Command, args []string, opts imageListOptions) error {
	if opts.limit < 1 || opts.limit > imagePageLimit {
		return fmt.Errorf("--limit must be between 1 and 100")
	}
	if opts.latest && (len(args) != 1 || opts.cursor != "" || opts.query != "" || opts.sort != "" || opts.all) {
		return fmt.Errorf("--latest requires one image and cannot be combined with --cursor, --q, --sort or --all")
	}
	if len(args) == 0 && opts.source != "" {
		return fmt.Errorf("--source-workspace requires a named image")
	}
	ctx := cmd.Context()
	client := core.GetClient()
	query := imageQuery(opts)
	if len(args) == 0 {
		page, err := collectImagePages(ctx, client, "images", query, opts.all)
		if err != nil {
			return err
		}
		core.OutputPreservingOrder(*getImageResource(), page.Data, core.GetOutputFormat())
		printImageCursor(cmd, page.Meta)
		return nil
	}
	kind, name, tag, err := parseImageRef(args[0])
	if err != nil {
		return err
	}
	if opts.latest {
		if tag != "" {
			return fmt.Errorf("--latest cannot be combined with an explicit tag")
		}
		latest, err := latestImageTag(ctx, client, kind, name, opts.source)
		if err != nil {
			return err
		}
		cmd.Printf("%s/%s:%s\n", kind, name, latest)
		return nil
	}
	if tag != "" {
		if opts.query != "" || opts.cursor != "" {
			return fmt.Errorf("an explicit tag cannot be combined with --q or --cursor")
		}
		query.Set("name", tag)
	}
	summary, err := fetchImageSummary(ctx, client, kind, name, opts.source)
	if err != nil {
		return err
	}
	page, err := collectImagePages(ctx, client, imagePath(kind, name)+"/tags", query, opts.all || tag != "")
	if err != nil {
		return err
	}
	if tag != "" && len(page.Data) == 0 {
		return core.MarkExpectedError(fmt.Errorf("tag %q not found for image %s/%s", tag, kind, name), core.CLIErrorNotFound)
	}
	spec, ok := summary["spec"].(map[string]any)
	if !ok {
		return fmt.Errorf("image summary is missing spec")
	}
	spec["tags"] = page.Data
	format := core.GetOutputFormat()
	if format == "table" || format == "" {
		displayImageWithTags(cmd.OutOrStdout(), summary, kind, name)
	} else {
		core.Output(*getImageResource(), []any{summary}, format)
	}
	printImageCursor(cmd, page.Meta)
	return nil
}

func printImageCursor(cmd *cobra.Command, meta core.PaginationMeta) {
	if meta.HasMore {
		cmd.PrintErrf("More results available. Continue with --cursor %q and the same filters.\n", meta.NextCursor)
	}
}

// displayImageWithTags shows an image and its tags in a table format
func displayImageWithTags(out io.Writer, image map[string]interface{}, resourceType, imageName string) {
	// Extract image metadata
	workspace := "-"
	lastDeployedAt := "-"
	totalSize := "-"
	tagCount := "-"

	if metadata, ok := image["metadata"].(map[string]interface{}); ok {
		if ws, ok := metadata["workspace"].(string); ok {
			workspace = ws
		}
		if lda, ok := metadata["lastDeployedAt"].(string); ok && len(lda) >= 10 {
			lastDeployedAt = lda[:10]
		}
	}

	if spec, ok := image["spec"].(map[string]interface{}); ok {
		if count, ok := spec["tagCount"]; ok && count != nil {
			tagCount = fmt.Sprint(count)
		}
		if size, ok := spec["size"]; ok {
			switch v := size.(type) {
			case float64:
				totalSize = formatBytes(int64(v))
			case int64:
				totalSize = formatBytes(v)
			case int:
				totalSize = formatBytes(int64(v))
			}
		}
	}

	_, _ = fmt.Fprintf(out, "Image: %s/%s\n", resourceType, imageName)
	_, _ = fmt.Fprintf(out, "Workspace: %s | Total Size: %s | Tags: %s | Last Deployed: %s\n\n", workspace, totalSize, tagCount, lastDeployedAt)

	// Extract tags from the image
	var tags []interface{}
	if spec, ok := image["spec"].(map[string]interface{}); ok {
		if tagsList, ok := spec["tags"].([]interface{}); ok {
			tags = tagsList
		}
	}

	if len(tags) == 0 {
		_, _ = fmt.Fprintln(out, "No tags found for this image.")
		return
	}

	// First pass: calculate the maximum width needed for the NAME column
	minNameWidth := 4 // minimum width for "NAME" header
	maxNameWidth := 0
	type tagRow struct {
		fullName  string
		size      string
		createdAt string
	}
	var rows []tagRow

	for _, tagInterface := range tags {
		if tagMap, ok := tagInterface.(map[string]interface{}); ok {
			// Display as resourceType/imageName:tag
			fullName := fmt.Sprintf("%s/%s", resourceType, imageName)
			if nameVal, ok := tagMap["name"].(string); ok {
				fullName = fmt.Sprintf("%s:%s", fullName, nameVal)
			}

			if len(fullName) > maxNameWidth {
				maxNameWidth = len(fullName)
			}

			size := "-"
			if sizeVal, ok := tagMap["size"]; ok {
				switch v := sizeVal.(type) {
				case float64:
					size = formatBytes(int64(v))
				case int64:
					size = formatBytes(v)
				case int:
					size = formatBytes(int64(v))
				}
			}

			createdAt := "-"
			if createdAtVal, ok := tagMap["createdAt"].(string); ok {
				// Extract just the date part (YYYY-MM-DD)
				if len(createdAtVal) >= 10 {
					createdAt = createdAtVal[:10]
				}
			}

			rows = append(rows, tagRow{
				fullName:  fullName,
				size:      size,
				createdAt: createdAt,
			})
		}
	}

	// Use the larger of minNameWidth or maxNameWidth
	nameWidth := maxNameWidth
	if nameWidth < minNameWidth {
		nameWidth = minNameWidth
	}

	// Create dynamic table format
	_, _ = fmt.Fprintln(out, "Tags:")
	separatorFormat := fmt.Sprintf("+-%s-+------------+------------+", strings.Repeat("-", nameWidth))
	headerFormat := fmt.Sprintf("| %-*s | SIZE       | CREATED_AT |", nameWidth, "NAME")
	rowFormat := fmt.Sprintf("| %%-%ds | %%-10s | %%-10s |", nameWidth)

	_, _ = fmt.Fprintln(out, separatorFormat)
	_, _ = fmt.Fprintln(out, headerFormat)
	_, _ = fmt.Fprintln(out, separatorFormat)

	for _, row := range rows {
		_, _ = fmt.Fprintf(out, rowFormat+"\n", row.fullName, row.size, row.createdAt)
	}
	_, _ = fmt.Fprintln(out, separatorFormat)
}

// formatBytes formats bytes to human-readable format
func formatBytes(bytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case bytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(bytes)/float64(TB))
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", bytes)
	}
}

func DeleteImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:               "image [resourceType/]imageName[:tag] ...",
		Aliases:           []string{"images", "img"},
		Short:             "Delete images or image tags",
		ValidArgsFunction: GetImageValidArgsFunction(),
		Long: `Delete container images or specific tags.

Usage patterns:
  bl delete image agent/my-image          Delete image with all its tags
  bl delete image agent/my-image:v1.0     Delete only the specified tag

The image reference format is: resourceType/imageName[:tag]
- resourceType: Type of resource (e.g., agent, function, job)
- imageName: The name of the image
- tag: Optional tag to delete only that specific version

WARNING: Deleting an image without specifying a tag will remove ALL tags.`,
		Example: `  # Delete an entire image (all tags)
  bl delete image agent/my-agent

  # Delete only a specific tag
  bl delete image agent/my-agent:v1.0

  # Delete multiple images/tags
  bl delete image agent/img1:v1 agent/img2:v2`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				err := core.MarkExpectedError(
					fmt.Errorf("no image reference provided\nUsage: bl delete image resourceType/imageName[:tag]"),
					core.CLIErrorUsage,
				)
				fmt.Println(err)
				core.ExitWithError(err)
			}

			hasFailures := false
			allFailuresExpected := true
			for _, arg := range args {
				// Parse the image reference
				resourceType, imageName, tag, err := parseImageRef(arg)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					hasFailures = true
					if !core.IsExpectedCLIError(err) {
						allFailuresExpected = false
					}
					continue
				}

				if err := deleteImage(resourceType, imageName, tag); err != nil {
					hasFailures = true
					if !core.IsExpectedCLIError(err) {
						allFailuresExpected = false
					}
				}
			}

			if hasFailures {
				err := fmt.Errorf("one or more image deletions failed")
				if allFailuresExpected {
					err = core.MarkExpectedError(err, core.CLIErrorOperational)
				}
				core.ExitWithError(err)
			}
		},
	}
	return cmd
}

func deleteImage(resourceType, imageName, tag string) error {
	ctx := context.Background()
	client := core.GetClient()

	var identifier string
	var err error

	// For tag deletion, include the tag in the image name
	imageRef := imageName
	if tag != "" {
		imageRef = imageName + ":" + tag
		identifier = fmt.Sprintf("%s/%s:%s", resourceType, imageName, tag)
	} else {
		identifier = fmt.Sprintf("%s/%s", resourceType, imageName)
	}

	_, err = client.Images.Delete(ctx, imageRef, blaxel.ImageDeleteParams{ResourceType: resourceType})

	if err != nil {
		fmt.Printf("Error deleting image %s: %v\n", identifier, err)
		return err
	}

	if tag != "" {
		fmt.Printf("Image tag %s deleted\n", identifier)
	} else {
		fmt.Printf("Image %s deleted (all tags)\n", identifier)
	}
	return nil
}

// ShareImagesCmd returns the cobra command for sharing images across workspaces
func ShareImagesCmd() *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:     "image resourceType/imageName",
		Aliases: []string{"images", "img"},
		Short:   "Share an image with another workspace",
		Long: `Share a container image with another workspace in your account.
Only the metadata is copied — the image data stays in the source workspace.

The image reference format is: resourceType/imageName
- resourceType: Type of resource (e.g., agent, function, job, sandbox)
- imageName: The name of the image`,
		Example: `  # Share an image with another workspace
  bl share image agent/my-agent --workspace other-workspace`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if workspace == "" {
				err := core.MarkExpectedError(
					fmt.Errorf("--workspace flag is required"),
					core.CLIErrorValidation,
				)
				fmt.Println(err)
				core.ExitWithError(err)
			}

			resourceType, imageName, tag, err := parseImageRef(args[0])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				core.ExitWithError(err)
			}

			if tag != "" {
				err := core.MarkExpectedError(
					fmt.Errorf("sharing a specific tag is not supported, remove ':%s' from the reference", tag),
					core.CLIErrorValidation,
				)
				fmt.Println(err)
				core.ExitWithError(err)
			}

			ctx := context.Background()
			client := core.GetClient()

			body := map[string]string{"targetWorkspace": workspace}
			path := fmt.Sprintf("images/%s/%s/share", resourceType, imageName)
			err = client.Post(ctx, path, body, nil)
			if err != nil {
				err = fmt.Errorf("error sharing image %s/%s: %w", resourceType, imageName, err)
				fmt.Println(err)
				core.ExitWithError(err)
			}

			fmt.Printf("Image %s/%s shared with workspace %s\n", resourceType, imageName, workspace)
		},
	}
	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Target workspace to share with (required)")
	_ = cmd.MarkFlagRequired("workspace")
	return cmd
}

// UnshareImagesCmd returns the cobra command for unsharing images from workspaces
func UnshareImagesCmd() *cobra.Command {
	var workspace string
	cmd := &cobra.Command{
		Use:     "image resourceType/imageName",
		Aliases: []string{"images", "img"},
		Short:   "Unshare an image from another workspace",
		Long: `Remove a shared image from another workspace.
This removes the metadata copy from the target workspace.
The original image in the source workspace is not affected.

The image reference format is: resourceType/imageName
- resourceType: Type of resource (e.g., agent, function, job, sandbox)
- imageName: The name of the image`,
		Example: `  # Unshare an image from another workspace
  bl unshare image agent/my-agent --workspace other-workspace`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			if workspace == "" {
				err := core.MarkExpectedError(
					fmt.Errorf("--workspace flag is required"),
					core.CLIErrorValidation,
				)
				fmt.Println(err)
				core.ExitWithError(err)
			}

			resourceType, imageName, tag, err := parseImageRef(args[0])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				core.ExitWithError(err)
			}

			if tag != "" {
				err := core.MarkExpectedError(
					fmt.Errorf("unsharing a specific tag is not supported, remove ':%s' from the reference", tag),
					core.CLIErrorValidation,
				)
				fmt.Println(err)
				core.ExitWithError(err)
			}

			ctx := context.Background()
			client := core.GetClient()

			path := fmt.Sprintf("images/%s/%s/share/%s", resourceType, imageName, workspace)
			err = client.Delete(ctx, path, nil, nil)
			if err != nil {
				err = fmt.Errorf("error unsharing image %s/%s: %w", resourceType, imageName, err)
				fmt.Println(err)
				core.ExitWithError(err)
			}

			fmt.Printf("Image %s/%s unshared from workspace %s\n", resourceType, imageName, workspace)
		},
	}
	cmd.Flags().StringVarP(&workspace, "workspace", "w", "", "Target workspace to unshare from (required)")
	_ = cmd.MarkFlagRequired("workspace")
	return cmd
}

// getImageResource returns the Image resource definition
func getImageResource() *core.Resource {
	resources := core.GetResources()
	for _, r := range resources {
		if r.Kind == "Image" {
			return r
		}
	}
	return nil
}

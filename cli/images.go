package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

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
		return "", "", "", fmt.Errorf("invalid image reference format. Expected 'resourceType/imageName' or 'resourceType/imageName:tag', got '%s'", ref)
	}

	resourceType = imageParts[0]
	imageName = imageParts[1]

	return resourceType, imageName, tag, nil
}

func GetImagesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "image [resourceType/imageName[:tag]]",
		Aliases: []string{"images", "img"},
		Short:   "Get image information",
		Long: `Get information about container images.

Usage patterns:
  bl get images                          List all images (without tags)
  bl get image agent/my-image            Get image details with all tags
  bl get image agent/my-image:v1.0       Get specific tag information

The image reference format is: resourceType/imageName[:tag]
- resourceType: The type of resource (e.g., agent, function, job)
- imageName: The name of the image
- tag: Optional tag to filter for a specific version`,
		Example: `  # List all images
  bl get images

  # Get all tags for a specific image
  bl get image agent/my-agent

  # Get a specific tag
  bl get image agent/my-agent:latest

  # Use different output formats
  bl get images -o json
  bl get image agent/my-agent -o pretty`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) == 0 {
				// List all images
				ListAllImages()
				return
			}

			if len(args) != 1 {
				err := fmt.Errorf("expected zero or one argument\nUsage: bl get image [resourceType/imageName[:tag]]")
				fmt.Println(err)
				core.ExitWithError(err)
			}

			// Parse the image reference
			resourceType, imageName, tag, err := parseImageRef(args[0])
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				core.ExitWithError(err)
			}

			getImage(resourceType, imageName, tag)
		},
	}
	return cmd
}

// ListAllImages lists all images without their tags
func ListAllImages() {
	ctx := context.Background()
	client := core.GetClient()

	response, err := client.ListImages(ctx)
	if err != nil {
		err = fmt.Errorf("error listing images: %v", err)
		fmt.Println(err)
		core.ExitWithError(err)
	}
	defer func() { _ = response.Body.Close() }()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		err = fmt.Errorf("error reading response: %v", err)
		fmt.Println(err)
		core.ExitWithError(err)
	}

	if response.StatusCode == 404 {
		// No images found - return empty list
		resource := getImageResource()
		core.Output(*resource, []interface{}{}, core.GetOutputFormat())
		return
	}

	if response.StatusCode >= 400 {
		err := fmt.Errorf("error listing images: %s", buf.String())
		fmt.Println(err)
		core.ExitWithError(err)
	}

	// Parse the response
	var images []interface{}
	if err := json.Unmarshal(buf.Bytes(), &images); err != nil {
		err = fmt.Errorf("error parsing response: %v", err)
		fmt.Println(err)
		core.ExitWithError(err)
	}

	// Remove tags from each image for the list view
	for i, img := range images {
		if imgMap, ok := img.(map[string]interface{}); ok {
			if spec, ok := imgMap["spec"].(map[string]interface{}); ok {
				// Remove the tags field from spec
				delete(spec, "tags")
			}
			images[i] = imgMap
		}
	}

	// Get the image resource for output formatting
	resource := getImageResource()
	core.Output(*resource, images, core.GetOutputFormat())
}

func getImage(resourceType, imageName, tag string) {
	ctx := context.Background()
	client := core.GetClient()

	response, err := client.GetImage(ctx, resourceType, imageName)
	if err != nil {
		err = fmt.Errorf("error getting image %s/%s: %v", resourceType, imageName, err)
		fmt.Println(err)
		core.ExitWithError(err)
	}
	defer func() { _ = response.Body.Close() }()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		err = fmt.Errorf("error reading response: %v", err)
		fmt.Println(err)
		core.ExitWithError(err)
	}

	if response.StatusCode >= 400 {
		err := fmt.Errorf("error getting image %s/%s: %s", resourceType, imageName, buf.String())
		fmt.Println(err)
		core.ExitWithError(err)
	}

	// Parse the response
	var image map[string]interface{}
	if err := json.Unmarshal(buf.Bytes(), &image); err != nil {
		err = fmt.Errorf("error parsing response: %v", err)
		fmt.Println(err)
		core.ExitWithError(err)
	}

	// If a specific tag is requested, filter the tags
	if tag != "" {
		if spec, ok := image["spec"].(map[string]interface{}); ok {
			if tags, ok := spec["tags"].([]interface{}); ok {
				var filteredTags []interface{}
				for _, t := range tags {
					if tagMap, ok := t.(map[string]interface{}); ok {
						if tagName, ok := tagMap["name"].(string); ok && tagName == tag {
							filteredTags = append(filteredTags, t)
						}
					}
				}
				spec["tags"] = filteredTags

				if len(filteredTags) == 0 {
					err := fmt.Errorf("tag '%s' not found for image %s/%s", tag, resourceType, imageName)
					fmt.Println(err)
					core.ExitWithError(err)
				}
			}
		}
	}

	// Check output format - if table, display tags in a table
	outputFormat := core.GetOutputFormat()
	if outputFormat == "table" || outputFormat == "" {
		displayImageWithTags(image, resourceType, imageName)
	} else {
		// For other formats (json, yaml, pretty), use standard output
		resource := getImageResource()
		core.Output(*resource, []interface{}{image}, outputFormat)
	}
}

// displayImageWithTags shows an image and its tags in a table format
func displayImageWithTags(image map[string]interface{}, resourceType, imageName string) {
	// Extract image metadata
	workspace := "-"
	lastDeployedAt := "-"
	totalSize := "-"

	if metadata, ok := image["metadata"].(map[string]interface{}); ok {
		if ws, ok := metadata["workspace"].(string); ok {
			workspace = ws
		}
		if lda, ok := metadata["lastDeployedAt"].(string); ok && len(lda) >= 10 {
			lastDeployedAt = lda[:10]
		}
	}

	if spec, ok := image["spec"].(map[string]interface{}); ok {
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

	fmt.Printf("Image: %s/%s\n", resourceType, imageName)
	fmt.Printf("Workspace: %s | Total Size: %s | Last Deployed: %s\n\n", workspace, totalSize, lastDeployedAt)

	// Extract tags from the image
	var tags []interface{}
	if spec, ok := image["spec"].(map[string]interface{}); ok {
		if tagsList, ok := spec["tags"].([]interface{}); ok {
			tags = tagsList
		}
	}

	if len(tags) == 0 {
		fmt.Println("No tags found for this image.")
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
	fmt.Println("Tags:")
	separatorFormat := fmt.Sprintf("+-%s-+------------+------------+", strings.Repeat("-", nameWidth))
	headerFormat := fmt.Sprintf("| %-*s | SIZE       | CREATED_AT |", nameWidth, "NAME")
	rowFormat := fmt.Sprintf("| %%-%ds | %%-10s | %%-10s |", nameWidth)

	fmt.Println(separatorFormat)
	fmt.Println(headerFormat)
	fmt.Println(separatorFormat)

	for _, row := range rows {
		fmt.Printf(rowFormat+"\n", row.fullName, row.size, row.createdAt)
	}
	fmt.Println(separatorFormat)
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
		Use:     "image resourceType/imageName[:tag] [resourceType/imageName[:tag]...]",
		Aliases: []string{"images", "img"},
		Short:   "Delete images or image tags",
		Long: `Delete container images or specific tags.

Usage patterns:
  bl delete image agent/my-image          Delete image with all its tags
  bl delete image agent/my-image:v1.0     Delete only the specified tag

The image reference format is: resourceType/imageName[:tag]
- resourceType: The type of resource (e.g., agent, function, job)
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
				err := fmt.Errorf("no image reference provided\nUsage: bl delete image resourceType/imageName[:tag]")
				fmt.Println(err)
				core.ExitWithError(err)
			}

			hasFailures := false
			for _, arg := range args {
				// Parse the image reference
				resourceType, imageName, tag, err := parseImageRef(arg)
				if err != nil {
					fmt.Printf("Error: %v\n", err)
					hasFailures = true
					continue
				}

				if err := deleteImage(resourceType, imageName, tag); err != nil {
					hasFailures = true
				}
			}

			if hasFailures {
				core.ExitWithError(fmt.Errorf("one or more image deletions failed"))
			}
		},
	}
	return cmd
}

func deleteImage(resourceType, imageName, tag string) error {
	ctx := context.Background()
	client := core.GetClient()

	var response *http.Response
	var err error
	var identifier string

	if tag != "" {
		// Delete specific tag
		identifier = fmt.Sprintf("%s/%s:%s", resourceType, imageName, tag)
		// fmt.Printf("[DEBUG] Calling DeleteImageTag(ctx, '%s', '%s', '%s')\n", resourceType, imageName, tag)
		response, err = client.DeleteImageTag(ctx, resourceType, imageName, tag)
	} else {
		// Delete entire image (all tags)
		identifier = fmt.Sprintf("%s/%s", resourceType, imageName)
		response, err = client.DeleteImage(ctx, resourceType, imageName)
	}

	if err != nil {
		fmt.Printf("Error deleting image %s: %v\n", identifier, err)
		return err
	}
	defer func() { _ = response.Body.Close() }()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, response.Body); err != nil {
		fmt.Printf("Error reading response: %v\n", err)
		return err
	}

	if response.StatusCode >= 400 {
		fmt.Printf("Error deleting image %s: %s\n", identifier, buf.String())
		return fmt.Errorf("delete failed with status %d", response.StatusCode)
	}

	if tag != "" {
		fmt.Printf("Image tag %s deleted\n", identifier)
	} else {
		fmt.Printf("Image %s deleted (all tags)\n", identifier)
	}
	return nil
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

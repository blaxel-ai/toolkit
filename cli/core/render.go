package core

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/jedib0t/go-pretty/v6/table"
	"golang.org/x/term"
	"gopkg.in/yaml.v2"
)

// getImageColumnWidth calculates the optimal width for the image column based on terminal size
func getImageColumnWidth() int {
	// Get terminal width
	width, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		// Fallback to default if we can't get terminal size
		return 100
	}

	// Calculate space used by other columns (with some padding for separators and margins)
	// WORKSPACE: ~15, NAME: ~20, CREATED_AT: 10, STATUS: ~10
	// Plus separators and padding: ~20 (more conservative)
	usedSpace := 15 + 20 + 10 + 10 + 20 // ~75 chars for other columns (removed UPDATED_AT: 10)

	// Calculate available space for image column
	availableSpace := width - usedSpace

	// Set minimum and maximum bounds (reduced max from 50 to 35)
	minWidth := 15
	maxWidth := 100

	if availableSpace < minWidth {
		return minWidth
	}
	if availableSpace > maxWidth {
		return maxWidth
	}

	return availableSpace
}

func Output(resource Resource, slices []interface{}, outputFormat string) {
	// Sort slices by creation date before rendering
	sortedSlices := sortByCreationDate(slices)

	if outputFormat == "pretty" {
		printYaml(resource, sortedSlices, true)
		return
	}
	if outputFormat == "yaml" {
		printYaml(resource, sortedSlices, false)
		return
	}
	if outputFormat == "json" {
		printJson(resource, sortedSlices)
		return
	}
	printTable(resource, sortedSlices)
}

func retrieveKey(itemMap map[string]interface{}, key string) string {
	// Split the key by dots to handle nested access
	keys := strings.Split(key, ".")

	// Try to navigate through the nested structure
	value := navigateToKey(itemMap, keys)
	if value != nil {
		if str, ok := value.(string); ok {
			return str
		}
	}

	// Fallback: check metadata for the full key or last part of the key
	if metadata, ok := itemMap["metadata"].(map[string]interface{}); ok {
		// Try full key first
		if value, ok := metadata[key]; ok {
			if str, ok := value.(string); ok {
				return str
			}
		}
		// Try just the last part of the key
		lastKey := keys[len(keys)-1]
		if value, ok := metadata[lastKey]; ok {
			if str, ok := value.(string); ok {
				return str
			}
		}
	}

	return "-"
}

// truncateString truncates a string to the specified max length with ellipsis
func truncateString(s string, maxLength int) string {
	if len(s) <= maxLength {
		return s
	}
	if maxLength <= 3 {
		return s[:maxLength]
	}
	return s[:maxLength-3] + "..."
}

// formatVolumeSize formats volume size in MB to human-readable format
// Note: The API returns size in MB
func formatVolumeSize(sizeInMB int) string {
	if sizeInMB >= 1024 {
		sizeInGB := float64(sizeInMB) / 1024.0
		return fmt.Sprintf("%.2f GB", sizeInGB)
	}
	return fmt.Sprintf("%d MB", sizeInMB)
}

// formatBytesSize formats size in bytes to human-readable format
func formatBytesSize(sizeInBytes int64) string {
	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
		TB = 1024 * GB
	)

	switch {
	case sizeInBytes >= TB:
		return fmt.Sprintf("%.2f TB", float64(sizeInBytes)/float64(TB))
	case sizeInBytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(sizeInBytes)/float64(GB))
	case sizeInBytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(sizeInBytes)/float64(MB))
	case sizeInBytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(sizeInBytes)/float64(KB))
	default:
		return fmt.Sprintf("%d B", sizeInBytes)
	}
}

// formatSizeValue formats a size value (from any path) into human-readable format
// Note: The API returns volume sizes in bytes (not MB as originally documented)
func formatSizeValue(value interface{}) string {
	switch v := value.(type) {
	case int:
		return formatBytesSize(int64(v))
	case float64:
		return formatBytesSize(int64(v))
	case *int:
		if v != nil {
			return formatBytesSize(int64(*v))
		}
	case string:
		// Try to parse string as int64
		var size int64
		if _, err := fmt.Sscanf(v, "%d", &size); err == nil {
			return formatBytesSize(size)
		}
		return v
	}
	return "-"
}

// formatImageSizeValue formats image size (in bytes) to human-readable format
func formatImageSizeValue(value interface{}) string {
	switch v := value.(type) {
	case int:
		return formatBytesSize(int64(v))
	case int64:
		return formatBytesSize(v)
	case float64:
		return formatBytesSize(int64(v))
	case string:
		// Try to parse string as int64
		var size int64
		if _, err := fmt.Sscanf(v, "%d", &size); err == nil {
			return formatBytesSize(size)
		}
		return v
	}
	return "-"
}

// navigateToKey recursively navigates through nested maps using the provided keys
func navigateToKey(m map[string]interface{}, keys []string) interface{} {
	if len(keys) == 0 {
		return nil
	}

	if len(keys) == 1 {
		return m[keys[0]]
	}

	if nested, ok := m[keys[0]].(map[string]interface{}); ok {
		return navigateToKey(nested, keys[1:])
	}

	return nil
}

func printTable(resource Resource, slices []interface{}) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	// Build header dynamically from Fields
	header := buildTableHeader(resource)
	t.AppendHeader(header)

	// Calculate dynamic image width once for all rows
	imageWidth := getImageColumnWidth()

	// Add rows to the table
	for _, item := range slices {
		if itemMap, ok := item.(map[string]interface{}); ok {
			row := buildTableRow(resource, itemMap, imageWidth)
			t.AppendRow(row)
		}
	}
	// Render the table - this automatically sizes columns based on content
	t.Render()
}

// buildTableHeader builds the table header dynamically based on Fields
func buildTableHeader(resource Resource) table.Row {
	header := table.Row{}

	// Add fields in their declared order
	for _, field := range resource.Fields {
		header = append(header, field.Key)
	}

	return header
}

// buildTableRow builds a table row dynamically based on Fields
func buildTableRow(resource Resource, itemMap map[string]interface{}, imageWidth int) table.Row {
	row := table.Row{}

	// Add fields in their declared order
	for _, field := range resource.Fields {
		value := retrieveFieldValue(itemMap, field, imageWidth)

		// Special handling for NAME field in Image resources
		if field.Key == "NAME" && resource.Kind == "Image" {
			if metadata, ok := itemMap["metadata"].(map[string]interface{}); ok {
				if resourceType, ok := metadata["resourceType"].(string); ok {
					value = resourceType + "/" + value
				}
			}
		}

		row = append(row, value)
	}

	return row
}

// retrieveFieldValue retrieves and formats a field value based on its type
func retrieveFieldValue(itemMap map[string]interface{}, field Field, imageWidth int) string {
	// Handle special field types first
	switch field.Special {
	case "count":
		// Count elements in an array field
		value := navigateToKey(itemMap, strings.Split(field.Value, "."))
		if value != nil {
			if arr, ok := value.([]interface{}); ok {
				return fmt.Sprintf("%d", len(arr))
			}
		}
		return "0"
	case "date":
		// Format as date only (YYYY-MM-DD)
		rawValue := retrieveKey(itemMap, field.Value)
		return formatDate(rawValue, "2006-01-02")
	case "datetime":
		// Format as full datetime (YYYY-MM-DD HH:MM:SS)
		rawValue := retrieveKey(itemMap, field.Value)
		return formatDate(rawValue, "2006-01-02 15:04:05")
	case "size":
		// Format size in MB (for volumes)
		value := navigateToKey(itemMap, strings.Split(field.Value, "."))
		if value != nil {
			return formatSizeValue(value)
		}
		return "-"
	case "imagesize":
		// Format size in bytes (for images)
		value := navigateToKey(itemMap, strings.Split(field.Value, "."))
		if value != nil {
			return formatImageSizeValue(value)
		}
		return "-"
	case "image":
		// Strip optional "sandbox/" prefix and truncate image name to fit column width
		rawValue := retrieveKey(itemMap, field.Value)
		// Remove "sandbox/" prefix if present
		rawValue = strings.TrimPrefix(rawValue, "sandbox/")
		return truncateString(rawValue, imageWidth)
	}

	// No special formatting, just return the raw value
	rawValue := retrieveKey(itemMap, field.Value)
	return rawValue
}

// formatDate formats a timestamp string using the provided format
func formatDate(timestamp string, format string) string {
	if timestamp == "" || timestamp == "-" {
		return "-"
	}

	// Parse the timestamp
	parsedTime, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		// Try RFC3339 format as fallback
		parsedTime, err = time.Parse(time.RFC3339, timestamp)
		if err != nil {
			return timestamp
		}
	}

	// Convert to local time
	localTime := parsedTime.Local()
	if utc {
		localTime = parsedTime.UTC()
	}

	// Format using the provided format string
	return localTime.Format(format)
}

func printJson(resource Resource, slices []interface{}) {
	formatted := []Result{}
	for _, slice := range slices {
		if sliceMap, ok := slice.(map[string]interface{}); ok {
			status := "-"
			if statusVal, ok := sliceMap["status"]; ok {
				status = statusVal.(string)
			}
			formatted = append(formatted, Result{
				ApiVersion: "blaxel.ai/v1alpha1",
				Kind:       resource.Kind,
				Metadata:   sliceMap["metadata"],
				Spec:       sliceMap["spec"],
				Status:     status,
			})
		}
	}

	jsonData, err := json.MarshalIndent(formatted, "", "  ")
	if err != nil {
		fmt.Println(err)
		ExitWithError(err)
	}
	fmt.Println(string(jsonData))
}

func printYaml(resource Resource, slices []interface{}, pretty bool) {
	yamlData := renderYaml(resource, slices, pretty)
	// Print the YAML with colored keys and values
	if pretty {
		printColoredYAML(yamlData)
	} else {
		fmt.Println(string(yamlData))
	}
}

func renderYaml(resource Resource, slices []interface{}, pretty bool) []byte {
	formatted := []Result{}
	for _, slice := range slices {
		if sliceMap, ok := slice.(map[string]interface{}); ok {
			status := "-"
			if statusVal, ok := sliceMap["status"]; ok {
				status = statusVal.(string)
			}
			formatted = append(formatted, Result{
				ApiVersion: "blaxel.ai/v1alpha1",
				Kind:       resource.Kind,
				Metadata:   sliceMap["metadata"],
				Spec:       sliceMap["spec"],
				Status:     status,
			})
		}
	}
	// Convert each object to YAML and add separators
	var yamlData []byte
	for _, result := range formatted {
		data, err := yaml.Marshal(result)
		if err != nil {
			fmt.Println(err)
			ExitWithError(err)
		}
		yamlData = append(yamlData, []byte("---\n")...)
		yamlData = append(yamlData, data...)
		yamlData = append(yamlData, []byte("\n")...)
	}
	return yamlData
}

func printColoredYAML(yamlData []byte) {
	lines := bytes.Split(yamlData, []byte("\n"))
	keyColor := color.New(color.FgBlue).SprintFunc()
	stringValueColor := color.New(color.FgGreen).SprintFunc()
	numberValueColor := color.New(color.FgYellow).SprintFunc()

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		// Split the line into key and value
		parts := bytes.SplitN(line, []byte(":"), 2)
		if len(parts) < 2 {
			fmt.Println(string(line))
			continue
		}
		key := parts[0]
		value := parts[1]

		// Determine the type of value and color it accordingly
		var coloredValue string
		if bytes.HasPrefix(value, []byte(" ")) {
			value = bytes.TrimSpace(value)
			if len(value) > 0 && (value[0] == '"' || value[0] == '\'') {
				coloredValue = stringValueColor(string(value))
			} else if _, err := fmt.Sscanf(string(value), "%f", new(float64)); err == nil {
				coloredValue = numberValueColor(string(value))
			} else {
				coloredValue = string(value)
			}
		}

		// Print the colored key and value
		fmt.Printf("%s: %s\n", keyColor(string(key)), coloredValue)
	}
}

// sortByCreationDate sorts slices by creation date (newest first)
func sortByCreationDate(slices []interface{}) []interface{} {
	// Create a copy to avoid modifying the original slice
	sortedSlices := make([]interface{}, len(slices))
	copy(sortedSlices, slices)

	sort.Slice(sortedSlices, func(i, j int) bool {
		iMap, iOk := sortedSlices[i].(map[string]interface{})
		jMap, jOk := sortedSlices[j].(map[string]interface{})

		if !iOk || !jOk {
			return false
		}

		iCreatedAt := retrieveKey(iMap, "createdAt")
		jCreatedAt := retrieveKey(jMap, "createdAt")

		// Parse times for comparison
		iTime, iErr := time.Parse(time.RFC3339, iCreatedAt)
		jTime, jErr := time.Parse(time.RFC3339, jCreatedAt)

		// If either time couldn't be parsed, put them at the end
		if iErr != nil && jErr != nil {
			return false
		}
		if iErr != nil {
			return false
		}
		if jErr != nil {
			return true
		}

		// Sort newest first (descending order)
		return iTime.After(jTime)
	})

	return sortedSlices
}

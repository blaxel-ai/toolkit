package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/fatih/color"
	"gopkg.in/yaml.v2"
)

// ColumnWidths holds the calculated maximum width for each column
type ColumnWidths struct {
	workspace int
	name      int
	image     int
	createdAt int
	updatedAt int
	status    int
}

// TableRowArgs holds the format string and values for a table row
type TableRowArgs struct {
	format string
	values []interface{}
}

func output(resource Resource, slices []interface{}, outputFormat string) {
	if outputFormat == "pretty" {
		printYaml(resource, slices, true)
		return
	}
	if outputFormat == "yaml" {
		printYaml(resource, slices, false)
		return
	}
	if outputFormat == "json" {
		printJson(resource, slices)
		return
	}
	printTable(resource, slices)
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

// buildTableRow dynamically builds format string and values based on enabled columns
func buildTableRow(widths ColumnWidths, resource Resource, data map[string]string) TableRowArgs {
	var formatParts []string
	var values []interface{}

	// Always include workspace and name
	formatParts = append(formatParts, "%-*s", "%-*s")
	values = append(values, widths.workspace, data["workspace"], widths.name, data["name"])

	// Add image column if enabled
	if resource.WithImage {
		formatParts = append(formatParts, "%-*s")
		values = append(values, widths.image, data["image"])
	}

	// Always include created_at and updated_at
	formatParts = append(formatParts, "%-*s", "%-*s")
	values = append(values, widths.createdAt, data["createdAt"], widths.updatedAt, data["updatedAt"])

	// Add status column if enabled
	if resource.WithStatus {
		formatParts = append(formatParts, "%-*s")
		values = append(values, widths.status, data["status"])
	}

	// Join format parts with spaces and add newline
	format := strings.Join(formatParts, " ") + "\n"

	return TableRowArgs{
		format: format,
		values: values,
	}
}

// calculateColumnWidths scans through all data to find maximum width needed for each column
func calculateColumnWidths(resource Resource, slices []interface{}) ColumnWidths {
	// Initialize with header lengths as minimum widths
	widths := ColumnWidths{
		workspace: len("WORKSPACE"),
		name:      len("NAME"),
		image:     len("IMAGE"),
		createdAt: len("CREATED_AT"),
		updatedAt: len("UPDATED_AT"),
		status:    len("STATUS"),
	}

	// Scan through all items to find maximum content width
	for _, item := range slices {
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Check workspace width
			workspace := retrieveKey(itemMap, "workspace")
			if len(workspace) > widths.workspace {
				widths.workspace = len(workspace)
			}

			// Check name width
			name := retrieveKey(itemMap, "name")
			if len(name) > widths.name {
				widths.name = len(name)
			}

			// Check image width if needed
			if resource.WithImage {
				image := retrieveKey(itemMap, "spec.runtime.image")
				if len(image) > widths.image {
					widths.image = len(image)
				}
			}

			// Check createdAt width
			createdAt := retrieveDate(itemMap, "createdAt")
			if len(createdAt) > widths.createdAt {
				widths.createdAt = len(createdAt)
			}

			// Check updatedAt width
			updatedAt := retrieveDate(itemMap, "updatedAt")
			if len(updatedAt) > widths.updatedAt {
				widths.updatedAt = len(updatedAt)
			}

			// Check status width if needed
			if resource.WithStatus {
				status := retrieveKey(itemMap, "status")
				if len(status) > widths.status {
					widths.status = len(status)
				}
			}
		}
	}

	// Add a small padding (2 spaces) to each column for better readability
	widths.workspace += 2
	widths.name += 2
	widths.image += 2
	widths.createdAt += 2
	widths.updatedAt += 2
	widths.status += 2

	return widths
}

func printTable(resource Resource, slices []interface{}) {
	// Calculate maximum width for each column
	maxWidths := calculateColumnWidths(resource, slices)

	// Print header
	headerArgs := buildTableRow(maxWidths, resource, map[string]string{
		"workspace": "WORKSPACE",
		"name":      "NAME",
		"image":     "IMAGE",
		"createdAt": "CREATED_AT",
		"updatedAt": "UPDATED_AT",
		"status":    "STATUS",
	})
	fmt.Printf(headerArgs.format, headerArgs.values...)

	// Print each item in the array
	for _, item := range slices {
		if itemMap, ok := item.(map[string]interface{}); ok {
			rowData := map[string]string{
				"workspace": retrieveKey(itemMap, "workspace"),
				"name":      retrieveKey(itemMap, "name"),
				"image":     retrieveKey(itemMap, "spec.runtime.image"),
				"createdAt": retrieveDate(itemMap, "createdAt"),
				"updatedAt": retrieveDate(itemMap, "updatedAt"),
				"status":    retrieveKey(itemMap, "status"),
			}

			rowArgs := buildTableRow(maxWidths, resource, rowData)
			fmt.Printf(rowArgs.format, rowArgs.values...)
		}
	}
}

func retrieveDate(itemMap map[string]interface{}, key string) string {
	value := retrieveKey(itemMap, key)
	if value != "-" {
		// Parse and format the date
		if parsedTime, err := time.Parse(time.RFC3339, value); err == nil {
			// Convert the parsed time to local time
			localTime := parsedTime.Local()
			if utc {
				localTime = parsedTime.UTC()
			}
			// Format the local time with dynamic timezone
			value = localTime.Format("2006-01-02 15:04:05 MST")
		}
	}
	return value
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
		os.Exit(1)
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
			os.Exit(1)
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

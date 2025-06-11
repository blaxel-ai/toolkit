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
		return 25
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

	// Set header based on what columns are enabled
	if resource.WithImage && resource.WithStatus {
		t.AppendHeader(table.Row{"WORKSPACE", "NAME", "STATUS", "IMAGE", "CREATED_AT"})
	} else if resource.WithImage {
		t.AppendHeader(table.Row{"WORKSPACE", "NAME", "IMAGE", "CREATED_AT"})
	} else if resource.WithStatus {
		t.AppendHeader(table.Row{"WORKSPACE", "NAME", "STATUS", "CREATED_AT"})
	} else {
		t.AppendHeader(table.Row{"WORKSPACE", "NAME", "CREATED_AT"})
	}

	// Calculate dynamic image width once for all rows
	imageWidth := getImageColumnWidth()

	// Add rows to the table
	for _, item := range slices {
		if itemMap, ok := item.(map[string]interface{}); ok {
			workspace := retrieveKey(itemMap, "workspace")
			name := retrieveKey(itemMap, "name")
			createdAt := retrieveDate(itemMap, "createdAt")

			if resource.WithImage && resource.WithStatus {
				status := retrieveKey(itemMap, "status")
				image := truncateString(retrieveKey(itemMap, "spec.runtime.image"), imageWidth)
				t.AppendRow(table.Row{workspace, name, status, image, createdAt})
			} else if resource.WithImage {
				image := truncateString(retrieveKey(itemMap, "spec.runtime.image"), imageWidth)
				t.AppendRow(table.Row{workspace, name, image, createdAt})
			} else if resource.WithStatus {
				status := retrieveKey(itemMap, "status")
				t.AppendRow(table.Row{workspace, name, status, createdAt})
			} else {
				t.AppendRow(table.Row{workspace, name, createdAt})
			}
		}
	}
	// Render the table - this automatically sizes columns based on content
	t.Render()
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
			// Format the local time with only date (no time)
			value = localTime.Format("2006-01-02")
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

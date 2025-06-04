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

func printTable(resource Resource, slices []interface{}) {
	// Print header with fixed width columns
	if resource.WithImage && resource.WithStatus {
		fmt.Printf("%-15s %-24s %-40s %-24s %-24s %-20s\n", "WORKSPACE", "NAME", "IMAGE", "CREATED_AT", "UPDATED_AT", "STATUS")
	} else if resource.WithImage {
		fmt.Printf("%-15s %-24s %-40s %-24s %-24s\n", "WORKSPACE", "NAME", "IMAGE", "CREATED_AT", "UPDATED_AT")
	} else if resource.WithStatus {
		fmt.Printf("%-15s %-24s %-24s %-24s %-20s\n", "WORKSPACE", "NAME", "CREATED_AT", "UPDATED_AT", "STATUS")
	} else {
		fmt.Printf("%-15s %-24s %-24s %-24s\n", "WORKSPACE", "NAME", "CREATED_AT", "UPDATED_AT")
	}

	// Print each item in the array
	for _, item := range slices {
		// Convert item to map to access fields
		if itemMap, ok := item.(map[string]interface{}); ok {
			// Get the workspace field, default to "-" if not found
			workspace := retrieveKey(itemMap, "workspace")

			// Get the name field, default to "-" if not found
			name := retrieveKey(itemMap, "name")

			// Get the created_at field, default to "-" if not found
			createdAt := retrieveDate(itemMap, "createdAt")

			// Get the updated_at field, default to "-" if not found
			updatedAt := retrieveDate(itemMap, "updatedAt")

			if resource.WithImage && resource.WithStatus {
				image := retrieveKey(itemMap, "spec.runtime.image")
				status := retrieveKey(itemMap, "status")
				fmt.Printf("%-15s %-24s %-40s %-24s %-24s %-20s\n", workspace, name, image, createdAt, updatedAt, status)
			} else if resource.WithImage {
				image := retrieveKey(itemMap, "spec.runtime.image")
				fmt.Printf("%-15s %-24s %-40s %-24s %-24s\n", workspace, name, image, createdAt, updatedAt)
			} else if resource.WithStatus {
				status := retrieveKey(itemMap, "status")
				fmt.Printf("%-15s %-24s %-24s %-24s %-20s\n", workspace, name, createdAt, updatedAt, status)
			} else {
				fmt.Printf("%-15s %-24s %-24s %-24s\n", workspace, name, createdAt, updatedAt)
			}
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

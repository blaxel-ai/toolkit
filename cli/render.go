package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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
	if value, ok := itemMap[key]; ok {
		return value.(string)
	}
	if value, ok := itemMap["metadata"].(map[string]interface{})[key]; ok {
		return value.(string)
	}
	if value, ok := itemMap["status"].(map[string]interface{})[key]; ok {
		return value.(string)
	}
	return "-"
}

func printTable(resource Resource, slices []interface{}) {
	// Print header with fixed width columns
	if resource.WithStatus {
		fmt.Printf("%-15s %-24s %-20s %-20s %-20s\n", "WORKSPACE", "NAME", "CREATED_AT", "UPDATED_AT", "STATUS")
	} else {
		fmt.Printf("%-15s %-24s %-20s %-20s\n", "WORKSPACE", "NAME", "CREATED_AT", "UPDATED_AT")
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
			createdAt := retrieveKey(itemMap, "createdAt")
			if createdAt != "-" {
				// Parse and format the date
				if parsedTime, err := time.Parse(time.RFC3339, createdAt); err == nil {
					createdAt = parsedTime.Format("2006-01-02 15:04:05")
				}
			}

			// Get the updated_at field, default to "-" if not found
			updatedAt := retrieveKey(itemMap, "updatedAt")
			if updatedAt != "-" {
				// Parse and format the date
				// Parse and format the date
				if parsedTime, err := time.Parse(time.RFC3339, updatedAt); err == nil {
					updatedAt = parsedTime.Format("2006-01-02 15:04:05")
				}
			}

			if resource.WithStatus {
				status := retrieveKey(itemMap, "deploymentStatus")
				fmt.Printf("%-15s %-24s %-20s %-20s %-20s\n", workspace, name, createdAt, updatedAt, status)
			} else {
				fmt.Printf("%-15s %-24s %-20s %-20s\n", workspace, name, createdAt, updatedAt)
			}
		}
	}
}

func printJson(resource Resource, slices []interface{}) {
	formatted := []Result{}
	for _, slice := range slices {
		if sliceMap, ok := slice.(map[string]interface{}); ok {
			formatted = append(formatted, Result{
				ApiVersion: "beamlit.com/v1alpha1",
				Kind:       resource.Kind,
				Metadata:   sliceMap["metadata"],
				Spec:       sliceMap["spec"],
				Status:     sliceMap["status"],
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
	formatted := []Result{}
	for _, slice := range slices {
		if sliceMap, ok := slice.(map[string]interface{}); ok {
			formatted = append(formatted, Result{
				ApiVersion: "beamlit.com/v1alpha1",
				Kind:       resource.Kind,
				Metadata:   sliceMap["metadata"],
				Spec:       sliceMap["spec"],
				Status:     sliceMap["status"],
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

	// Print the YAML with colored keys and values
	if pretty {
		printColoredYAML(yamlData)
	} else {
		fmt.Println(string(yamlData))
	}
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

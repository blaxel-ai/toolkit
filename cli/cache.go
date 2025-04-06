package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
)

func (r *Operations) SeedCache(cwd string) error {
	cacheFile := filepath.Join(cwd, ".cache.yaml")
	// Remove file if it exists
	_ = os.Remove(cacheFile)

	var results string
	for _, resource := range resources {
		switch resource.Kind {
		case "Function":
			if len(config.Functions) == 0 {
				continue
			}
			res, err := resource.ListExec()
			if err == nil {
				results += string(filterCache(*resource, res, config.Functions))
			}
		case "Model":
			if len(config.Functions) == 0 {
				continue
			}
			res, err := resource.ListExec()
			if err == nil {
				results += string(filterCache(*resource, res, config.Models))
			}
		case "Agent":
			if len(config.Agents) == 0 {
				continue
			}
			res, err := resource.ListExec()
			if err == nil {
				results += string(filterCache(*resource, res, config.Agents))
			}
		}
	}

	// Create file
	if results != "" {
		file, err := os.Create(cacheFile)
		if err != nil {
			return fmt.Errorf("failed to create cache file: %w", err)
		}
		_, err = file.WriteString(results)
		if err != nil {
			return fmt.Errorf("failed to write cache file: %w", err)
		}
	}
	return nil
}

type NameRetriever struct {
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
}

func filterCache(resource Resource, res []interface{}, names []string) string {
	if slices.Contains(names, "all") {
		return string(renderYaml(resource, res, false))
	}
	content := ""
	for _, r := range res {
		var nameRetriever NameRetriever
		jsonBytes, err := json.Marshal(r)
		if err != nil {
			continue
		}
		err = json.Unmarshal(jsonBytes, &nameRetriever)
		if err != nil {
			continue
		}
		if slices.Contains(names, nameRetriever.Metadata.Name) {
			content += string(renderYaml(resource, []interface{}{r}, false))
		}
	}
	return content
}

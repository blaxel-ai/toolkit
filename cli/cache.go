package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func (r *Operations) SeedCache(cwd string) error {
	cacheFile := filepath.Join(cwd, ".cache.yaml")
	// Remove file if it exists
	_ = os.Remove(cacheFile)

	var results string
	for _, resource := range resources {
		res, err := resource.ListExec()
		if err == nil {
			results += string(renderYaml(*resource, res, false))
		}
	}

	// Create file
	file, err := os.Create(cacheFile)
	if err != nil {
		return fmt.Errorf("failed to create cache file: %w", err)
	}
	file.WriteString(results)
	return nil
}

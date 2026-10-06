package cli

import "path/filepath"

// resolveDeployConfig validates --config before anything reads it. Unlike the
// default blaxel.toml, an explicit file that is missing is an error, not a
// fallback to defaults.
func resolveDeployConfig(cwd, folder, path string) error {
	if path == "" {
		return deployInputError("--config must not be empty")
	}
	_, err := resolveProjectFile("Config file", filepath.Join(cwd, folder), path)
	return err
}

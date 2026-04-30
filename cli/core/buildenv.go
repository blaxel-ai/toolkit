package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadBuildEnv reads a .build-env file and returns a map of KEY=VALUE pairs.
// If customPath is non-empty, it is used as the file path (error if missing).
// If customPath is empty, it looks for .build-env in projectDir (silently skips if missing).
// Lines starting with # and blank lines are skipped.
// Empty values (KEY=) are valid.
func ReadBuildEnv(projectDir string, customPath string) (map[string]string, error) {
	var filePath string
	var required bool

	if customPath != "" {
		if filepath.IsAbs(customPath) {
			filePath = customPath
		} else {
			filePath = filepath.Join(projectDir, customPath)
		}
		required = true
	} else {
		filePath = filepath.Join(projectDir, ".build-env")
		required = false
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) && !required {
			return nil, nil
		}
		if os.IsNotExist(err) && required {
			return nil, fmt.Errorf("build-env file not found: %s", filePath)
		}
		return nil, fmt.Errorf("failed to read build-env file: %w", err)
	}

	return parseBuildEnv(string(content))
}

// parseBuildEnv parses KEY=VALUE content. Skips blank lines and # comments.
func parseBuildEnv(content string) (map[string]string, error) {
	args := make(map[string]string)
	scanner := bufio.NewScanner(strings.NewReader(content))
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		eqIdx := strings.Index(line, "=")
		if eqIdx < 0 {
			return nil, fmt.Errorf(".build-env line %d: invalid format (expected KEY=VALUE): %s", lineNum, line)
		}

		key := strings.TrimSpace(line[:eqIdx])
		value := strings.TrimSpace(line[eqIdx+1:])

		if key == "" {
			return nil, fmt.Errorf(".build-env line %d: empty key", lineNum)
		}

		args[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse .build-env: %w", err)
	}

	if len(args) == 0 {
		return nil, nil
	}

	return args, nil
}

// MergeBuildEnvContent merges build args from blaxel.toml [build.args] and .build-env file content.
// The .build-env content takes precedence on duplicate keys.
// Returns the merged content as KEY=VALUE lines suitable for injection into the archive.
func MergeBuildEnvContent(tomlArgs map[string]string, envArgs map[string]string) []byte {
	if len(tomlArgs) == 0 && len(envArgs) == 0 {
		return nil
	}

	merged := make(map[string]string)

	// Start with toml args
	for k, v := range tomlArgs {
		merged[k] = v
	}

	// Override with .build-env args
	for k, v := range envArgs {
		merged[k] = v
	}

	if len(merged) == 0 {
		return nil
	}

	// Serialize to KEY=VALUE format
	var lines []string
	for k, v := range merged {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}

	return []byte(strings.Join(lines, "\n") + "\n")
}

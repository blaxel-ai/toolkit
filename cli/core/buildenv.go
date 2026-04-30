package core

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadBuildEnv reads a .env.build file and returns a map of KEY=VALUE pairs.
// If customPath is non-empty, it is used as the file path (error if missing).
// If customPath is empty, it looks for .env.build in projectDir (silently skips if missing).
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
		filePath = filepath.Join(projectDir, ".env.build")
		required = false
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) && !required {
			return nil, nil
		}
		if os.IsNotExist(err) && required {
			return nil, fmt.Errorf(".env.build file not found: %s", filePath)
		}
		return nil, fmt.Errorf("failed to read .env.build file: %w", err)
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
			return nil, fmt.Errorf(".env.build line %d: invalid format (expected KEY=VALUE): %s", lineNum, line)
		}

		key := strings.TrimSpace(line[:eqIdx])
		value := strings.TrimSpace(line[eqIdx+1:])

		if key == "" {
			return nil, fmt.Errorf(".env.build line %d: empty key", lineNum)
		}

		args[key] = value
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("failed to parse .env.build: %w", err)
	}

	if len(args) == 0 {
		return nil, nil
	}

	return args, nil
}

// MergeBuildEnvContent merges build args from blaxel.toml [build.args] and .env.build file content.
// The .env.build content takes precedence on duplicate keys.
// Returns the merged content as KEY=VALUE lines suitable for injection into the archive,
// and the number of unique merged args.
func MergeBuildEnvContent(tomlArgs map[string]string, envArgs map[string]string) ([]byte, int) {
	if len(tomlArgs) == 0 && len(envArgs) == 0 {
		return nil, 0
	}

	merged := make(map[string]string)

	// Start with toml args
	for k, v := range tomlArgs {
		merged[k] = v
	}

	// Override with .env.build args
	for k, v := range envArgs {
		merged[k] = v
	}

	if len(merged) == 0 {
		return nil, 0
	}

	// Serialize to KEY=VALUE format
	var lines []string
	for k, v := range merged {
		lines = append(lines, fmt.Sprintf("%s=%s", k, v))
	}

	return []byte(strings.Join(lines, "\n") + "\n"), len(merged)
}

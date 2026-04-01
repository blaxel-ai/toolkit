package core

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"
)

// DockerConfigAuth represents a single registry auth entry in a Docker config.json.
type DockerConfigAuth struct {
	Auth string `json:"auth"`
}

// DockerConfig represents the Docker config.json structure used for registry authentication.
type DockerConfig struct {
	Auths map[string]DockerConfigAuth `json:"auths"`
}

// ParseRegistryCred parses a registry credential string in the format "registry=username:password".
// Returns the registry URL and the base64-encoded "username:password" auth string.
func ParseRegistryCred(cred string) (string, string, error) {
	registry, userPass, found := strings.Cut(cred, "=")
	if !found {
		return "", "", fmt.Errorf("invalid registry credential format: expected registry=username:password")
	}

	if registry == "" {
		return "", "", fmt.Errorf("invalid registry credential: registry cannot be empty")
	}

	username, password, found := strings.Cut(userPass, ":")
	if !found {
		return "", "", fmt.Errorf("invalid registry credential: expected username:password after '='")
	}

	if username == "" {
		return "", "", fmt.Errorf("invalid registry credential: username cannot be empty")
	}
	if password == "" {
		return "", "", fmt.Errorf("invalid registry credential: password cannot be empty")
	}

	auth := base64.StdEncoding.EncodeToString([]byte(userPass))
	return registry, auth, nil
}

// LoadDockerConfigFile reads and parses a Docker config.json file from the given path.
func LoadDockerConfigFile(path string) (*DockerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read docker config file: %w", err)
	}

	var config DockerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse docker config file: %w", err)
	}

	if config.Auths == nil {
		config.Auths = make(map[string]DockerConfigAuth)
	}

	return &config, nil
}

// BuildDockerConfigFromFlags builds a DockerConfig from a slice of --registry-cred flag values.
func BuildDockerConfigFromFlags(registryCreds []string) (*DockerConfig, error) {
	if len(registryCreds) == 0 {
		return nil, nil
	}

	config := &DockerConfig{
		Auths: make(map[string]DockerConfigAuth),
	}

	for _, cred := range registryCreds {
		registry, auth, err := ParseRegistryCred(cred)
		if err != nil {
			return nil, err
		}
		config.Auths[registry] = DockerConfigAuth{Auth: auth}
	}

	return config, nil
}

// MergeDockerConfigs merges two DockerConfigs. The override entries win when the
// same registry appears in both. Either input can be nil.
func MergeDockerConfigs(base, override *DockerConfig) *DockerConfig {
	if base == nil && override == nil {
		return nil
	}

	result := &DockerConfig{Auths: make(map[string]DockerConfigAuth)}

	if base != nil {
		maps.Copy(result.Auths, base.Auths)
	}
	if override != nil {
		maps.Copy(result.Auths, override.Auths)
	}

	if len(result.Auths) == 0 {
		return nil
	}

	return result
}

// ResolveDockerConfig resolves the final Docker config by merging all sources.
// Priority (highest to lowest):
//  1. --registry-cred flag values
//  2. --docker-config flag file
//  3. .docker/config.json auto-discovered in project directory
//
// Returns the JSON bytes of the merged config, or nil if no config was found.
func ResolveDockerConfig(projectDir string, registryCreds []string, dockerConfigPath string) ([]byte, error) {
	// 1. Auto-discover .docker/config.json in project directory
	var base *DockerConfig
	projectConfigPath := filepath.Join(projectDir, ".docker", "config.json")
	if _, err := os.Stat(projectConfigPath); err == nil {
		base, err = LoadDockerConfigFile(projectConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load project docker config: %w", err)
		}
	}

	// 2. If --docker-config flag is set, load and merge over base
	if dockerConfigPath != "" {
		flagFileConfig, err := LoadDockerConfigFile(dockerConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load docker config from flag: %w", err)
		}
		base = MergeDockerConfigs(base, flagFileConfig)
	}

	// 3. If --registry-cred flags are set, build config and merge over everything
	if len(registryCreds) > 0 {
		flagConfig, err := BuildDockerConfigFromFlags(registryCreds)
		if err != nil {
			return nil, err
		}
		base = MergeDockerConfigs(base, flagConfig)
	}

	if base == nil {
		return nil, nil
	}

	data, err := json.Marshal(base)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal docker config: %w", err)
	}

	return data, nil
}

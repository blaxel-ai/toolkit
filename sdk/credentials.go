package sdk

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v2"
)

func loadConfig() Config {
	config := Config{}
	homeDir, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(homeDir, ".blaxel", "config.yaml")
		if data, err := os.ReadFile(configPath); err == nil {
			if err := yaml.Unmarshal(data, &config); err != nil {
				// Invalid YAML, use empty credentials
				config = Config{}
			}
		}
	}
	return config
}

func saveConfig(config Config) {
	yamlData, err := yaml.Marshal(config)
	if err != nil {
		panic(err)
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting home directory: %v\n", err)
		os.Exit(1)
	}
	configDir := filepath.Join(homeDir, ".blaxel")
	configFile := filepath.Join(configDir, "config.yaml")
	if err := os.WriteFile(configFile, yamlData, 0600); err != nil {
		fmt.Printf("Error writing credentials file: %v\n", err)
		os.Exit(1)
	}
}

func ListWorkspaces() []string {
	config := loadConfig()
	workspaces := []string{}
	for _, workspace := range config.Workspaces {
		workspaces = append(workspaces, workspace.Name)
	}
	return workspaces
}

func CurrentContext() ContextConfig {
	config := loadConfig()
	return config.Context
}

func SetCurrentWorkspace(workspaceName string) {
	config := loadConfig()
	// Check if workspace exists
	found := false
	for _, workspace := range config.Workspaces {
		if workspace.Name == workspaceName {
			found = true
			break
		}
	}
	if !found {
		fmt.Printf("Workspace %s not found\n", workspaceName)
		os.Exit(1)
	}
	config.Context.Workspace = workspaceName
	saveConfig(config)
}

func LoadCredentials(workspaceName string) Credentials {
	config := loadConfig()
	for _, workspace := range config.Workspaces {
		if workspace.Name == workspaceName {
			return workspace.Credentials
		}
	}
	return Credentials{}
}

func LoadEnv(workspaceName string) string {
	config := loadConfig()
	for _, workspace := range config.Workspaces {
		if workspace.Name == workspaceName {
			return workspace.Env
		}
	}
	env := os.Getenv("BL_ENV")
	if env != "" {
		return env
	}
	return "prod"
}

func createHomeDirIfMissing() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Error getting home directory: %v\n", err)
		return
	}

	credentialsDir := filepath.Join(homeDir, ".blaxel")
	credentialsFile := filepath.Join(credentialsDir, "credentials.json")

	// Check if credentials file exists
	if _, err := os.Stat(credentialsFile); err == nil {
		fmt.Println("You are already logged in. Enter a new API key to overwrite it.")
	} else {
		if err := os.MkdirAll(credentialsDir, 0700); err != nil {
			fmt.Printf("Error creating credentials directory: %v\n", err)
			return
		}
	}
}

func SaveCredentials(workspaceName string, credentials Credentials) {
	createHomeDirIfMissing()
	if credentials.AccessToken == "" && credentials.APIKey == "" && credentials.ClientCredentials == "" {
		fmt.Println("No credentials to save, error")
		return
	}
	config := loadConfig()
	found := false
	for i, workspace := range config.Workspaces {
		if workspace.Name == workspaceName {
			config.Workspaces[i].Credentials = credentials
			found = true
			break
		}
	}
	if !found {
		config.Workspaces = append(config.Workspaces, WorkspaceConfig{Name: workspaceName, Credentials: credentials})
	}
	saveConfig(config)
}

func ClearCredentials(workspaceName string) {
	config := loadConfig()
	for i, workspace := range config.Workspaces {
		if workspace.Name == workspaceName {
			config.Workspaces = append(config.Workspaces[:i], config.Workspaces[i+1:]...)
			break
		}
	}
	if config.Context.Workspace == workspaceName {
		config.Context.Workspace = ""
	}
	saveConfig(config)
}

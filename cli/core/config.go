package core

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/charmbracelet/huh"
	"github.com/fatih/color"
)

type Field struct {
	Key     string
	Value   string
	Special string
}

type Resource struct {
	Kind     string
	Short    string
	Plural   string
	Singular string
	Aliases  []string
	SpecType reflect.Type
	List     interface{}
	Get      interface{}
	Delete   interface{}
	Put      interface{}
	Post     interface{}
	Fields   []Field // ordered slice of fields - e.g., {Key: "STATUS", Value: "status"}
}

var resources = []*Resource{
	{
		Kind:     "Policy",
		Short:    "pol",
		Plural:   "policies",
		Singular: "policy",
		SpecType: reflect.TypeOf(sdk.Policy{}),
		Fields: []Field{
			{Key: "WORKSPACE", Value: "workspace"},
			{Key: "NAME", Value: "name"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
		},
	},
	{
		Kind:     "Model",
		Short:    "ml",
		Plural:   "models",
		Singular: "model",
		SpecType: reflect.TypeOf(sdk.Model{}),
		Fields: []Field{
			{Key: "WORKSPACE", Value: "workspace"},
			{Key: "NAME", Value: "name"},
			{Key: "STATUS", Value: "status"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
		},
	},
	{
		Kind:     "Function",
		Short:    "fn",
		Plural:   "functions",
		Singular: "function",
		Aliases:  []string{"mcp", "mcps"},
		SpecType: reflect.TypeOf(sdk.Function{}),
		Fields: []Field{
			{Key: "WORKSPACE", Value: "workspace"},
			{Key: "NAME", Value: "name"},
			{Key: "IMAGE", Value: "spec.runtime.image", Special: "image"},
			{Key: "STATUS", Value: "status"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
		},
	},
	{
		Kind:     "Agent",
		Short:    "ag",
		Plural:   "agents",
		Singular: "agent",
		SpecType: reflect.TypeOf(sdk.Agent{}),
		Fields: []Field{
			{Key: "WORKSPACE", Value: "workspace"},
			{Key: "NAME", Value: "name"},
			{Key: "IMAGE", Value: "spec.runtime.image", Special: "image"},
			{Key: "STATUS", Value: "status"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
		},
	},
	{
		Kind:     "IntegrationConnection",
		Short:    "ic",
		Plural:   "integrationconnections",
		Singular: "integrationconnection",
		SpecType: reflect.TypeOf(sdk.IntegrationConnection{}),
		Fields: []Field{
			{Key: "WORKSPACE", Value: "workspace"},
			{Key: "NAME", Value: "name"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
		},
	},
	{
		Kind:     "Sandbox",
		Short:    "sbx",
		Plural:   "sandboxes",
		Singular: "sandbox",
		SpecType: reflect.TypeOf(sdk.Sandbox{}),
		Fields: []Field{
			{Key: "WORKSPACE", Value: "workspace"},
			{Key: "NAME", Value: "name"},
			{Key: "IMAGE", Value: "spec.runtime.image", Special: "image"},
			{Key: "REGION", Value: "spec.region"},
			{Key: "STATUS", Value: "status"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
		},
	},
	{
		Kind:     "Job",
		Short:    "jb",
		Plural:   "jobs",
		Singular: "job",
		SpecType: reflect.TypeOf(sdk.Job{}),
		Fields: []Field{
			{Key: "WORKSPACE", Value: "workspace"},
			{Key: "NAME", Value: "name"},
			{Key: "REGION", Value: "spec.region"},
			{Key: "STATUS", Value: "status"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
		},
	},
	{
		Kind:     "Volume",
		Short:    "vol",
		Plural:   "volumes",
		Singular: "volume",
		SpecType: reflect.TypeOf(sdk.Volume{}),
		Fields: []Field{
			{Key: "WORKSPACE", Value: "workspace"},
			{Key: "NAME", Value: "name"},
			{Key: "SIZE", Value: "spec.size", Special: "size"},
			{Key: "REGION", Value: "spec.region"},
			{Key: "STATUS", Value: "status"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
		},
	},
	{
		Kind:     "VolumeTemplate",
		Short:    "vt",
		Plural:   "volumetemplates",
		Singular: "volumetemplate",
		SpecType: reflect.TypeOf(sdk.VolumeTemplate{}),
		Fields: []Field{
			{Key: "WORKSPACE", Value: "workspace"},
			{Key: "NAME", Value: "name"},
			{Key: "SIZE", Value: "spec.defaultSize", Special: "size"},
			{Key: "VERSION", Value: "state.latestVersion"},
			{Key: "STATUS", Value: "state.status"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
		},
	},
	{
		Kind:     "Image",
		Short:    "img",
		Plural:   "images",
		Singular: "image",
		SpecType: reflect.TypeOf(sdk.Image{}),
		Fields: []Field{
			{Key: "WORKSPACE", Value: "workspace"},
			{Key: "NAME", Value: "name"},
			{Key: "SIZE", Value: "spec.size", Special: "imagesize"},
			{Key: "LAST_DEPLOYED_AT", Value: "metadata.lastDeployedAt", Special: "date"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
		},
	},
}

type Package struct {
	Path string `toml:"path"`
	Port int    `toml:"port,omitempty"`
	Type string `toml:"type,omitempty"`
}

// readConfigToml reads the config.toml file and upgrade config according to content
type Config struct {
	Name        string                    `toml:"name"`
	Workspace   string                    `toml:"workspace"`
	Type        string                    `toml:"type"`
	Protocol    string                    `toml:"protocol"`
	Functions   []string                  `toml:"functions"`
	Models      []string                  `toml:"models"`
	Agents      []string                  `toml:"agents"`
	Entrypoint  Entrypoints               `toml:"entrypoint"`
	Env         Envs                      `toml:"env"`
	Function    map[string]Package        `toml:"function"`
	Agent       map[string]Package        `toml:"agent"`
	Job         map[string]Package        `toml:"job"`
	SkipRoot    bool                      `toml:"skipRoot"`
	Runtime     *map[string]interface{}   `toml:"runtime"`
	Triggers    *[]map[string]interface{} `toml:"triggers"`
	Volumes     *[]map[string]interface{} `toml:"volumes,omitempty"`
	Policies    []string                  `toml:"policies,omitempty"`
	DefaultSize *int                      `toml:"defaultSize,omitempty"`
	Directory   string                    `toml:"directory,omitempty"`
	Region      string                    `toml:"region,omitempty"`
	Public      *bool                     `toml:"public,omitempty"`
}

// blaxelTomlWarning stores any warning from parsing blaxel.toml
var blaxelTomlWarning string

func readConfigToml(folder string, setDefaultType bool) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}

	content, err := os.ReadFile(filepath.Join(cwd, folder, "blaxel.toml"))
	if err != nil {
		// No blaxel.toml file found
		config.Functions = []string{"all"}
		config.Models = []string{"all"}

		// Set default type only if requested
		if setDefaultType {
			config.Type = "agent"
		}
		return
	}

	err = toml.Unmarshal(content, &config)
	if err != nil {
		// Store the warning for the caller to handle
		blaxelTomlWarning = buildBlaxelTomlWarning(err)
		return
	}

	if config.Type == "" && setDefaultType {
		config.Type = "agent"
	}

	if config.Workspace != "" {
		workspace = config.Workspace
	}
}

// GetBlaxelTomlWarning returns any warning from parsing blaxel.toml
func GetBlaxelTomlWarning() string {
	return blaxelTomlWarning
}

// ClearBlaxelTomlWarning clears the blaxel.toml warning
func ClearBlaxelTomlWarning() {
	blaxelTomlWarning = ""
}

// buildBlaxelTomlWarning detects common blaxel.toml configuration errors and builds
// a warning message with sample configurations
func buildBlaxelTomlWarning(err error) string {
	errStr := err.Error()

	// Check for common field type errors
	// The TOML library returns errors like "type mismatch for core.Entrypoints: expected table but found string"
	if strings.Contains(errStr, "entrypoint") && (strings.Contains(errStr, "expected table") || strings.Contains(errStr, "expected struct")) {
		return formatBlaxelTomlWarning("entrypoint", "The 'entrypoint' field must be a table with 'prod' and/or 'dev' keys, not a string.")
	}

	if strings.Contains(errStr, "runtime") && strings.Contains(errStr, "expected") {
		return formatBlaxelTomlWarning("runtime", "The 'runtime' field must be a table, not a simple value.")
	}

	if strings.Contains(errStr, "triggers") && strings.Contains(errStr, "expected") {
		return formatBlaxelTomlWarning("triggers", "The 'triggers' field must be an array of tables.")
	}

	if strings.Contains(errStr, "volumes") && strings.Contains(errStr, "expected") {
		return formatBlaxelTomlWarning("volumes", "The 'volumes' field must be an array of tables.")
	}

	// Generic TOML parse error - show the error and a sample
	return formatBlaxelTomlWarning("", errStr)
}

// formatBlaxelTomlWarning formats a warning message with a sample blaxel.toml
func formatBlaxelTomlWarning(field string, reason string) string {
	codeColor := color.New(color.FgCyan)
	warningColor := color.New(color.FgYellow)

	var warningMsg strings.Builder

	warningMsg.WriteString("⚠️  blaxel.toml Configuration Warning\n")
	warningMsg.WriteString(strings.Repeat("━", 60) + "\n\n")

	if field != "" {
		warningMsg.WriteString(fmt.Sprintf("%s Invalid '%s' field\n", warningColor.Sprint("⚠"), codeColor.Sprint(field)))
	}
	warningMsg.WriteString(fmt.Sprintf("%s %s\n\n", warningColor.Sprint("Reason:"), reason))

	warningMsg.WriteString("Here is a complete sample of a valid blaxel.toml:\n\n")

	sample := getBlaxelTomlSample()
	for _, line := range strings.Split(sample, "\n") {
		warningMsg.WriteString(codeColor.Sprint(line) + "\n")
	}

	warningMsg.WriteString("\n" + strings.Repeat("━", 60) + "\n")
	warningMsg.WriteString("Learn more: https://docs.blaxel.ai/Agents/Deploy-an-agent\n\n")
	warningMsg.WriteString("⚠️  Blaxel will attempt to deploy with default settings, but this may fail.\n")
	warningMsg.WriteString(strings.Repeat("━", 60))

	return warningMsg.String()
}

// getBlaxelTomlSample returns a complete sample of a valid blaxel.toml
func getBlaxelTomlSample() string {
	return `# Basic configuration
type = "agent"  # Can be: agent, function, job, sandbox, volume-template
name = "my-resource" # Optional, default to the directory name
# public = true  # Optional, makes the agent publicly accessible (agent only)

# Entrypoint configuration (optional)
[entrypoint]
prod = "python main.py"
dev = "python main.py --dev"

# Environment variables (optional)
[env]
MY_VAR = "my-value"

# Runtime configuration (optional)
[runtime]
memory = 4096
# Job configuration (optional)
# maxConcurrentTasks = 10
# timeout = "15m"  # Supports: 30s, 5m, 1h, 2d, 1w or plain seconds (900)
# maxRetries = 0

# Volumes for Sandbox (optional)
# [[volumes]]
# name = "my-volume"
# mountPath = "/data"

# Volume templates (optional)
# directory = "."
# defaultSize = 1024

# Job triggers - for scheduled jobs (type = "job")
# [[triggers]]
# type = "schedule"
# schedule = "0 * * * *"  # Cron expression (every hour)

# HTTP triggers - for agents/functions
# [[triggers]]
# id = "my-trigger"
# type = "http"
# [triggers.configuration]
# path = "/webhook"
# authenticationType = "public"

# Async HTTP triggers with timeout
# [[triggers]]
# id = "async-trigger"
# type = "http-async"
# timeout = "15m"  # Supports: 30s, 5m, 15m or plain seconds (900)`
}

// promptForDeploymentType prompts the user to select what they want to deploy
// when no blaxel.toml file is found
func PromptForDeploymentType() string {
	var selected string
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("What are you trying to deploy ?").
				Options(
					huh.NewOption("Agent", "agent"),
					huh.NewOption("MCP (Function)", "function"),
					huh.NewOption("Sandbox", "sandbox"),
					huh.NewOption("Job", "job"),
					huh.NewOption("Volume Template", "volumetemplate"),
				).
				Value(&selected),
		),
	)
	form.WithTheme(GetHuhTheme())
	if err := form.Run(); err != nil {
		// User cancelled or error occurred
		return ""
	}
	return selected
}

// Add missing methods for Resource struct

// ListExec method for Resource
func (r *Resource) ListExec() ([]interface{}, error) {
	// This is a placeholder - the actual implementation should be moved here from CLI files
	return nil, nil
}

// PutFn method for Resource - placeholder implementation
func (r *Resource) PutFn(resourceName string, name string, resourceObject interface{}) interface{} {
	// This is a placeholder - the actual implementation should be moved here from CLI files
	return nil
}

// PostFn method for Resource - placeholder implementation
func (r *Resource) PostFn(resourceName string, name string, resourceObject interface{}) interface{} {
	// This is a placeholder - the actual implementation should be moved here from CLI files
	return nil
}

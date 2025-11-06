package core

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/BurntSushi/toml"
	"github.com/blaxel-ai/toolkit/sdk"
)

type Resource struct {
	Kind             string
	Short            string
	Plural           string
	Singular         string
	Aliases          []string
	SpecType         reflect.Type
	List             interface{}
	Get              interface{}
	Delete           interface{}
	Put              interface{}
	Post             interface{}
	AdditionalFields map[string]string // map[columnName]fieldPath - e.g., "STATUS": "status", "IMAGE": "spec.runtime.image"
}

var resources = []*Resource{
	{
		Kind:             "Policy",
		Short:            "pol",
		Plural:           "policies",
		Singular:         "policy",
		SpecType:         reflect.TypeOf(sdk.Policy{}),
		AdditionalFields: map[string]string{},
	},
	{
		Kind:     "Model",
		Short:    "ml",
		Plural:   "models",
		Singular: "model",
		SpecType: reflect.TypeOf(sdk.Model{}),
		AdditionalFields: map[string]string{
			"STATUS": "status",
		},
	},
	{
		Kind:     "Function",
		Short:    "fn",
		Plural:   "functions",
		Singular: "function",
		Aliases:  []string{"mcp", "mcps"},
		SpecType: reflect.TypeOf(sdk.Function{}),
		AdditionalFields: map[string]string{
			"STATUS": "status",
			"IMAGE":  "spec.runtime.image",
		},
	},
	{
		Kind:     "Agent",
		Short:    "ag",
		Plural:   "agents",
		Singular: "agent",
		SpecType: reflect.TypeOf(sdk.Agent{}),
		AdditionalFields: map[string]string{
			"STATUS": "status",
			"IMAGE":  "spec.runtime.image",
		},
	},
	{
		Kind:             "IntegrationConnection",
		Short:            "ic",
		Plural:           "integrationconnections",
		Singular:         "integrationconnection",
		SpecType:         reflect.TypeOf(sdk.IntegrationConnection{}),
		AdditionalFields: map[string]string{},
	},
	{
		Kind:     "Sandbox",
		Short:    "sbx",
		Plural:   "sandboxes",
		Singular: "sandbox",
		SpecType: reflect.TypeOf(sdk.Sandbox{}),
		AdditionalFields: map[string]string{
			"STATUS": "status",
			"IMAGE":  "spec.runtime.image",
			"REGION": "spec.region",
		},
	},
	{
		Kind:     "Job",
		Short:    "jb",
		Plural:   "jobs",
		Singular: "job",
		SpecType: reflect.TypeOf(sdk.Job{}),
		AdditionalFields: map[string]string{
			"STATUS": "status",
			"IMAGE":  "spec.runtime.image",
			"REGION": "spec.region",
		},
	},
	{
		Kind:     "Volume",
		Short:    "vol",
		Plural:   "volumes",
		Singular: "volume",
		SpecType: reflect.TypeOf(sdk.Volume{}),
		AdditionalFields: map[string]string{
			"STATUS": "status",
			"SIZE":   "spec.size",
			"REGION": "spec.region",
		},
	},
	{
		Kind:     "VolumeTemplate",
		Short:    "vt",
		Plural:   "volumetemplates",
		Singular: "volumetemplate",
		SpecType: reflect.TypeOf(sdk.VolumeTemplate{}),
		AdditionalFields: map[string]string{
			"SIZE": "spec.defaultSize",
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
}

func readConfigToml(folder string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}

	content, err := os.ReadFile(filepath.Join(cwd, folder, "blaxel.toml"))
	if err != nil {
		config.Functions = []string{"all"}
		config.Models = []string{"all"}
		config.Type = "agent"
		return
	}

	err = toml.Unmarshal(content, &config)
	if err != nil {
		fmt.Println(err)
		return
	}

	if config.Type == "" {
		config.Type = "agent"
	}

	if config.Workspace != "" {
		workspace = config.Workspace
	}
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

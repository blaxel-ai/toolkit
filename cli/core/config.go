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
	Kind       string
	Short      string
	Plural     string
	Singular   string
	SpecType   reflect.Type
	List       interface{}
	Get        interface{}
	Delete     interface{}
	Put        interface{}
	Post       interface{}
	WithStatus bool
	WithImage  bool
}

var resources = []*Resource{
	{
		Kind:       "Policy",
		Short:      "pol",
		Plural:     "policies",
		Singular:   "policy",
		SpecType:   reflect.TypeOf(sdk.Policy{}),
		WithStatus: false,
		WithImage:  false,
	},
	{
		Kind:       "Model",
		Short:      "ml",
		Plural:     "models",
		Singular:   "model",
		SpecType:   reflect.TypeOf(sdk.Model{}),
		WithStatus: true,
		WithImage:  false,
	},
	{
		Kind:       "Function",
		Short:      "fn",
		Plural:     "functions",
		Singular:   "function",
		SpecType:   reflect.TypeOf(sdk.Function{}),
		WithStatus: true,
		WithImage:  true,
	},
	{
		Kind:       "Agent",
		Short:      "ag",
		Plural:     "agents",
		Singular:   "agent",
		SpecType:   reflect.TypeOf(sdk.Agent{}),
		WithStatus: true,
		WithImage:  true,
	},
	{
		Kind:       "IntegrationConnection",
		Short:      "ic",
		Plural:     "integrationconnections",
		Singular:   "integrationconnection",
		SpecType:   reflect.TypeOf(sdk.IntegrationConnection{}),
		WithStatus: false,
		WithImage:  false,
	},
	{
		Kind:       "Sandbox",
		Short:      "sbx",
		Plural:     "sandboxes",
		Singular:   "sandbox",
		SpecType:   reflect.TypeOf(sdk.Sandbox{}),
		WithStatus: true,
		WithImage:  true,
	},
	{
		Kind:       "Job",
		Short:      "jb",
		Plural:     "jobs",
		Singular:   "job",
		SpecType:   reflect.TypeOf(sdk.Job{}),
		WithStatus: true,
		WithImage:  true,
	},
}

type Package struct {
	Path string `toml:"path"`
	Port int    `toml:"port,omitempty"`
	Type string `toml:"type,omitempty"`
}

// readConfigToml reads the config.toml file and upgrade config according to content
type Config struct {
	Name       string                    `toml:"name"`
	Workspace  string                    `toml:"workspace"`
	Type       string                    `toml:"type"`
	Protocol   string                    `toml:"protocol"`
	Functions  []string                  `toml:"functions"`
	Models     []string                  `toml:"models"`
	Agents     []string                  `toml:"agents"`
	Entrypoint Entrypoints               `toml:"entrypoint"`
	Env        Envs                      `toml:"env"`
	Function   map[string]Package        `toml:"function"`
	Agent      map[string]Package        `toml:"agent"`
	SkipRoot   bool                      `toml:"skipRoot"`
	Runtime    *map[string]interface{}   `toml:"runtime"`
	Triggers   *[]map[string]interface{} `toml:"triggers"`
	Policies   []string                  `toml:"policies,omitempty"`
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

package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"

	"github.com/BurntSushi/toml"
	"github.com/beamlit/toolkit/sdk"
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
}

var resources = []*Resource{
	{
		Kind:     "Policy",
		Short:    "pol",
		Plural:   "policies",
		Singular: "policy",
		SpecType: reflect.TypeOf(sdk.Policy{}),
	},
	{
		Kind:       "Model",
		Short:      "ml",
		Plural:     "models",
		Singular:   "model",
		SpecType:   reflect.TypeOf(sdk.Model{}),
		WithStatus: true,
	},
	{
		Kind:       "Function",
		Short:      "fn",
		Plural:     "functions",
		Singular:   "function",
		SpecType:   reflect.TypeOf(sdk.Function{}),
		WithStatus: true,
	},
	{
		Kind:       "Agent",
		Short:      "ag",
		Plural:     "agents",
		Singular:   "agent",
		SpecType:   reflect.TypeOf(sdk.Agent{}),
		WithStatus: true,
	},
	{
		Kind:     "IntegrationConnection",
		Short:    "ic",
		Plural:   "integrationconnections",
		Singular: "integrationconnection",
		SpecType: reflect.TypeOf(sdk.IntegrationConnection{}),
	},
	{
		Kind:     "UVM",
		Short:    "uvm",
		Plural:   "uvms",
		Singular: "uvm",
		SpecType: reflect.TypeOf(sdk.UVM{}),
	},
}

type Package struct {
	Path string `toml:"path"`
	Port int    `toml:"port,omitempty"`
	Type string `toml:"type,omitempty"`
}

// readConfigToml reads the config.toml file and upgrade config according to content
type Config struct {
	Name       string             `toml:"name"`
	Workspace  string             `toml:"workspace"`
	Type       string             `toml:"type"`
	Protocol   string             `toml:"protocol"`
	Functions  []string           `toml:"functions"`
	Models     []string           `toml:"models"`
	Agents     []string           `toml:"agents"`
	Entrypoint Entrypoints        `toml:"entrypoint"`
	Env        Envs               `toml:"env"`
	Function   map[string]Package `toml:"function"`
	Agent      map[string]Package `toml:"agent"`

	Runtime  *map[string]interface{}   `toml:"runtime"`
	Triggers *[]map[string]interface{} `toml:"triggers"`
	Policies []string                  `toml:"policies,omitempty"`
}

func readConfigToml() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}

	content, err := os.ReadFile(filepath.Join(cwd, "blaxel.toml"))
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
		fmt.Printf("Using workspace %s from blaxel.toml\n", config.Workspace)
		workspace = config.Workspace
	}
}

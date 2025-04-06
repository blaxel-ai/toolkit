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
	Packages   map[string]Package `toml:"packages"`
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
		return
	}

	err = toml.Unmarshal(content, &config)
	if err != nil {
		fmt.Println(err)
		return
	}

	if config.Workspace != "" {
		fmt.Printf("Using workspace %s from blaxel.toml\n", config.Workspace)
		workspace = config.Workspace
	}
}

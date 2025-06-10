package core

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

type ResultMetadata struct {
	Workspace string
	Name      string
}

type Result struct {
	ApiVersion string      `yaml:"apiVersion" json:"apiVersion"`
	Kind       string      `yaml:"kind" json:"kind"`
	Metadata   interface{} `yaml:"metadata" json:"metadata"`
	Spec       interface{} `yaml:"spec" json:"spec"`
	Status     string      `yaml:"status,omitempty" json:"status,omitempty"`
}

func (r *Result) ToString() string {
	yaml, err := yaml.Marshal(r)
	if err != nil {
		return ""
	}
	return string(yaml)
}

type CommandEnv map[string]string

func (c *CommandEnv) Set(key, value string) {
	(*c)[key] = value
}

func (c *CommandEnv) AddClientEnv() {
	for _, envVar := range os.Environ() {
		parts := strings.Split(envVar, "=")
		if len(parts) < 2 {
			continue
		}
		key := parts[0]
		value := strings.Join(parts[1:], "=")
		c.Set(key, value)
	}
}

func (c *CommandEnv) ToEnv() []string {
	env := []string{}
	for k, v := range *c {
		env = append(env, fmt.Sprintf("%s=%s", k, v))
	}
	return env
}

package cli

import "gopkg.in/yaml.v2"

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

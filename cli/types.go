package cli

type ResultMetadata struct {
	Workspace string
	Name      string
}

type Result struct {
	ApiVersion string      `yaml:"apiVersion" json:"apiVersion"`
	Kind       string      `yaml:"kind" json:"kind"`
	Metadata   interface{} `yaml:"metadata" json:"metadata"`
	Spec       interface{} `yaml:"spec" json:"spec"`
	Status     interface{} `yaml:"status,omitempty" json:"status,omitempty"`
}

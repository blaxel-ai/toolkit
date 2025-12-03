package sdk

type Config struct {
	Context    ContextConfig     `yaml:"context"`
	Workspaces []WorkspaceConfig `yaml:"workspaces"`
	Tracking   *bool             `yaml:"tracking,omitempty"`
}
type WorkspaceConfig struct {
	Name        string      `yaml:"name"`
	Credentials Credentials `yaml:"credentials"`
	Env         string      `yaml:"env"`
}

type ContextConfig struct {
	Workspace string `yaml:"workspace"`
}

type Credentials struct {
	APIKey            string `yaml:"apiKey"`
	AccessToken       string `yaml:"access_token"`
	RefreshToken      string `yaml:"refresh_token"`
	ExpiresIn         int    `yaml:"expires_in"`
	DeviceCode        string `yaml:"device_code"`
	ClientCredentials string `yaml:"client_credentials"`
}

func (c Credentials) IsValid() bool {
	return c.APIKey != "" || c.AccessToken != "" || c.RefreshToken != "" || c.ClientCredentials != ""
}

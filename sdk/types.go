package sdk

type Config struct {
	Context    ContextConfig     `yaml:"context"`
	Workspaces []WorkspaceConfig `yaml:"workspaces"`
}
type WorkspaceConfig struct {
	Name        string      `yaml:"name"`
	Credentials Credentials `yaml:"credentials"`
}

type ContextConfig struct {
	Workspace   string `yaml:"workspace"`
	Environment string `yaml:"environment"`
}

type Credentials struct {
	APIKey       string `yaml:"apiKey"`
	AccessToken  string `yaml:"accessToken"`
	RefreshToken string `yaml:"refreshToken"`
	ExpiresIn    int    `yaml:"expiresIn"`
	DeviceCode   string `yaml:"deviceCode"`
}

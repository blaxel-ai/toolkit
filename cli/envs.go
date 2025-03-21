package cli

type Envs map[string]string
type Env struct {
	Name  string
	Value string
}

func GetEnvs() []Env {
	var envs []Env
	for _, secret := range secrets {
		envs = append(envs, Env{
			Name:  secret.Name,
			Value: secret.Value,
		})
	}
	for k, v := range config.Env {
		envs = append(envs, Env{
			Name:  k,
			Value: v,
		})
	}
	return envs
}

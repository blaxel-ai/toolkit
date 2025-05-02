package cli

import "slices"

type Envs map[string]string
type Env struct {
	Name  string
	Value string
}

var ignoredEnvs = []string{
	"BL_API_KEY",
}

func GetEnvs() []Env {
	var envs []Env
	for _, secret := range secrets {
		if slices.Contains(ignoredEnvs, secret.Name) {
			continue
		}
		envs = append(envs, Env{
			Name:  secret.Name,
			Value: secret.Value,
		})
	}
	for k, v := range config.Env {
		if slices.Contains(ignoredEnvs, k) {
			continue
		}
		envs = append(envs, Env{
			Name:  k,
			Value: v,
		})
	}
	return envs
}

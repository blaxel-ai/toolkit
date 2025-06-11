package core

import (
	"fmt"
	"os"
	"slices"
)

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
		if v == "$secrets."+k || v == "${secrets."+k+"}" || v == "${ secrets."+k+" }" {
			if envValue, exists := os.LookupEnv(k); exists {
				v = envValue
			} else {
				fmt.Printf("It appears that the secret variable %s is not set. If it is not intentional, please set it in the .env file or in environment\n", k)
			}
		} else if v == "$"+k || v == "${"+k+"}" || v == "${ "+k+" }" {
			if envValue, exists := os.LookupEnv(k); exists {
				v = envValue
			} else {
				fmt.Printf("It appears that the environment variable %s is not set. If it is not intentional, please set it in the .env file or in environment\n", k)
			}
		}
		envs = append(envs, Env{
			Name:  k,
			Value: v,
		})
	}
	return envs
}

func GetUniqueEnvs() []Env {
	envs := GetEnvs()
	uniqueNames := make(map[string]struct{})
	for _, env := range envs {
		uniqueNames[env.Name] = struct{}{}
	}
	namesList := make([]Env, 0, len(uniqueNames))
	for name := range uniqueNames {
		for _, env := range envs {
			if env.Name == name {
				namesList = append(namesList, env)
				break
			}
		}
	}
	return namesList
}

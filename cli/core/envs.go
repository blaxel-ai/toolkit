package core

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"
)

var (
	// Matches $secrets.KEY, ${secrets.KEY}, $secrets.KEY:default, ${secrets.KEY:default}
	secretsEnvRegex = regexp.MustCompile(`^\$secrets\.([A-Za-z0-9_]+)(?::([^\s}]*))?$|^\$\{\s?secrets\.([A-Za-z0-9_]+)(?::([^}]*))?\s?\}$`)
	// Matches $KEY, ${KEY}, ${KEY:default}
	plainEnvRegex = regexp.MustCompile(`^\$\{\s?([A-Za-z0-9_]+)(?::([^}]*))?\s?\}$|^\$([A-Za-z0-9_]+)$`)
)

type Envs map[string]string
type Env struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

var ignoredEnvs = []string{
	"BL_API_KEY",
}

// ResolveVarValue resolves variable interpolation patterns in a string value.
// Supports: $secrets.KEY, ${secrets.KEY}, ${secrets.KEY:default}, $KEY, ${KEY}, ${KEY:default}
// Returns the resolved value and whether any warning should be printed (unresolved var name, or empty).
func ResolveVarValue(v string) (string, string) {
	// Handle ${secrets.KEY:default} and $secrets.KEY:default patterns
	if secretsMatch := secretsEnvRegex.FindStringSubmatch(v); secretsMatch != nil {
		secretName := secretsMatch[1]
		if secretName == "" {
			secretName = secretsMatch[3]
		}
		defaultValue := strings.TrimSpace(secretsMatch[2])
		if defaultValue == "" {
			defaultValue = strings.TrimSpace(secretsMatch[4])
		}
		if envValue, exists := os.LookupEnv(secretName); exists {
			return envValue, ""
		} else if secretValue := LookupSecret(secretName); secretValue != "" {
			return secretValue, ""
		} else if defaultValue != "" {
			return defaultValue, ""
		}
		return v, fmt.Sprintf("It appears that the secret variable %s is not set. If it is not intentional, please set it in the .env file or in environment", secretName)
	}
	// Handle ${KEY:default} and $KEY patterns (non-secrets)
	if envMatch := plainEnvRegex.FindStringSubmatch(v); envMatch != nil {
		varName := envMatch[1]
		defaultValue := strings.TrimSpace(envMatch[2])
		if varName == "" {
			varName = envMatch[3]
		}
		if envValue, exists := os.LookupEnv(varName); exists {
			return envValue, ""
		} else if defaultValue != "" {
			return defaultValue, ""
		}
		return v, fmt.Sprintf("It appears that the environment variable %s is not set. If it is not intentional, please set it in the .env file or in environment", varName)
	}
	return v, ""
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

		// Check if the environment variable is already in envs
		alreadyInEnvs := false
		for _, env := range envs {
			if env.Name == k {
				alreadyInEnvs = true
				break
			}
		}

		resolved, warning := ResolveVarValue(v)
		v = resolved
		if warning != "" && !alreadyInEnvs {
			fmt.Println(warning)
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

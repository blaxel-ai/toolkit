package core

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
)

// readSecret from .env file at root of project
type Secrets []Env

var secrets Secrets

func loadCommandSecrets() {
	for _, secret := range commandSecrets {
		parts := strings.Split(secret, "=")
		if len(parts) < 2 {
			fmt.Println("Invalid secret format", secret)
			continue
		}
		secrets = append(secrets, Env{
			Name:  parts[0],
			Value: strings.Join(parts[1:], "="),
		})
	}
}

func readSecrets(folder string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}

	for _, file := range envFiles {
		envMap, err := godotenv.Read(filepath.Join(cwd, folder, file))
		if err != nil {
			return
		}
		for key, value := range envMap {
			secrets = append(secrets, Env{
				Name:  key,
				Value: value,
			})
		}
	}
}

// GetSecrets returns the current secrets
func GetSecrets() []Env {
	return secrets
}

func LookupSecret(name string) string {
	for _, secret := range secrets {
		if secret.Name == name {
			return secret.Value
		}
	}
	return ""
}

package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// readSecret from .env file at root of project
type Secrets []Env

var secrets Secrets

func readSecrets() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		return
	}

	envMap, err := godotenv.Read(filepath.Join(cwd, ".env"))
	if err != nil {
		fmt.Println("Error reading .env file:", err)
		return
	}

	for key, value := range envMap {
		secrets = append(secrets, Env{
			Name:  key,
			Value: value,
		})
	}

}

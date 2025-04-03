package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
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

	content, err := os.ReadFile(filepath.Join(cwd, ".env"))
	if err != nil {
		return
	}

	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.Split(line, "=")
		if len(parts) != 2 {
			continue
		}
		secrets = append(secrets, Env{
			Name:  parts[0],
			Value: parts[1],
		})
	}
}

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func findTSRootCmdAsString(hotreload bool) ([]string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting current directory: %v", err)
	}
	packageJsonPath := filepath.Join(currentDir, "package.json")
	if _, err := os.Stat(packageJsonPath); err == nil {
		packageJson, err := os.ReadFile(packageJsonPath)
		if err != nil {
			return nil, fmt.Errorf("error reading package.json: %v", err)
		}
		var packageJsonObj PackageJson
		err = json.Unmarshal(packageJson, &packageJsonObj)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling package.json: %v", err)
		}
		if hotreload {
			if packageJsonObj.Scripts["dev"] != "" {
				return []string{"npm", "run", "dev"}, nil
			}
			fmt.Println("Warning: dev script not found in package.json, hotreload will not work")
		}
		if packageJsonObj.Scripts["start"] != "" {
			return []string{"npm", "run", "start"}, nil
		}
		fmt.Println("Warning: start script not found in package.json")
	} else {
		fmt.Println("Warning: package.json not found in current directory")
	}

	if _, err := os.Stat("index.ts"); err == nil {
		return []string{"tsx", "index.ts"}, nil
	}
	if _, err := os.Stat("index.js"); err == nil {
		return []string{"node", "index.js"}, nil
	}
	return nil, fmt.Errorf("index.js or index.ts not found in current directory")
}

func findTSPackageManagerCommandAsString(production bool) ([]string, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("error getting current directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(currentDir, "pnpm-lock.yaml")); err == nil {
		baseCmd := []string{"npm", "install", "-g", "pnpm", "&&", "pnpm", "install"}
		if production {
			return append(baseCmd, "--production"), nil
		}
		return baseCmd, nil
	}
	if _, err := os.Stat(filepath.Join(currentDir, "yarn.lock")); err == nil {
		if production {
			return []string{"yarn", "--production"}, nil
		}
		return []string{"yarn"}, nil
	}
	if production {
		return []string{"npm", "install", "--production"}, nil
	}
	return []string{"npm", "install"}, nil
}

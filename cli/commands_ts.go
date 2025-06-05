package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type PackageJson struct {
	Scripts map[string]string `json:"scripts"`
}

func startTypescriptServer(port int, host string, hotreload bool) *exec.Cmd {
	ts, err := findRootCmd(port, host, hotreload)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Printf("Starting server : %s\n", strings.Join(ts.Args, " "))
	if os.Getenv("COMMAND") != "" {
		command := strings.Split(os.Getenv("COMMAND"), " ")
		if len(command) > 1 {
			ts = exec.Command(command[0], command[1:]...)
		} else {
			ts = exec.Command(command[0])
		}
	}
	ts.Stdout = os.Stdout
	ts.Stderr = os.Stderr
	ts.Dir = folder

	// Set env variables
	envs := getServerEnvironment(port, host, hotreload)
	ts.Env = envs.ToEnv()

	err = ts.Start()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	return ts
}

func getPackageJson() (PackageJson, error) {
	currentDir, err := os.Getwd()
	if err != nil {
		return PackageJson{}, fmt.Errorf("error getting current directory: %v", err)
	}
	packageJsonPath := filepath.Join(currentDir, folder, "package.json")
	if _, err := os.Stat(packageJsonPath); err == nil {
		packageJson, err := os.ReadFile(packageJsonPath)
		if err != nil {
			return PackageJson{}, fmt.Errorf("error reading package.json: %v", err)
		}
		var packageJsonObj PackageJson
		err = json.Unmarshal(packageJson, &packageJsonObj)
		if err != nil {
			return PackageJson{}, fmt.Errorf("error unmarshalling package.json: %v", err)
		}
		return packageJsonObj, nil
	}
	return PackageJson{}, fmt.Errorf("package.json not found in current directory")
}

func findTSPackageManagerLockFile() string {
	currentDir, err := os.Getwd()
	if err != nil {
		return ""
	}
	if _, err := os.Stat(filepath.Join(currentDir, "pnpm-lock.yaml")); err == nil {
		return "pnpm-lock.yaml"
	}
	if _, err := os.Stat(filepath.Join(currentDir, "yarn.lock")); err == nil {
		return "yarn.lock"
	}
	if _, err := os.Stat(filepath.Join(currentDir, "package-lock.json")); err == nil {
		return "package-lock.json"
	}
	return ""
}

func findTSPackageManager() string {
	lockFile := findTSPackageManagerLockFile()
	switch lockFile {
	case "pnpm-lock.yaml":
		return "pnpm"
	case "yarn.lock":
		return "yarn"
	default:
		return "npm"
	}
}

func findStartCommand(script string) ([]string, error) {
	packageManager := findTSPackageManager()
	switch packageManager {
	case "pnpm":
		return []string{"npx", "pnpm", "run", script}, nil
	case "yarn":
		return []string{"yarn", "run", script}, nil
	default:
		return []string{"npm", "run", script}, nil
	}
}

func findTSRootCmdAsString(config RootCmdConfig) ([]string, error) {
	if config.Entrypoint.Production != "" || config.Entrypoint.Development != "" {
		cmd := config.Entrypoint.Production
		if config.Hotreload && config.Entrypoint.Development != "" {
			cmd = config.Entrypoint.Development
		}
		return strings.Split(cmd, " "), nil
	}

	packageJson, err := getPackageJson()
	if err == nil {
		if config.Hotreload {
			if packageJson.Scripts["dev"] != "" {
				return findStartCommand("dev")
			}
			fmt.Println("Warning: dev script not found in package.json, hotreload will not work")
		}
		if config.Production && packageJson.Scripts["prod"] != "" {
			return findStartCommand("prod")
		}
		if packageJson.Scripts["start"] != "" {
			return findStartCommand("start")
		}
		fmt.Println("Warning: start script not found in package.json")
	} else {
		fmt.Println("Warning: package.json not found in current directory")
	}

	files := []string{
		"dist/index.js",
		"dist/app.js",
		"dist/server.js",
		"index.js",
		"app.js",
		"server.js",
		"src/index.js",
		"src/app.js",
		"src/server.js",
	}

	for _, file := range files {
		if _, err := os.Stat(filepath.Join(config.Folder, file)); err == nil {
			return []string{"node", file}, nil
		}
	}
	return nil, fmt.Errorf("index.js, index.ts, app.js or app.ts not found in current directory")
}

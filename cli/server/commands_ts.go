package server

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
)

type PackageJson struct {
	Scripts map[string]string `json:"scripts"`
}

// FindNodeExecutable checks for node in PATH and returns the executable name
// Returns an error if node is not available
func FindNodeExecutable() (string, error) {
	if _, err := exec.LookPath("node"); err == nil {
		return "node", nil
	}
	return "", fmt.Errorf("node is not available on this system")
}

// FindPackageManagerExecutable checks for npm, yarn, or pnpm in PATH
// Returns the executable name and an error if none are available
func FindPackageManagerExecutable() (string, error) {
	// Check in order: pnpm, yarn, npm
	if _, err := exec.LookPath("pnpm"); err == nil {
		return "pnpm", nil
	}
	if _, err := exec.LookPath("yarn"); err == nil {
		return "yarn", nil
	}
	if _, err := exec.LookPath("npm"); err == nil {
		return "npm", nil
	}
	return "", fmt.Errorf("no package manager found (npm, yarn, or pnpm)")
}

func StartTypescriptServer(port int, host string, hotreload bool, folder string, config core.Config) *exec.Cmd {
	// Check if Node.js is available before attempting to start
	nodeExec, err := FindNodeExecutable()
	if err != nil {
		core.PrintError("Serve", err)
		core.PrintInfo("Please install Node.js:")
		core.PrintInfo("  - macOS: brew install node")
		core.PrintInfo("  - Linux: sudo apt-get install nodejs npm (or use your distribution's package manager)")
		core.PrintInfo("  - Windows: Download from https://nodejs.org/")
		core.PrintInfo("After installation, ensure Node.js is in your PATH.")
		os.Exit(1)
	}
	_ = nodeExec // Will be used by findTSRootCmdAsString

	ts, err := FindRootCmd(port, host, hotreload, folder, config)
	if err != nil {
		core.PrintError("Serve", err)
		os.Exit(1)
	}
	// Extract the actual command, hiding "sh -c" wrapper if present
	cmdDisplay := strings.Join(ts.Args, " ")
	if len(ts.Args) >= 3 && ts.Args[0] == "sh" && ts.Args[1] == "-c" {
		// Extract just the command after "sh -c"
		cmdDisplay = ts.Args[2]
	}
	fmt.Printf("Starting server : %s\n", cmdDisplay)
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
	envs := GetServerEnvironment(port, host, hotreload, config)
	ts.Env = envs.ToEnv()

	err = ts.Start()
	if err != nil {
		core.PrintError("Serve", fmt.Errorf("failed to start TypeScript server: %w", err))
		os.Exit(1)
	}

	return ts
}

func getPackageJson(folder string) (PackageJson, error) {
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
		// Check if pnpm is available
		if _, err := exec.LookPath("pnpm"); err != nil {
			return nil, fmt.Errorf("pnpm is not available - please install it with: npm install -g pnpm")
		}
		return []string{"npx", "pnpm", "run", script}, nil
	case "yarn":
		// Check if yarn is available
		if _, err := exec.LookPath("yarn"); err != nil {
			return nil, fmt.Errorf("yarn is not available - please install it with: npm install -g yarn")
		}
		return []string{"yarn", "run", script}, nil
	default:
		// Check if npm is available
		if _, err := exec.LookPath("npm"); err != nil {
			return nil, fmt.Errorf("npm is not available - please install Node.js which includes npm")
		}
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

	packageJson, err := getPackageJson(config.Folder)
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
			nodeExec, err := FindNodeExecutable()
			if err != nil {
				return nil, err
			}
			return []string{nodeExec, file}, nil
		}
	}
	return nil, fmt.Errorf("index.js, index.ts, app.js or app.ts not found in current directory")
}

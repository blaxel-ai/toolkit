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

func getTSDockerfile() (string, error) {
	rootCommand, err := findRootCmdAsString(RootCmdConfig{
		Hotreload:  false,
		Production: true,
	})
	if err != nil {
		return "", fmt.Errorf("failed to find root command: %w", err)
	}
	packageManagerCommand := findTSPackageManagerCommandAsString(true)
	buildCommandArgs := getTSBuildCommand()
	buildCommand := ""
	if len(buildCommandArgs) > 0 {
		buildCommand = "RUN " + strings.Join(buildCommandArgs, " ")
	}
	lockFile := findTSPackageManagerLockFile()
	if lockFile != "" {
		lockFile = "COPY " + lockFile + " /blaxel/" + lockFile
	}
	return fmt.Sprintf(`FROM node:22-alpine
WORKDIR /blaxel
COPY package.json /blaxel/package.json
%s
RUN %s
COPY . .
%s
ENTRYPOINT ["%s"]`,
			lockFile,
			strings.Join(packageManagerCommand, " "),
			buildCommand,
			strings.Join(rootCommand, "\",\"")),
		nil
}

func startTypescriptServer(port int, host string, hotreload bool) *exec.Cmd {
	ts, err := findRootCmd(hotreload)
	fmt.Printf("Starting server : %s\n", strings.Join(ts.Args, " "))
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
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

	// Set env variables
	envs := getServerEnvironment(port, host)
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
	packageJsonPath := filepath.Join(currentDir, "package.json")
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

func findTSRootCmdAsString(config RootCmdConfig) ([]string, error) {
	packageJson, err := getPackageJson()
	if err == nil {
		if config.Hotreload {
			if packageJson.Scripts["dev"] != "" {
				return strings.Split(packageJson.Scripts["dev"], " "), nil
			}
			fmt.Println("Warning: dev script not found in package.json, hotreload will not work")
		}
		if config.Production && packageJson.Scripts["prod"] != "" {
			return strings.Split(packageJson.Scripts["prod"], " "), nil
		}
		if packageJson.Scripts["start"] != "" {
			return strings.Split(packageJson.Scripts["start"], " "), nil
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
	if _, err := os.Stat("app.ts"); err == nil {
		return []string{"tsx", "app.ts"}, nil
	}
	if _, err := os.Stat("app.js"); err == nil {
		return []string{"node", "app.js"}, nil
	}
	return nil, fmt.Errorf("index.js, index.ts, app.js or app.ts not found in current directory")
}

func getTSBuildCommand() []string {
	packageJson, err := getPackageJson()
	if err != nil {
		return nil
	}
	if packageJson.Scripts["build"] == "" {
		return nil
	}
	packageManager := findTSPackageManager()
	switch packageManager {
	case "pnpm":
		return []string{"pnpm", "build"}
	case "yarn":
		return []string{"yarn", "build"}
	default:
		return strings.Split(packageJson.Scripts["build"], " ")
	}
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

func findTSPackageManagerCommandAsString(production bool) []string {
	// Production mode is not supported for now cause we build in onestage
	packageManager := findTSPackageManager()
	if packageManager == "pnpm" {
		baseCmd := []string{"npm", "install", "-g", "pnpm", "&&", "pnpm", "install"}
		// if production {
		// 	return append(baseCmd, "--production")
		// }
		return baseCmd
	}
	if packageManager == "yarn" {
		// if production {
		// 	return []string{"yarn", "--production"}
		// }
		return []string{"yarn"}
	}
	// if production {
	// 	return []string{"npm", "install", "--production"}
	// }
	return []string{"npm", "install"}
}

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

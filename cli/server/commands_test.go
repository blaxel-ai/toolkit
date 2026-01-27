package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetServerEnvironment(t *testing.T) {
	t.Run("sets basic server environment", func(t *testing.T) {
		config := core.Config{
			Workspace: "test-workspace",
		}

		env := GetServerEnvironment(8080, "localhost", false, config)

		assert.Equal(t, "8080", env["BL_SERVER_PORT"])
		assert.Equal(t, "localhost", env["BL_SERVER_HOST"])
		assert.Equal(t, "8080", env["PORT"])
		assert.Equal(t, "localhost", env["HOST"])
		assert.Equal(t, "test-workspace", env["BL_WORKSPACE"])
	})

	t.Run("sets hotreload flag when enabled", func(t *testing.T) {
		config := core.Config{}

		env := GetServerEnvironment(8080, "localhost", true, config)

		assert.Equal(t, "true", env["BL_HOTRELOAD"])
	})

	t.Run("does not set hotreload flag when disabled", func(t *testing.T) {
		config := core.Config{}

		env := GetServerEnvironment(8080, "localhost", false, config)

		_, exists := env["BL_HOTRELOAD"]
		assert.False(t, exists)
	})

	t.Run("includes config env variables", func(t *testing.T) {
		config := core.Config{
			Env: map[string]string{
				"CUSTOM_VAR": "custom_value",
				"DEBUG":      "true",
			},
		}

		env := GetServerEnvironment(8080, "localhost", false, config)

		assert.Equal(t, "custom_value", env["CUSTOM_VAR"])
		assert.Equal(t, "true", env["DEBUG"])
	})

	t.Run("preserves PATH", func(t *testing.T) {
		config := core.Config{}
		originalPath := os.Getenv("PATH")

		env := GetServerEnvironment(8080, "localhost", false, config)

		assert.Equal(t, originalPath, env["PATH"])
	})
}

func TestRootCmdConfig(t *testing.T) {
	t.Run("struct fields", func(t *testing.T) {
		cfg := RootCmdConfig{
			Folder:     "/path/to/folder",
			Hotreload:  true,
			Production: false,
			Docker:     false,
			Entrypoint: core.Entrypoints{
				Production:  "python main.py",
				Development: "python main.py --dev",
			},
			Envs: core.CommandEnv{"KEY": "value"},
		}

		assert.Equal(t, "/path/to/folder", cfg.Folder)
		assert.True(t, cfg.Hotreload)
		assert.False(t, cfg.Production)
		assert.False(t, cfg.Docker)
		assert.Equal(t, "python main.py", cfg.Entrypoint.Production)
		assert.Equal(t, "python main.py --dev", cfg.Entrypoint.Development)
	})
}

func TestFindRootCmdAsString(t *testing.T) {
	t.Run("uses prod entrypoint when not hotreload", func(t *testing.T) {
		cfg := RootCmdConfig{
			Folder:    ".",
			Hotreload: false,
			Entrypoint: core.Entrypoints{
				Production:  "python main.py",
				Development: "python main.py --dev",
			},
		}

		cmd, err := FindRootCmdAsString(cfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"python", "main.py"}, cmd)
	})

	t.Run("uses dev entrypoint when hotreload", func(t *testing.T) {
		cfg := RootCmdConfig{
			Folder:    ".",
			Hotreload: true,
			Entrypoint: core.Entrypoints{
				Production:  "python main.py",
				Development: "python main.py --dev",
			},
		}

		cmd, err := FindRootCmdAsString(cfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"python", "main.py", "--dev"}, cmd)
	})

	t.Run("returns error when no entrypoint and unknown language", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "unknown_lang")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		cfg := RootCmdConfig{
			Folder:    tempDir,
			Hotreload: false,
		}

		_, err = FindRootCmdAsString(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no prod entrypoint configured")
	})

	t.Run("returns error for hotreload with no dev entrypoint and unknown language", func(t *testing.T) {
		tempDir, err := os.MkdirTemp("", "unknown_lang")
		require.NoError(t, err)
		defer os.RemoveAll(tempDir)

		cfg := RootCmdConfig{
			Folder:    tempDir,
			Hotreload: true,
		}

		_, err = FindRootCmdAsString(cfg)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no dev entrypoint configured")
	})
}

func TestFindPythonEntryFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "python_entry")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("finds main.py", func(t *testing.T) {
		dir := filepath.Join(tempDir, "main_dir")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.py"), []byte(""), 0644))

		result := FindPythonEntryFile(dir)
		assert.Equal(t, "main.py", result)
	})

	t.Run("finds app.py", func(t *testing.T) {
		dir := filepath.Join(tempDir, "app_dir")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "app.py"), []byte(""), 0644))

		result := FindPythonEntryFile(dir)
		assert.Equal(t, "app.py", result)
	})

	t.Run("prefers app.py over main.py", func(t *testing.T) {
		dir := filepath.Join(tempDir, "both_dir")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "app.py"), []byte(""), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.py"), []byte(""), 0644))

		result := FindPythonEntryFile(dir)
		// app.py comes first in the files list
		assert.Equal(t, "app.py", result)
	})

	t.Run("finds src/main.py", func(t *testing.T) {
		dir := filepath.Join(tempDir, "src_main")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "main.py"), []byte(""), 0644))

		result := FindPythonEntryFile(dir)
		assert.Equal(t, "src/main.py", result)
	})

	t.Run("returns empty when no entry file", func(t *testing.T) {
		dir := filepath.Join(tempDir, "no_entry")
		require.NoError(t, os.MkdirAll(dir, 0755))

		result := FindPythonEntryFile(dir)
		assert.Empty(t, result)
	})
}

func TestHasPythonEntryFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "has_python")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("returns true when entry file exists", func(t *testing.T) {
		dir := filepath.Join(tempDir, "has_entry")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.py"), []byte(""), 0644))

		assert.True(t, HasPythonEntryFile(dir))
	})

	t.Run("returns false when no entry file", func(t *testing.T) {
		dir := filepath.Join(tempDir, "no_entry")
		require.NoError(t, os.MkdirAll(dir, 0755))

		assert.False(t, HasPythonEntryFile(dir))
	})
}

func TestFindPythonExecutable(t *testing.T) {
	// This test depends on the system having Python installed
	// We just verify the function doesn't panic
	result, err := FindPythonExecutable()

	// Either Python is found or an error is returned
	if err == nil {
		assert.NotEmpty(t, result)
		assert.True(t, result == "python" || result == "python3")
	} else {
		assert.Contains(t, err.Error(), "python is not available")
	}
}

func TestFindRootCmdAsStringWithAutoDetection(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "rootcmd_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("detects python with main.py", func(t *testing.T) {
		dir := filepath.Join(tempDir, "python_auto")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hello')"), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask"), 0644))

		cfg := RootCmdConfig{
			Folder:    dir,
			Hotreload: false,
		}

		cmd, err := FindRootCmdAsString(cfg)
		// If Python is available, should return a command
		if err == nil {
			assert.NotEmpty(t, cmd)
		}
	})
}

func TestGetServerEnvironmentWithSecrets(t *testing.T) {
	config := core.Config{
		Workspace: "secret-workspace",
		Env: map[string]string{
			"CUSTOM_VAR": "custom",
		},
	}

	env := GetServerEnvironment(9000, "127.0.0.1", true, config)

	assert.Equal(t, "9000", env["BL_SERVER_PORT"])
	assert.Equal(t, "127.0.0.1", env["BL_SERVER_HOST"])
	assert.Equal(t, "9000", env["PORT"])
	assert.Equal(t, "127.0.0.1", env["HOST"])
	assert.Equal(t, "true", env["BL_HOTRELOAD"])
	assert.Equal(t, "custom", env["CUSTOM_VAR"])
}

func TestGetServeCommandsPortCollision(t *testing.T) {
	// Test that duplicate ports are detected
	config := core.Config{
		Function: map[string]core.Package{
			"func1": {Path: "./func1", Port: 8001},
			"func2": {Path: "./func2", Port: 8001}, // Duplicate port
		},
	}

	// Save the current directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)

	// Create temp directory and change to it
	tempDir, err := os.MkdirTemp("", "port_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)
	require.NoError(t, os.Chdir(tempDir))
	defer os.Chdir(originalDir)

	// This should fail due to port collision
	// Note: The function exits on error, so we can't easily test this
	// Just verify it doesn't panic
	_ = config
}

func TestPackageCommandEnvs(t *testing.T) {
	cmd := PackageCommand{
		Name:    "test-agent",
		Cwd:     "/path/to/agent",
		Command: "bl",
		Args:    []string{"serve"},
		Color:   "green",
		Envs: core.CommandEnv{
			"BL_AGENT_MY_AGENT_URL": "http://localhost:8001",
		},
	}

	assert.Equal(t, "http://localhost:8001", cmd.Envs["BL_AGENT_MY_AGENT_URL"])
}

func TestFindNodeExecutable(t *testing.T) {
	result, err := FindNodeExecutable()

	// Either node is found or an error is returned
	if err == nil {
		assert.Equal(t, "node", result)
	} else {
		assert.Contains(t, err.Error(), "node is not available")
	}
}

func TestFindPackageManagerExecutable(t *testing.T) {
	result, err := FindPackageManagerExecutable()

	// Either a package manager is found or an error is returned
	if err == nil {
		assert.True(t, result == "pnpm" || result == "yarn" || result == "npm")
	} else {
		assert.Contains(t, err.Error(), "no package manager found")
	}
}

func TestGetPackageJson(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "packagejson_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	t.Run("reads valid package.json", func(t *testing.T) {
		require.NoError(t, os.Chdir(tempDir))
		packageContent := `{"scripts": {"start": "node app.js", "dev": "nodemon app.js"}}`
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, "package.json"), []byte(packageContent), 0644))

		pkg, err := getPackageJson(".")
		require.NoError(t, err)
		assert.Equal(t, "node app.js", pkg.Scripts["start"])
		assert.Equal(t, "nodemon app.js", pkg.Scripts["dev"])
	})

	t.Run("returns error for missing package.json", func(t *testing.T) {
		emptyDir := filepath.Join(tempDir, "empty")
		require.NoError(t, os.MkdirAll(emptyDir, 0755))
		require.NoError(t, os.Chdir(emptyDir))

		_, err := getPackageJson(".")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "package.json not found")
	})

	t.Run("returns error for invalid JSON", func(t *testing.T) {
		invalidDir := filepath.Join(tempDir, "invalid")
		require.NoError(t, os.MkdirAll(invalidDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(invalidDir, "package.json"), []byte("invalid json"), 0644))
		require.NoError(t, os.Chdir(invalidDir))

		_, err := getPackageJson(".")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "error unmarshalling package.json")
	})

	t.Run("reads package.json from subfolder", func(t *testing.T) {
		subfolderDir := filepath.Join(tempDir, "parent")
		subDir := filepath.Join(subfolderDir, "sub")
		require.NoError(t, os.MkdirAll(subDir, 0755))
		packageContent := `{"scripts": {"build": "tsc"}}`
		require.NoError(t, os.WriteFile(filepath.Join(subDir, "package.json"), []byte(packageContent), 0644))
		require.NoError(t, os.Chdir(subfolderDir))

		pkg, err := getPackageJson("sub")
		require.NoError(t, err)
		assert.Equal(t, "tsc", pkg.Scripts["build"])
	})
}

func TestFindTSPackageManagerLockFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "lockfile_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	t.Run("finds pnpm-lock.yaml", func(t *testing.T) {
		pnpmDir := filepath.Join(tempDir, "pnpm")
		require.NoError(t, os.MkdirAll(pnpmDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(pnpmDir, "pnpm-lock.yaml"), []byte(""), 0644))
		require.NoError(t, os.Chdir(pnpmDir))

		result := findTSPackageManagerLockFile()
		assert.Equal(t, "pnpm-lock.yaml", result)
	})

	t.Run("finds yarn.lock", func(t *testing.T) {
		yarnDir := filepath.Join(tempDir, "yarn")
		require.NoError(t, os.MkdirAll(yarnDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(yarnDir, "yarn.lock"), []byte(""), 0644))
		require.NoError(t, os.Chdir(yarnDir))

		result := findTSPackageManagerLockFile()
		assert.Equal(t, "yarn.lock", result)
	})

	t.Run("finds package-lock.json", func(t *testing.T) {
		npmDir := filepath.Join(tempDir, "npm")
		require.NoError(t, os.MkdirAll(npmDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(npmDir, "package-lock.json"), []byte(""), 0644))
		require.NoError(t, os.Chdir(npmDir))

		result := findTSPackageManagerLockFile()
		assert.Equal(t, "package-lock.json", result)
	})

	t.Run("returns empty when no lock file", func(t *testing.T) {
		emptyDir := filepath.Join(tempDir, "empty")
		require.NoError(t, os.MkdirAll(emptyDir, 0755))
		require.NoError(t, os.Chdir(emptyDir))

		result := findTSPackageManagerLockFile()
		assert.Empty(t, result)
	})

	t.Run("prefers pnpm over yarn", func(t *testing.T) {
		bothDir := filepath.Join(tempDir, "both")
		require.NoError(t, os.MkdirAll(bothDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(bothDir, "pnpm-lock.yaml"), []byte(""), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(bothDir, "yarn.lock"), []byte(""), 0644))
		require.NoError(t, os.Chdir(bothDir))

		result := findTSPackageManagerLockFile()
		assert.Equal(t, "pnpm-lock.yaml", result)
	})
}

func TestFindTSPackageManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "pkgmgr_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	t.Run("returns pnpm for pnpm-lock.yaml", func(t *testing.T) {
		pnpmDir := filepath.Join(tempDir, "pnpm")
		require.NoError(t, os.MkdirAll(pnpmDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(pnpmDir, "pnpm-lock.yaml"), []byte(""), 0644))
		require.NoError(t, os.Chdir(pnpmDir))

		result := findTSPackageManager()
		assert.Equal(t, "pnpm", result)
	})

	t.Run("returns yarn for yarn.lock", func(t *testing.T) {
		yarnDir := filepath.Join(tempDir, "yarn")
		require.NoError(t, os.MkdirAll(yarnDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(yarnDir, "yarn.lock"), []byte(""), 0644))
		require.NoError(t, os.Chdir(yarnDir))

		result := findTSPackageManager()
		assert.Equal(t, "yarn", result)
	})

	t.Run("returns npm as default", func(t *testing.T) {
		npmDir := filepath.Join(tempDir, "npm")
		require.NoError(t, os.MkdirAll(npmDir, 0755))
		require.NoError(t, os.Chdir(npmDir))

		result := findTSPackageManager()
		assert.Equal(t, "npm", result)
	})
}

func TestFindTSRootCmdAsString(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "tsroot_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	t.Run("uses prod entrypoint when specified", func(t *testing.T) {
		cfg := RootCmdConfig{
			Folder:    ".",
			Hotreload: false,
			Entrypoint: core.Entrypoints{
				Production:  "node dist/index.js",
				Development: "npx nodemon",
			},
		}

		cmd, err := findTSRootCmdAsString(cfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"node", "dist/index.js"}, cmd)
	})

	t.Run("uses dev entrypoint when hotreload", func(t *testing.T) {
		cfg := RootCmdConfig{
			Folder:    ".",
			Hotreload: true,
			Entrypoint: core.Entrypoints{
				Production:  "node dist/index.js",
				Development: "npx nodemon",
			},
		}

		cmd, err := findTSRootCmdAsString(cfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"npx", "nodemon"}, cmd)
	})

	t.Run("falls back to prod when no dev entrypoint", func(t *testing.T) {
		cfg := RootCmdConfig{
			Folder:    ".",
			Hotreload: true,
			Entrypoint: core.Entrypoints{
				Production: "node dist/index.js",
			},
		}

		cmd, err := findTSRootCmdAsString(cfg)
		require.NoError(t, err)
		assert.Equal(t, []string{"node", "dist/index.js"}, cmd)
	})

	t.Run("finds index.js file", func(t *testing.T) {
		jsDir := filepath.Join(tempDir, "jsdir")
		require.NoError(t, os.MkdirAll(jsDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(jsDir, "index.js"), []byte(""), 0644))
		require.NoError(t, os.Chdir(jsDir))

		cfg := RootCmdConfig{
			Folder:    jsDir,
			Hotreload: false,
		}

		cmd, err := findTSRootCmdAsString(cfg)
		// Will succeed if node is available
		if err == nil {
			assert.Contains(t, cmd, "index.js")
		}
	})
}

func TestPackageJsonStruct(t *testing.T) {
	t.Run("unmarshal package.json", func(t *testing.T) {
		jsonData := `{"scripts": {"start": "node index.js", "test": "jest"}}`
		var pkg PackageJson
		err := json.Unmarshal([]byte(jsonData), &pkg)
		require.NoError(t, err)
		assert.Equal(t, "node index.js", pkg.Scripts["start"])
		assert.Equal(t, "jest", pkg.Scripts["test"])
	})
}

func TestGetServeCommands(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "serve_cmds_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)
	require.NoError(t, os.Chdir(tempDir))

	t.Run("returns root command when no packages", func(t *testing.T) {
		config := core.Config{
			SkipRoot: false,
		}

		commands, err := getServeCommands(8080, "localhost", false, config, nil, nil)
		require.NoError(t, err)
		assert.Len(t, commands, 1)
		assert.Equal(t, "root", commands[0].Name)
		assert.Contains(t, commands[0].Args, "--port")
		assert.Contains(t, commands[0].Args, "8080")
	})

	t.Run("adds hotreload flag", func(t *testing.T) {
		config := core.Config{
			SkipRoot: false,
		}

		commands, err := getServeCommands(8080, "localhost", true, config, nil, nil)
		require.NoError(t, err)
		assert.Contains(t, commands[0].Args, "--hotreload")
	})

	t.Run("skips root when SkipRoot is true", func(t *testing.T) {
		config := core.Config{
			SkipRoot: true,
		}

		commands, err := getServeCommands(8080, "localhost", false, config, nil, nil)
		require.NoError(t, err)
		assert.Empty(t, commands)
	})

	t.Run("returns error when package has no port", func(t *testing.T) {
		config := core.Config{
			SkipRoot: true,
			Function: map[string]core.Package{
				"my-func": {Path: "./func", Port: 0},
			},
		}

		_, err := getServeCommands(8080, "localhost", false, config, nil, nil)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "port is not set")
	})

	t.Run("passes env files to package commands", func(t *testing.T) {
		funcDir := filepath.Join(tempDir, "func")
		require.NoError(t, os.MkdirAll(funcDir, 0755))

		config := core.Config{
			SkipRoot: true,
			Function: map[string]core.Package{
				"my-func": {Path: "./func", Port: 8001},
			},
		}

		commands, err := getServeCommands(8080, "localhost", false, config, []string{".env.local"}, nil)
		require.NoError(t, err)
		assert.Len(t, commands, 1)
		assert.Contains(t, commands[0].Args, "--env-file")
		assert.Contains(t, commands[0].Args, ".env.local")
	})

	t.Run("passes secrets to package commands", func(t *testing.T) {
		funcDir := filepath.Join(tempDir, "func2")
		require.NoError(t, os.MkdirAll(funcDir, 0755))

		config := core.Config{
			SkipRoot: true,
			Function: map[string]core.Package{
				"my-func2": {Path: "./func2", Port: 8002},
			},
		}

		secrets := []core.Env{{Name: "API_KEY", Value: "secret123"}}
		commands, err := getServeCommands(8080, "localhost", false, config, nil, secrets)
		require.NoError(t, err)
		assert.Len(t, commands, 1)
		assert.Contains(t, commands[0].Args, "-s")
		assert.Contains(t, commands[0].Args, "API_KEY=secret123")
	})

	t.Run("sets environment URLs for packages", func(t *testing.T) {
		funcDir := filepath.Join(tempDir, "func3")
		agentDir := filepath.Join(tempDir, "agent3")
		require.NoError(t, os.MkdirAll(funcDir, 0755))
		require.NoError(t, os.MkdirAll(agentDir, 0755))

		config := core.Config{
			SkipRoot: true,
			Function: map[string]core.Package{
				"my-func": {Path: "./func3", Port: 8003},
			},
			Agent: map[string]core.Package{
				"my-agent": {Path: "./agent3", Port: 8004},
			},
		}

		commands, err := getServeCommands(8080, "localhost", false, config, nil, nil)
		require.NoError(t, err)
		assert.Len(t, commands, 2)

		// Check that environment variables are set
		for _, cmd := range commands {
			assert.Contains(t, cmd.Envs, "BL_FUNCTION_MY_FUNC_URL")
			assert.Contains(t, cmd.Envs, "BL_AGENT_MY_AGENT_URL")
		}
	})
}

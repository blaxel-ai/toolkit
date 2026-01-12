package core

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSlugify(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"simple lowercase", "hello", "hello"},
		{"uppercase to lowercase", "Hello", "hello"},
		{"mixed case", "HelloWorld", "helloworld"},
		{"spaces to hyphens", "hello world", "hello-world"},
		{"underscores to hyphens", "hello_world", "hello-world"},
		{"special characters removed", "hello@world!", "helloworld"},
		{"numbers preserved", "agent123", "agent123"},
		{"multiple spaces", "hello   world", "hello-world"},
		{"multiple hyphens", "hello---world", "hello-world"},
		{"leading hyphen removed", "-hello", "hello"},
		{"trailing hyphen removed", "hello-", "hello"},
		{"complex name", "My Agent 123!", "my-agent-123"},
		{"empty string", "", "resource"},
		{"only special chars", "@#$%", "resource"},
		{"mixed with numbers", "Agent_v2_Test", "agent-v2-test"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Slugify(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestPluralize(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"regular word", "agent", "agents"},
		{"word ending in s", "class", "classes"},
		{"word ending in x", "box", "boxes"},
		{"word ending in z", "quiz", "quizes"},
		{"word ending in ch", "watch", "watches"},
		{"word ending in sh", "brush", "brushes"},
		{"word ending in y (consonant)", "policy", "policies"},
		{"word ending in ay", "day", "days"},
		{"word ending in ey", "key", "keys"},
		{"word ending in oy", "boy", "boys"},
		{"word ending in uy", "guy", "guys"},
		{"uppercase preserved", "Agent", "Agents"},
		{"model", "model", "models"},
		{"function", "function", "functions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := Pluralize(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsVolumeTemplate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"volumetemplate", "volumetemplate", true},
		{"volume-template", "volume-template", true},
		{"vt abbreviation", "vt", true},
		{"agent", "agent", false},
		{"function", "function", false},
		{"job", "job", false},
		{"sandbox", "sandbox", false},
		{"empty", "", false},
		{"uppercase VT", "VT", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsVolumeTemplate(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetDeployDocURL(t *testing.T) {
	tests := []struct {
		name         string
		resourceType string
		expected     string
	}{
		{"agent", "agent", "https://docs.blaxel.ai/Agents/Deploy-an-agent"},
		{"function", "function", "https://docs.blaxel.ai/Functions/Deploy-a-function"},
		{"job", "job", "https://docs.blaxel.ai/Jobs/Deploy-a-job"},
		{"unknown", "unknown", "https://docs.blaxel.ai/Agents/Deploy-an-agent"},
		{"empty", "", "https://docs.blaxel.ai/Agents/Deploy-an-agent"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GetDeployDocURL(tt.resourceType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestModuleLanguage(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "module_language_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("detects python with pyproject.toml", func(t *testing.T) {
		dir := filepath.Join(tempDir, "python_pyproject")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[tool.poetry]"), 0644))

		result := ModuleLanguage(dir)
		assert.Equal(t, "python", result)
	})

	t.Run("detects python with requirements.txt", func(t *testing.T) {
		dir := filepath.Join(tempDir, "python_requirements")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("flask==2.0"), 0644))

		result := ModuleLanguage(dir)
		assert.Equal(t, "python", result)
	})

	t.Run("detects typescript with package.json", func(t *testing.T) {
		dir := filepath.Join(tempDir, "typescript")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "test"}`), 0644))

		result := ModuleLanguage(dir)
		assert.Equal(t, "typescript", result)
	})

	t.Run("detects go with go.mod", func(t *testing.T) {
		dir := filepath.Join(tempDir, "golang")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test"), 0644))

		result := ModuleLanguage(dir)
		assert.Equal(t, "go", result)
	})

	t.Run("detects python with main.py", func(t *testing.T) {
		dir := filepath.Join(tempDir, "python_main")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.py"), []byte("print('hello')"), 0644))

		result := ModuleLanguage(dir)
		assert.Equal(t, "python", result)
	})

	t.Run("returns empty for unknown", func(t *testing.T) {
		dir := filepath.Join(tempDir, "unknown")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "readme.md"), []byte("# Hello"), 0644))

		result := ModuleLanguage(dir)
		assert.Equal(t, "", result)
	})
}

func TestHasPythonEntryFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "python_entry_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("finds main.py", func(t *testing.T) {
		dir := filepath.Join(tempDir, "main_py")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.py"), []byte(""), 0644))

		assert.True(t, HasPythonEntryFile(dir))
	})

	t.Run("finds app.py", func(t *testing.T) {
		dir := filepath.Join(tempDir, "app_py")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "app.py"), []byte(""), 0644))

		assert.True(t, HasPythonEntryFile(dir))
	})

	t.Run("finds src/main.py", func(t *testing.T) {
		dir := filepath.Join(tempDir, "src_main")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "main.py"), []byte(""), 0644))

		assert.True(t, HasPythonEntryFile(dir))
	})

	t.Run("returns false when no entry file", func(t *testing.T) {
		dir := filepath.Join(tempDir, "no_entry")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "utils.py"), []byte(""), 0644))

		assert.False(t, HasPythonEntryFile(dir))
	})
}

func TestHasGoEntryFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "go_entry_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("finds main.go", func(t *testing.T) {
		dir := filepath.Join(tempDir, "main_go")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644))

		assert.True(t, HasGoEntryFile(dir))
	})

	t.Run("finds cmd/main.go", func(t *testing.T) {
		dir := filepath.Join(tempDir, "cmd_main")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "main.go"), []byte("package main"), 0644))

		assert.True(t, HasGoEntryFile(dir))
	})

	t.Run("returns false when no entry file", func(t *testing.T) {
		dir := filepath.Join(tempDir, "no_entry")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "utils.go"), []byte("package utils"), 0644))

		assert.False(t, HasGoEntryFile(dir))
	})
}

func TestHasTypeScriptEntryFile(t *testing.T) {
	// Create a temporary directory for testing
	tempDir, err := os.MkdirTemp("", "ts_entry_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	t.Run("finds index.js", func(t *testing.T) {
		dir := filepath.Join(tempDir, "index_js")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "index.js"), []byte(""), 0644))

		assert.True(t, HasTypeScriptEntryFile(dir))
	})

	t.Run("finds src/index.js", func(t *testing.T) {
		dir := filepath.Join(tempDir, "src_index")
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "src"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "src", "index.js"), []byte(""), 0644))

		assert.True(t, HasTypeScriptEntryFile(dir))
	})

	t.Run("finds package.json with start script", func(t *testing.T) {
		dir := filepath.Join(tempDir, "pkg_start")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"scripts": {"start": "node index.js"}}`), 0644))

		assert.True(t, HasTypeScriptEntryFile(dir))
	})

	t.Run("returns false when no entry file and no start script", func(t *testing.T) {
		dir := filepath.Join(tempDir, "no_entry")
		require.NoError(t, os.MkdirAll(dir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name": "test"}`), 0644))

		assert.False(t, HasTypeScriptEntryFile(dir))
	})
}

func TestFormatOperationId(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"simple operation", "GetAgent", []string{"get", "agent"}},
		{"list operation", "ListFunctions", []string{"list", "functions"}},
		{"delete operation", "DeleteModel", []string{"delete", "model"}},
		{"create operation", "CreateJob", []string{"create", "job"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatOperationId(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestBuildServerEnvWarning(t *testing.T) {
	t.Run("python warning", func(t *testing.T) {
		result := BuildServerEnvWarning("python", "agent")
		assert.Contains(t, result, "HOST")
		assert.Contains(t, result, "PORT")
		assert.Contains(t, result, "import os")
		assert.Contains(t, result, "os.environ.get")
	})

	t.Run("typescript warning", func(t *testing.T) {
		result := BuildServerEnvWarning("typescript", "function")
		assert.Contains(t, result, "HOST")
		assert.Contains(t, result, "PORT")
		assert.Contains(t, result, "process.env")
	})

	t.Run("go warning", func(t *testing.T) {
		result := BuildServerEnvWarning("go", "agent")
		assert.Contains(t, result, "HOST")
		assert.Contains(t, result, "PORT")
		assert.Contains(t, result, "os.Getenv")
	})

	t.Run("unknown language warning", func(t *testing.T) {
		result := BuildServerEnvWarning("unknown", "agent")
		assert.Contains(t, result, "HOST")
		assert.Contains(t, result, "PORT")
	})
}

func TestCheckServerEnvUsage(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "check_server_env_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Save and restore original working directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer os.Chdir(originalDir)

	// Change to temp directory
	err = os.Chdir(tempDir)
	require.NoError(t, err)

	t.Run("finds HOST in python file", func(t *testing.T) {
		pyDir := filepath.Join(tempDir, "py_with_host")
		require.NoError(t, os.MkdirAll(pyDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(pyDir, "app.py"), []byte(`
import os
host = os.environ.get("HOST", "0.0.0.0")
`), 0644))

		result := CheckServerEnvUsage("py_with_host", "python")
		assert.True(t, result)
	})

	t.Run("finds PORT in typescript file", func(t *testing.T) {
		tsDir := filepath.Join(tempDir, "ts_with_port")
		require.NoError(t, os.MkdirAll(tsDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(tsDir, "index.ts"), []byte(`
const port = process.env.PORT || 8080;
`), 0644))

		result := CheckServerEnvUsage("ts_with_port", "typescript")
		assert.True(t, result)
	})

	t.Run("finds BL_SERVER_HOST in go file", func(t *testing.T) {
		goDir := filepath.Join(tempDir, "go_with_bl_host")
		require.NoError(t, os.MkdirAll(goDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(goDir, "main.go"), []byte(`
package main
import "os"
func main() {
    host := os.Getenv("BL_SERVER_HOST")
}
`), 0644))

		result := CheckServerEnvUsage("go_with_bl_host", "go")
		assert.True(t, result)
	})

	t.Run("returns false when no patterns found", func(t *testing.T) {
		emptyDir := filepath.Join(tempDir, "no_patterns")
		require.NoError(t, os.MkdirAll(emptyDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(emptyDir, "app.py"), []byte(`
def hello():
    return "Hello, World!"
`), 0644))

		result := CheckServerEnvUsage("no_patterns", "python")
		assert.False(t, result)
	})

	t.Run("returns false for empty directory", func(t *testing.T) {
		emptyDir := filepath.Join(tempDir, "empty_dir")
		require.NoError(t, os.MkdirAll(emptyDir, 0755))

		result := CheckServerEnvUsage("empty_dir", "python")
		assert.False(t, result)
	})

	t.Run("returns false for non-existent directory", func(t *testing.T) {
		result := CheckServerEnvUsage("non_existent_dir", "python")
		assert.False(t, result)
	})

	t.Run("uses all extensions for unknown language", func(t *testing.T) {
		unknownDir := filepath.Join(tempDir, "unknown_lang")
		require.NoError(t, os.MkdirAll(unknownDir, 0755))
		require.NoError(t, os.WriteFile(filepath.Join(unknownDir, "main.rb"), []byte(`
host = ENV['HOST'] || 'localhost'
`), 0644))

		result := CheckServerEnvUsage("unknown_lang", "")
		assert.True(t, result)
	})

	t.Run("skips node_modules directory", func(t *testing.T) {
		skipDir := filepath.Join(tempDir, "with_node_modules")
		require.NoError(t, os.MkdirAll(filepath.Join(skipDir, "node_modules"), 0755))
		require.NoError(t, os.WriteFile(filepath.Join(skipDir, "node_modules", "lib.js"), []byte(`
const port = process.env.PORT;
`), 0644))
		require.NoError(t, os.WriteFile(filepath.Join(skipDir, "app.js"), []byte(`
console.log("No env usage");
`), 0644))

		result := CheckServerEnvUsage("with_node_modules", "typescript")
		assert.False(t, result)
	})
}

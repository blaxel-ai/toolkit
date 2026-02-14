package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeployCmd(t *testing.T) {
	cmd := DeployCmd()

	assert.Equal(t, "deploy", cmd.Use)
	assert.Contains(t, cmd.Aliases, "d")
	assert.Contains(t, cmd.Aliases, "dp")
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags exist
	flag := cmd.Flags().Lookup("name")
	assert.NotNil(t, flag)
	assert.Equal(t, "n", flag.Shorthand)

	dryRunFlag := cmd.Flags().Lookup("dryrun")
	assert.NotNil(t, dryRunFlag)

	recursiveFlag := cmd.Flags().Lookup("recursive")
	assert.NotNil(t, recursiveFlag)
	assert.Equal(t, "r", recursiveFlag.Shorthand)

	directoryFlag := cmd.Flags().Lookup("directory")
	assert.NotNil(t, directoryFlag)
	assert.Equal(t, "d", directoryFlag.Shorthand)

	envFileFlag := cmd.Flags().Lookup("env-file")
	assert.NotNil(t, envFileFlag)
	assert.Equal(t, "e", envFileFlag.Shorthand)

	secretsFlag := cmd.Flags().Lookup("secrets")
	assert.NotNil(t, secretsFlag)
	assert.Equal(t, "s", secretsFlag.Shorthand)

	skipBuildFlag := cmd.Flags().Lookup("skip-build")
	assert.NotNil(t, skipBuildFlag)

	yesFlag := cmd.Flags().Lookup("yes")
	assert.NotNil(t, yesFlag)
	assert.Equal(t, "y", yesFlag.Shorthand)
}

func TestDeploymentStruct(t *testing.T) {
	d := Deployment{
		dir:    ".blaxel",
		name:   "test-app",
		folder: "src",
		cwd:    "/tmp/test",
	}

	assert.Equal(t, ".blaxel", d.dir)
	assert.Equal(t, "test-app", d.name)
	assert.Equal(t, "src", d.folder)
	assert.Equal(t, "/tmp/test", d.cwd)
}

func TestDeploymentIgnoredPathsDefault(t *testing.T) {
	// Create a temp directory without .blaxelignore
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	d := Deployment{
		cwd: tempDir,
	}

	ignored := d.IgnoredPaths()

	// Should return default ignored paths
	assert.Contains(t, ignored, ".git")
	assert.Contains(t, ignored, "node_modules")
	assert.Contains(t, ignored, ".venv")
	assert.Contains(t, ignored, "__pycache__")
	assert.Contains(t, ignored, ".blaxel")
}

func TestDeploymentIgnoredPathsFromFile(t *testing.T) {
	// Create a temp directory with .blaxelignore
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create .blaxelignore file
	ignoreContent := `
# This is a comment
dist
build
*.log  # inline comment
`
	err = os.WriteFile(filepath.Join(tempDir, ".blaxelignore"), []byte(ignoreContent), 0644)
	require.NoError(t, err)

	d := Deployment{
		cwd: tempDir,
	}

	ignored := d.IgnoredPaths()

	assert.Contains(t, ignored, "dist")
	assert.Contains(t, ignored, "build")
	assert.Contains(t, ignored, "*.log")
}

func TestDeploymentShouldIgnorePath(t *testing.T) {
	d := Deployment{
		cwd: "/home/user/project",
	}

	ignoredPaths := []string{".git", "node_modules", "dist"}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"git directory", "/home/user/project/.git", true},
		{"nested git", "/home/user/project/subdir/.git/", true},
		{"node_modules", "/home/user/project/node_modules", true},
		{"nested node_modules", "/home/user/project/packages/node_modules/file.js", true},
		{"dist folder", "/home/user/project/dist", true},
		{"regular file", "/home/user/project/src/main.go", false},
		{"similar name", "/home/user/project/src/distribution", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.shouldIgnorePath(tt.path, ignoredPaths)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestResultKinds(t *testing.T) {
	// Test that core.Result struct works correctly for different kinds
	tests := []struct {
		name       string
		kind       string
		apiVersion string
	}{
		{"Agent", "Agent", "blaxel.ai/v1alpha1"},
		{"Function", "Function", "blaxel.ai/v1alpha1"},
		{"Job", "Job", "blaxel.ai/v1alpha1"},
		{"Sandbox", "Sandbox", "blaxel.ai/v1alpha1"},
		{"VolumeTemplate", "VolumeTemplate", "blaxel.ai/v1alpha1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := core.Result{
				ApiVersion: tt.apiVersion,
				Kind:       tt.kind,
				Metadata: map[string]interface{}{
					"name": "test-resource",
				},
				Spec: map[string]interface{}{},
			}

			assert.Equal(t, tt.apiVersion, result.ApiVersion)
			assert.Equal(t, tt.kind, result.Kind)

			metadata := result.Metadata.(map[string]interface{})
			assert.Equal(t, "test-resource", metadata["name"])
		})
	}
}

func TestProgressReader(t *testing.T) {
	// Create test data
	data := []byte("test data for progress tracking")
	reader := &progressReader{
		reader: nil,
		total:  int64(len(data)),
		read:   0,
		callback: func(bytesUploaded, totalBytes int64) {
			// Callback called
		},
	}

	assert.Equal(t, int64(len(data)), reader.total)
	assert.Equal(t, int64(0), reader.read)
}

func TestWithRecursiveOption(t *testing.T) {
	opts := &applyOptions{
		recursive: false,
	}

	fn := WithRecursive(true)
	fn(opts)

	assert.True(t, opts.recursive)
}

func TestIsBlaxelErrorDeploy(t *testing.T) {
	t.Run("not a blaxel error", func(t *testing.T) {
		var apiErr *blaxelErrorType
		err := assert.AnError

		result := isBlaxelErrorDeployHelper(err, &apiErr)
		assert.False(t, result)
	})
}

// Helper type for testing
type blaxelErrorType struct {
	StatusCode int
	Message    string
}

func (e *blaxelErrorType) Error() string {
	return e.Message
}

func isBlaxelErrorDeployHelper(err error, apiErr **blaxelErrorType) bool {
	if e, ok := err.(*blaxelErrorType); ok {
		*apiErr = e
		return true
	}
	return false
}

func TestDeploymentGenerateNameExtraction(t *testing.T) {
	tests := []struct {
		name     string
		cwd      string
		folder   string
		expected string
	}{
		{"unix path no folder", "/home/user/my-project", "", "my-project"},
		{"unix path with folder", "/home/user/project", "src", "src"},
		{"unix path trailing slash style", "/home/user/project/subdir", "", "subdir"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filepath.Base(filepath.Join(tt.cwd, tt.folder))
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestToArchivePath(t *testing.T) {
	// Verify that toArchivePath converts backslashes to forward slashes
	// This ensures archive entries always use forward slashes regardless of OS
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"unix path unchanged", "src/main.go", "src/main.go"},
		{"windows path converted", "src\\main.go", "src/main.go"},
		{"nested windows path", "src\\pkg\\utils\\helper.go", "src/pkg/utils/helper.go"},
		{"root file unchanged", "main.go", "main.go"},
		{"mixed separators", "src/pkg\\utils/helper.go", "src/pkg/utils/helper.go"},
		{"windows absolute prefix", "C:\\Users\\foo\\src\\main.go", "C:/Users/foo/src/main.go"},
		{"directory with trailing backslash", "src\\pkg\\", "src/pkg/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toArchivePath(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeploymentShouldIgnorePathWithDirectories(t *testing.T) {
	d := Deployment{
		cwd: "/home/user/project",
	}

	// Note: shouldIgnorePath uses string matching, not glob patterns
	ignoredPaths := []string{"logs", "build", "tmp"}

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"logs directory", "/home/user/project/logs", true},
		{"logs prefix start", "/home/user/project/logs/error.log", true},
		{"build dir", "/home/user/project/build/output", true},
		{"tmp directory", "/home/user/project/tmp", true},
		{"go file", "/home/user/project/main.go", false},
		{"js file", "/home/user/project/app.js", false},
		{"nested logs", "/home/user/project/src/logs/debug.log", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := d.shouldIgnorePath(tt.path, ignoredPaths)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeploymentIgnoredPathsWithSubfolder(t *testing.T) {
	// Create a temp directory with .blaxelignore at root
	// The IgnoredPaths function looks for .blaxelignore in cwd, not folder
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create subfolder
	subDir := filepath.Join(tempDir, "subfolder")
	require.NoError(t, os.MkdirAll(subDir, 0755))

	// Create .blaxelignore at root (where IgnoredPaths looks for it)
	ignoreContent := "custom_ignore\n"
	err = os.WriteFile(filepath.Join(tempDir, ".blaxelignore"), []byte(ignoreContent), 0644)
	require.NoError(t, err)

	d := Deployment{
		cwd:    tempDir,
		folder: "subfolder",
	}

	ignored := d.IgnoredPaths()

	assert.Contains(t, ignored, "custom_ignore")
}

func TestDeploymentWithVolumeTemplateConfig(t *testing.T) {
	// Create a temp directory with blaxel.toml for volume template
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create blaxel.toml
	tomlContent := `name = "my-volume"
type = "volumetemplate"
`
	err = os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(filepath.Join(tempDir, "blaxel.toml"))
	assert.NoError(t, err)
}

func TestDeploymentReadBlaxelToml(t *testing.T) {
	// Create a temp directory with blaxel.toml
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create blaxel.toml
	tomlContent := `name = "test-agent"
type = "agent"
workspace = "test-workspace"

[entrypoint]
prod = "python main.py"
dev = "python main.py --reload"
`
	err = os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644)
	require.NoError(t, err)

	// Save current directory and change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer os.Chdir(originalDir)

	// Reset config before reading
	core.ResetConfig()

	// Read and verify config
	core.ReadConfigToml("", false)
	config := core.GetConfig()

	assert.Equal(t, "test-agent", config.Name)
	assert.Equal(t, "agent", config.Type)
	assert.Equal(t, "python main.py", config.Entrypoint.Production)
	assert.Equal(t, "python main.py --reload", config.Entrypoint.Development)
}

func TestDeploymentIgnoredPathsEmptyLines(t *testing.T) {
	// Create a temp directory with .blaxelignore containing empty lines
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create .blaxelignore file with empty lines and whitespace
	ignoreContent := `

dist

# comment

build

`
	err = os.WriteFile(filepath.Join(tempDir, ".blaxelignore"), []byte(ignoreContent), 0644)
	require.NoError(t, err)

	d := Deployment{
		cwd: tempDir,
	}

	ignored := d.IgnoredPaths()

	// Should contain dist and build but not empty strings
	assert.Contains(t, ignored, "dist")
	assert.Contains(t, ignored, "build")

	// Count how many items in the list
	nonEmptyCount := 0
	for _, item := range ignored {
		if item != "" && item != "#" {
			nonEmptyCount++
		}
	}
	assert.Greater(t, nonEmptyCount, 0)
}

func TestProgressReaderCallback(t *testing.T) {
	callbackCalled := false
	var lastBytesUploaded int64
	var lastTotalBytes int64

	reader := &progressReader{
		reader: nil,
		total:  100,
		read:   0,
		callback: func(bytesUploaded, totalBytes int64) {
			callbackCalled = true
			lastBytesUploaded = bytesUploaded
			lastTotalBytes = totalBytes
		},
	}

	// Simulate progress
	reader.read = 50
	reader.callback(50, 100)

	assert.True(t, callbackCalled)
	assert.Equal(t, int64(50), lastBytesUploaded)
	assert.Equal(t, int64(100), lastTotalBytes)
}

func TestDeploymentWithJobConfig(t *testing.T) {
	// Create a temp directory with blaxel.toml for job
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create blaxel.toml for job
	tomlContent := `name = "my-job"
type = "job"
workspace = "test-workspace"

[entrypoint]
prod = "python job.py"
`
	err = os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644)
	require.NoError(t, err)

	// Save current directory and change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer os.Chdir(originalDir)

	core.ResetConfig()
	core.ReadConfigToml("", false)
	config := core.GetConfig()

	assert.Equal(t, "my-job", config.Name)
	assert.Equal(t, "job", config.Type)
}

func TestDeploymentWithFunctionConfig(t *testing.T) {
	// Create a temp directory with blaxel.toml for function
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create blaxel.toml for function
	tomlContent := `name = "my-function"
type = "function"
workspace = "test-workspace"
`
	err = os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644)
	require.NoError(t, err)

	// Save current directory and change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer os.Chdir(originalDir)

	core.ResetConfig()
	core.ReadConfigToml("", false)
	config := core.GetConfig()

	assert.Equal(t, "my-function", config.Name)
	assert.Equal(t, "function", config.Type)
}

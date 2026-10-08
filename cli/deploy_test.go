package cli

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/cli/deploy"
	"github.com/blaxel-ai/toolkit/cli/server"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

func TestDeploymentDryRunStructuredOutputJSON(t *testing.T) {
	deployment := Deployment{
		blaxelDeployments: []core.Result{
			{
				ApiVersion: "blaxel.ai/v1alpha1",
				Kind:       "Sandbox",
				Metadata: map[string]interface{}{
					"name": "pm1729-dryrun",
				},
				Spec: map[string]interface{}{
					"runtime": map[string]interface{}{
						"image": "ubuntu:latest",
					},
				},
			},
		},
	}

	output, err := deployment.renderDryRunStructuredOutput("json", true)
	require.NoError(t, err)

	var payload struct {
		DryRun    bool          `json:"dryRun"`
		Resources []core.Result `json:"resources"`
		Files     []dryRunFile  `json:"files"`
	}
	require.NoError(t, json.Unmarshal(output, &payload))
	assert.True(t, payload.DryRun)
	require.Len(t, payload.Resources, 1)
	assert.Equal(t, "Sandbox", payload.Resources[0].Kind)
	assert.Empty(t, payload.Files)
}

func TestDeploymentDryRunStructuredOutputRejectsUnknownFormat(t *testing.T) {
	deployment := Deployment{}

	output, err := deployment.renderDryRunStructuredOutput("table", true)

	require.Error(t, err)
	assert.Nil(t, output)
	assert.Contains(t, err.Error(), "unsupported dry-run output format")
}

func TestGenerateApplicationDeploymentUsesRevisionSpec(t *testing.T) {
	tempDir := t.TempDir()
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	defer func() { require.NoError(t, os.Chdir(originalDir)) }()

	tomlContent := `name = "my-app"
type = "application"
workspace = "test-workspace"
image = "registry.example.com/my-app:latest"
memory = 4096
port = 8080
region = "us-pdx-1"

[env]
FOO = "bar"
`
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644))
	require.NoError(t, os.Chdir(tempDir))
	core.ResetConfig()
	core.ReadConfigToml("", true)

	deployment := Deployment{name: "my-app", cwd: tempDir}
	result := deployment.GenerateDeployment(false)

	assert.Equal(t, "Application", result.Kind)
	spec := result.Spec.(map[string]interface{})
	assert.Equal(t, true, spec["enabled"])
	assert.Equal(t, "us-pdx-1", spec["region"])
	assert.Equal(t, 8080, spec["port"])
	assert.NotContains(t, spec, "image")
	assert.NotContains(t, spec, "memory")
	assert.NotContains(t, spec, "envs")

	revisions := spec["revisions"].([]interface{})
	require.Len(t, revisions, 1)
	revision := revisions[0].(map[string]interface{})
	assert.Equal(t, "registry.example.com/my-app:latest", revision["image"])
	assert.Equal(t, 4096, revision["memory"])
	assert.Len(t, revision["envs"], 1)
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
	defer func() { _ = os.RemoveAll(tempDir) }()

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
	assert.Contains(t, ignored, ".env*")

	matcher, err := newIgnoredPathMatcher(tempDir, ignored)
	require.NoError(t, err)
	for _, name := range []string{".env", ".env.local", ".env.production"} {
		matched, err := matcher.matches(filepath.Join(tempDir, name))
		require.NoError(t, err)
		assert.True(t, matched, "%s must not be included in deployment archives", name)
	}
}

func TestDeploymentIgnoredPathsFromFile(t *testing.T) {
	// Create a temp directory with .blaxelignore
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

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
	cwd := filepath.FromSlash("/home/user/project")
	ignoredPaths := []string{".git", "node_modules", "dist"}
	matcher, err := newIgnoredPathMatcher(cwd, ignoredPaths)
	require.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"git directory", filepath.Join(cwd, ".git"), true},
		{"nested git", filepath.Join(cwd, "subdir", ".git") + string(filepath.Separator), true},
		{"node_modules", filepath.Join(cwd, "node_modules"), true},
		{"nested node_modules", filepath.Join(cwd, "packages", "node_modules", "file.js"), true},
		{"dist folder", filepath.Join(cwd, "dist"), true},
		{"regular file", filepath.Join(cwd, "src", "main.go"), false},
		{"similar name", filepath.Join(cwd, "src", "distribution"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := matcher.matches(tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeploymentShouldIgnoreGlobPatterns(t *testing.T) {
	cwd := filepath.FromSlash("/home/user/project")
	matcher, err := newIgnoredPathMatcher(cwd, []string{
		"**/*.test.ts",
		"**/.env.*",
		"!sandbox/keep.test.ts",
	})
	require.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"root test file", filepath.Join(cwd, "app.test.ts"), true},
		{"nested test file", filepath.Join(cwd, "sandbox", "src", "app.test.ts"), true},
		{"nested environment file", filepath.Join(cwd, "sandbox", ".env.local"), true},
		{"negated test file", filepath.Join(cwd, "sandbox", "keep.test.ts"), false},
		{"regular source file", filepath.Join(cwd, "sandbox", "src", "app.ts"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := matcher.matches(tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeploymentShouldIgnoreAnchoredLiteralPath(t *testing.T) {
	cwd := filepath.FromSlash("/home/user/project")
	matcher, err := newIgnoredPathMatcher(cwd, []string{"./infra"})
	require.NoError(t, err)

	rootInfra, err := matcher.matches(filepath.Join(cwd, "infra", "main.tf"))
	require.NoError(t, err)
	assert.True(t, rootInfra)

	nestedInfra, err := matcher.matches(filepath.Join(cwd, "sandbox", "infra", "main.tf"))
	require.NoError(t, err)
	assert.False(t, nestedInfra)
}

func TestNewIgnoredPathMatcherRejectsInvalidPattern(t *testing.T) {
	_, err := newIgnoredPathMatcher("/home/user/project", []string{"[invalid"})
	require.ErrorContains(t, err, "invalid .blaxelignore pattern")
}

func TestIgnoredPathMatcherCanSkipIgnoredDirectory(t *testing.T) {
	withoutExclusions, err := newIgnoredPathMatcher("/home/user/project", []string{"node_modules"})
	require.NoError(t, err)
	assert.True(t, withoutExclusions.canSkipIgnoredDirectory())

	withExclusions, err := newIgnoredPathMatcher("/home/user/project", []string{
		"node_modules",
		"!node_modules/keep/package.json",
	})
	require.NoError(t, err)
	assert.False(t, withExclusions.canSkipIgnoredDirectory())
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
		total: int64(len(data)),
		read:  0,
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
	cwd := filepath.FromSlash("/home/user/project")
	ignoredPaths := []string{"logs", "build", "tmp"}
	matcher, err := newIgnoredPathMatcher(cwd, ignoredPaths)
	require.NoError(t, err)

	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{"logs directory", filepath.Join(cwd, "logs"), true},
		{"logs prefix start", filepath.Join(cwd, "logs", "error.log"), true},
		{"build dir", filepath.Join(cwd, "build", "output"), true},
		{"tmp directory", filepath.Join(cwd, "tmp"), true},
		{"go file", filepath.Join(cwd, "main.go"), false},
		{"js file", filepath.Join(cwd, "app.js"), false},
		{"nested logs", filepath.Join(cwd, "src", "logs", "debug.log"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := matcher.matches(tt.path)
			require.NoError(t, err)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDeploymentIgnoredPathsWithSubfolder(t *testing.T) {
	// Create a temp directory with .blaxelignore at root
	// The IgnoredPaths function looks for .blaxelignore in cwd, not folder
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

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
	defer func() { _ = os.RemoveAll(tempDir) }()

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

func TestVolumeTemplateTarExcludesBlaxelToml(t *testing.T) {
	// Create a temp directory simulating a volume-template project
	tempDir, err := os.MkdirTemp("", "deploy_vt_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create blaxel.toml (should be excluded from archive)
	tomlContent := `name = "my-volume"
type = "volume-template"
`
	err = os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644)
	require.NoError(t, err)

	// Create actual content files (should be included)
	err = os.WriteFile(filepath.Join(tempDir, "data.txt"), []byte("some data"), 0644)
	require.NoError(t, err)
	err = os.MkdirAll(filepath.Join(tempDir, "subdir"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "subdir", "nested.txt"), []byte("nested data"), 0644)
	require.NoError(t, err)

	// Save current directory and change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(originalDir) }()

	// Set up config as volume-template
	core.ResetConfig()
	core.ReadConfigToml("", false)
	config := core.GetConfig()
	assert.Equal(t, "volume-template", config.Type)

	// Create the tar archive
	d := Deployment{
		cwd: tempDir,
	}
	err = d.Tar()
	require.NoError(t, err)

	// Read the tar and collect all file names
	tarFile, err := os.Open(d.archive.Name())
	require.NoError(t, err)
	defer func() { _ = tarFile.Close() }()

	tarReader := tar.NewReader(tarFile)
	var archivedFiles []string
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		archivedFiles = append(archivedFiles, header.Name)
	}

	// blaxel.toml must NOT be in the archive
	assert.NotContains(t, archivedFiles, "blaxel.toml")

	// data files must be present
	assert.Contains(t, archivedFiles, "data.txt")
	assert.Contains(t, archivedFiles, "subdir/nested.txt")
}

func TestVolumeTemplateTarWithDirectoryExcludesBlaxelToml(t *testing.T) {
	// Create a temp directory simulating a volume-template with directory="app"
	tempDir, err := os.MkdirTemp("", "deploy_vt_dir_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

	// Create blaxel.toml at root with directory config
	tomlContent := `name = "my-volume"
type = "volume-template"
directory = "app"
`
	err = os.WriteFile(filepath.Join(tempDir, "blaxel.toml"), []byte(tomlContent), 0644)
	require.NoError(t, err)

	// Create app directory with content and a blaxel.toml (should still be excluded)
	err = os.MkdirAll(filepath.Join(tempDir, "app"), 0755)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "app", "index.html"), []byte("<html></html>"), 0644)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(tempDir, "app", "blaxel.toml"), []byte("should not be included"), 0644)
	require.NoError(t, err)

	// Save current directory and change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer func() { _ = os.Chdir(originalDir) }()

	// Set up config as volume-template
	core.ResetConfig()
	core.ReadConfigToml("", false)
	config := core.GetConfig()
	assert.Equal(t, "volume-template", config.Type)
	assert.Equal(t, "app", config.Directory)

	// Create the tar archive
	d := Deployment{
		cwd: tempDir,
	}
	err = d.Tar()
	require.NoError(t, err)

	// Read the tar and collect all file names
	tarFile, err := os.Open(d.archive.Name())
	require.NoError(t, err)
	defer func() { _ = tarFile.Close() }()

	tarReader := tar.NewReader(tarFile)
	var archivedFiles []string
	for {
		header, err := tarReader.Next()
		if err != nil {
			break
		}
		archivedFiles = append(archivedFiles, header.Name)
	}

	// blaxel.toml must NOT be in the archive
	assert.NotContains(t, archivedFiles, "blaxel.toml")

	// index.html must be present
	assert.Contains(t, archivedFiles, "index.html")
}

func TestDeploymentReadBlaxelToml(t *testing.T) {
	// Create a temp directory with blaxel.toml
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

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
	defer func() { _ = os.Chdir(originalDir) }()

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
	defer func() { _ = os.RemoveAll(tempDir) }()

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

	cb := func(bytesUploaded, totalBytes int64) {
		callbackCalled = true
		lastBytesUploaded = bytesUploaded
		lastTotalBytes = totalBytes
	}

	reader := &progressReader{
		read: 0,
	}

	// Simulate progress
	reader.read = 50
	cb(50, 100)
	_ = reader

	assert.True(t, callbackCalled)
	assert.Equal(t, int64(50), lastBytesUploaded)
	assert.Equal(t, int64(100), lastTotalBytes)
}

func TestDeploymentWithJobConfig(t *testing.T) {
	// Create a temp directory with blaxel.toml for job
	tempDir, err := os.MkdirTemp("", "deploy_test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tempDir) }()

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
	defer func() { _ = os.Chdir(originalDir) }()

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
	defer func() { _ = os.RemoveAll(tempDir) }()

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
	defer func() { _ = os.Chdir(originalDir) }()

	core.ResetConfig()
	core.ReadConfigToml("", false)
	config := core.GetConfig()

	assert.Equal(t, "my-function", config.Name)
	assert.Equal(t, "function", config.Type)
}

func TestDockerfileProvidesSandboxAPI(t *testing.T) {
	tests := []struct {
		name       string
		dockerfile string
		want       bool
	}{
		{
			name:       "blaxel sandbox base image",
			dockerfile: "FROM ghcr.io/blaxel-ai/sandbox:latest\n",
			want:       true,
		},
		{
			name:       "blaxel sandbox base image with platform flag",
			dockerfile: "FROM --platform=linux/amd64 ghcr.io/blaxel-ai/sandbox:latest\n",
			want:       true,
		},
		{
			name: "direct multi-stage copy from the image",
			dockerfile: `FROM debian:bookworm-slim
COPY --from=ghcr.io/blaxel-ai/sandbox:latest /sandbox-api /usr/local/bin/sandbox-api
`,
			want: true,
		},
		{
			name: "copy from a named build stage",
			dockerfile: `FROM --platform=linux/amd64 ghcr.io/blaxel-ai/sandbox:latest AS blaxel-sandbox
FROM --platform=linux/amd64 node:22-bookworm-slim
COPY --from=blaxel-sandbox /sandbox-api /usr/local/bin/sandbox-api
`,
			want: true,
		},
		{
			name: "stage names are case-insensitive",
			dockerfile: `FROM ghcr.io/blaxel-ai/sandbox:latest AS Blaxel-Sandbox
FROM debian:bookworm-slim
COPY --from=BLAXEL-SANDBOX /sandbox-api /usr/local/bin/sandbox-api
`,
			want: true,
		},
		{
			name: "copy from an indexed build stage",
			dockerfile: `FROM ghcr.io/blaxel-ai/sandbox:latest
FROM debian:bookworm-slim
COPY --from=0 /sandbox-api /usr/local/bin/sandbox-api
`,
			want: true,
		},
		{
			name: "copy from a stage built on a sandbox stage",
			dockerfile: `FROM ghcr.io/blaxel-ai/sandbox:latest AS base
FROM base AS tools
FROM debian:bookworm-slim
COPY --from=tools /sandbox-api /usr/local/bin/sandbox-api
`,
			want: true,
		},
		{
			name:       "plain image without the binary",
			dockerfile: "FROM debian:bookworm-slim\nRUN apt-get update\n",
			want:       false,
		},
		{
			name: "sandbox stage that is never copied from",
			dockerfile: `FROM ghcr.io/blaxel-ai/sandbox:latest AS unused
FROM debian:bookworm-slim
COPY --from=somewhere-else /thing /thing
`,
			want: false,
		},
		{
			name:       "empty dockerfile",
			dockerfile: "",
			want:       false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, dockerfileProvidesSandboxAPI(tt.dockerfile))
		})
	}
}

func TestDeployedStatusIsFinal(t *testing.T) {
	known := func(rev string) rolloutBaseline {
		return rolloutBaseline{revision: rev, deployedRevision: rev, known: true}
	}
	// r0 deployed, r1 still rolling out when the apply happened.
	inFlight := rolloutBaseline{revision: "r1", deployedRevision: "r0", known: true}
	unknown := rolloutBaseline{}
	tests := []struct {
		name             string
		autoGenerated    bool
		sawRolloutStatus bool
		baseline         rolloutBaseline
		deployedRevision string
		want             bool
	}{
		{"skip-build accepts DEPLOYED immediately", false, false, known("r0"), "r0", true},
		{"build: DEPLOYED of the previous revision is ignored", true, false, known("r0"), "r0", false},
		{"build: DEPLOYED of the previous revision is ignored even after a rollout status", true, true, known("r0"), "r0", false},
		{"build: DEPLOYED of a new revision is final without a rollout status", true, false, known("r0"), "r1", true},
		{"build: first deployment of a new resource is final", true, false, known(""), "r1", true},
		{"build: in-flight baseline still reports the old deployed revision", true, false, inFlight, "r0", false},
		{"build: in-flight rollout finishing is not this apply", true, true, inFlight, "r1", false},
		{"build: in-flight baseline accepts the revision created by this apply", true, false, inFlight, "r2", true},
		{"build: no revision on events falls back to the rollout status (ignored)", true, false, known("r0"), "", false},
		{"build: no revision on events falls back to the rollout status (final)", true, true, known("r0"), "", true},
		{"build: unreadable baseline falls back to the rollout status", true, false, unknown, "r1", false},
		{"build: unreadable baseline accepts DEPLOYED after a rollout status", true, true, unknown, "r1", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := deployedStatusIsFinal(tt.autoGenerated, tt.sawRolloutStatus, tt.baseline, tt.deployedRevision)
			if got != tt.want {
				t.Fatalf("deployedStatusIsFinal(%v, %v, %+v, %q) = %v, want %v", tt.autoGenerated, tt.sawRolloutStatus, tt.baseline, tt.deployedRevision, got, tt.want)
			}
		})
	}
}

func TestEventRevisions(t *testing.T) {
	events := `[
		{"type":"ai.blaxel.controlplane.deployment.succeeded","status":"DEPLOYED","time":"2026-09-23T23:00:00Z","revision":"r0"},
		{"type":"ai.blaxel.controlplane.api.update","status":"UPDATED","time":"2026-09-23T23:05:00Z","revision":""},
		{"type":"ai.blaxel.controlplane.deployment.created","status":"BUILT","time":"2026-09-23T23:05:10Z","revision":"r1"},
		{"type":"ai.blaxel.controlplane.deployment.ready","status":"DEPLOYED","time":"2026-09-23T22:59:00Z","revision":"stale"}
	]`
	latest, deployed := eventRevisions(json.RawMessage(events))
	if latest != "r1" {
		t.Fatalf("latest = %q, want r1", latest)
	}
	if deployed != "r0" {
		t.Fatalf("deployed = %q, want r0", deployed)
	}

	latest, deployed = eventRevisions(json.RawMessage(`[]`))
	if latest != "" || deployed != "" {
		t.Fatalf("empty events: got %q %q", latest, deployed)
	}
	latest, deployed = eventRevisions(json.RawMessage(`not json`))
	if latest != "" || deployed != "" {
		t.Fatalf("malformed events: got %q %q", latest, deployed)
	}
}

const noSpecificCause = "No more specific cause was returned."

const badRunDeployMessage = `#5 [2/2] RUN echo BAD_RUN >&2; exit 42
#5 0.064 BAD_RUN
#5 ERROR: process "/bin/sh -c echo BAD_RUN >&2; exit 42" did not complete successfully: exit code: 42
Dockerfile:2
--------------------
   1 | FROM alpine:3.22
   2 | >>> RUN echo BAD_RUN >&2; exit 42
--------------------
error: failed to solve: process "/bin/sh -c echo BAD_RUN >&2; exit 42" did not complete successfully: exit code: 42`

// A failing command line holding a URL is infrastructure-looking; the rest of
// the build log must still be reported.
const urlRunDeployMessage = `#5 [2/2] RUN wget -q https://example.invalid/x
#5 ERROR: process "/bin/sh -c wget -q https://example.invalid/x" did not complete successfully: exit code: 4
Dockerfile:2
--------------------
   1 | FROM alpine:3.22
   2 | >>> RUN wget -q https://example.invalid/x
--------------------
error: failed to solve: process "/bin/sh -c wget -q https://example.invalid/x" did not complete successfully: exit code: 4`

const missingBaseDeployMessage = `#2 ERROR: docker.io/library/alpine:missing-tag: not found
Dockerfile:1
--------------------
   1 | >>> FROM alpine:missing-tag
--------------------
error: failed to solve: alpine:missing-tag: not found`

func deployTestClient(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	original, workspace := core.GetClient(), core.GetWorkspace()
	server := httptest.NewServer(handler)
	client := blaxel.NewClient(option.WithBaseURL(server.URL+"/"), option.WithAPIKey("test"), option.WithMaxRetries(0))
	core.SetClient(&client)
	core.SetWorkspace("test-workspace")
	core.RegisterResourceOperations(context.Background())
	t.Cleanup(func() {
		core.SetClient(original)
		core.SetWorkspace(workspace)
		core.RegisterResourceOperations(context.Background())
		server.Close()
	})
}

func deployEventJSON(t *testing.T, eventType, revision, message string, at time.Time) json.RawMessage {
	t.Helper()
	data, err := json.Marshal([]deploy.Event{{Type: "ai.blaxel.controlplane." + eventType, Revision: revision, Status: "FAILED", Message: message, Time: at.UTC().Format(time.RFC3339Nano)}})
	require.NoError(t, err)
	return data
}

func captureDeployStdout(t *testing.T, fn func()) string {
	t.Helper()
	original := os.Stdout
	file, err := os.CreateTemp(t.TempDir(), "stdout")
	require.NoError(t, err)
	os.Stdout = file
	defer func() { os.Stdout = original; _ = file.Close() }()
	fn()
	_, err = file.Seek(0, 0)
	require.NoError(t, err)
	data, err := io.ReadAll(file)
	require.NoError(t, err)
	return string(data)
}

func TestDeployFailureDiagnostics(t *testing.T) {
	cases := []struct{ name, eventType, message, logs, code, cause, step string }{
		{"failed RUN", "buildimage.failed", badRunDeployMessage, "", "BUILD_FAILED", "Dockerfile RUN failed with exit code 42", "Dockerfile:2 RUN echo BAD_RUN >&2; exit 42"},
		{"URL in the failing command", "buildimage.failed", urlRunDeployMessage, "", "BUILD_FAILED", "Dockerfile RUN failed with exit code 4", "Dockerfile:2 RUN"},
		{"missing base", "buildimage.failed", missingBaseDeployMessage, "", "BUILD_FAILED", "docker.io/library/alpine:missing-tag: not found", "Dockerfile:1 FROM alpine:missing-tag"},
		{"generic rollout with exit evidence", "deployment.failed", "Deployment has failed", "STARTUP_EXIT\nApplication exited with 0x2a00 (exit code: 42)", "ROLLOUT_FAILED", noSpecificCause, ""},
		{"generic rollout empty logs", "deployment.failed", "Deployment has failed", "", "ROLLOUT_FAILED", noSpecificCause, ""},
		// The step must survive an evidence tail cut off by the 20 line bound.
		{"failed RUN with long output", "buildimage.failed", badRunDeployMessage + "\n" + strings.Repeat("later evidence\n", 100), "", "BUILD_FAILED", "Dockerfile RUN failed with exit code 42", "Dockerfile:2 RUN echo BAD_RUN >&2; exit 42"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var logReads atomic.Int32
			deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == "/observability/logs" {
					logReads.Add(1)
					require.Equal(t, "agents", r.URL.Query().Get("resourceType"))
					require.Equal(t, "test", r.URL.Query().Get("workloadIds"))
					require.Equal(t, "1000", r.URL.Query().Get("limit"))
					_, _ = fmt.Fprintf(w, `{"test":{"logs":[{"timestamp":%q,"message":%q}]},"other-tenant":{"logs":[{"message":"must not appear"}]}}`, time.Now().UTC().Format(time.RFC3339Nano), tc.logs)
					return
				}
				_, _ = fmt.Fprintf(w, `{"status":"FAILED","events":%s}`, deployEventJSON(t, tc.eventType, "r1", tc.message, time.Now()))
			})
			d := Deployment{cwd: t.TempDir(), name: "test", observations: []deployObservation{{kind: "agent", name: "test", started: time.Now().Add(-time.Minute)}}}
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			err := d.waitForRollouts(ctx, time.Millisecond)
			require.Error(t, err)
			require.True(t, core.IsExpectedCLIError(err))
			diagnostic := d.observations[0].diagnostics
			require.Equal(t, tc.code, diagnostic.Code)
			require.Equal(t, tc.cause, diagnostic.Cause)
			require.Equal(t, tc.step, diagnostic.Step)
			require.LessOrEqual(t, len(diagnostic.LogTail), 20)
			if tc.code == "BUILD_FAILED" {
				require.Zero(t, logReads.Load(), "failure event is primary build evidence")
				require.Equal(t, "resource.events", diagnostic.Source)
				require.Equal(t, []string{"bl", "deploy", "--yes", "--wait", "-w", "test-workspace"}, diagnostic.Next.Command)
			} else {
				require.EqualValues(t, 1, logReads.Load())
				require.Equal(t, []string{"bl", "logs", "agent", "test", "-w", "test-workspace", "--period", "30m", "--utc"}, diagnostic.Next.Command)
				if tc.logs != "" {
					require.Contains(t, strings.Join(diagnostic.LogTail, "\n"), "exit code: 42")
				}
			}
			pretty := captureStderr(t, func() { require.True(t, d.printFailureDiagnostics()) })
			require.Contains(t, pretty, "cause: "+tc.cause)
			require.NotContains(t, pretty, "https://", "infrastructure details never reach the terminal")
			require.Equal(t, 1, strings.Count(pretty, "next:"))
			if tc.step != "" {
				require.Contains(t, pretty, "step: "+tc.step)
			}
		})
	}
}

func TestDeployRuntimeDiagnosticReadFailureDoesNotMaskFailure(t *testing.T) {
	var reads atomic.Int32
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		reads.Add(1)
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	o := deployObservation{kind: "agent", name: "test", started: time.Now(), rollout: resourceRollout{Status: "FAILED"}}
	diagnostic := (&Deployment{}).failureDiagnostics(o, "")
	require.Equal(t, "ROLLOUT_FAILED", diagnostic.Code)
	require.Equal(t, noSpecificCause, diagnostic.Cause)
	require.EqualValues(t, 1, reads.Load(), "no retry or pagination")
}

func TestDeployWaitReportsEachResourceAndPreservesTimeout(t *testing.T) {
	core.SetConfigType("agent")
	started := time.Now().Add(-time.Minute)
	var extraReads atomic.Int32
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path != "/agents/extra":
			_, _ = fmt.Fprintf(w, `{"status":"FAILED","events":%s}`, deployEventJSON(t, "buildimage.failed", "r1", badRunDeployMessage, time.Now()))
		case extraReads.Add(1) == 1:
			_, _ = w.Write([]byte(`{"status":"DEPLOYING"}`))
		default:
			<-r.Context().Done() // a hung API read must not extend --timeout
		}
	})
	d := Deployment{name: "test", cwd: t.TempDir(), observations: []deployObservation{{kind: "agent", name: "test", started: started}, {kind: "agent", name: "extra", started: started}}}
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	begin := time.Now()
	err := d.waitForRollouts(ctx, time.Millisecond)
	require.Error(t, err)
	require.Less(t, time.Since(begin), time.Second)
	require.Equal(t, "BUILD_FAILED", d.observations[0].diagnostics.Code)
	require.Equal(t, "DEPLOY_TIMEOUT", d.observations[1].diagnostics.Code)
	require.Equal(t, "monitor", d.observations[1].diagnostics.Phase)
	var result struct {
		Success   bool `json:"success"`
		Resources []struct {
			Status      string             `json:"status"`
			Diagnostics deploy.Diagnostics `json:"diagnostics"`
		} `json:"resources"`
	}
	output := captureDeployStdout(t, func() { d.printStructuredOutput("json", time.Now(), true, err) })
	decoder := json.NewDecoder(strings.NewReader(output))
	require.NoError(t, decoder.Decode(&result))
	require.ErrorIs(t, decoder.Decode(new(any)), io.EOF, "only the JSON document on stdout")
	require.False(t, result.Success)
	require.Len(t, result.Resources, 2)
	require.Equal(t, "FAILED", result.Resources[0].Status)
	require.Equal(t, "DEPLOYING", result.Resources[1].Status, "a timeout keeps the last observed status")
	require.Equal(t, "DEPLOY_TIMEOUT", result.Resources[1].Diagnostics.Code)
	var yamlResult struct {
		Resources []struct{ Diagnostics deploy.Diagnostics } `yaml:"resources"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(captureDeployStdout(t, func() { d.printStructuredOutput("yaml", time.Now(), true, err) })), &yamlResult))
	require.Equal(t, "BUILD_FAILED", yamlResult.Resources[0].Diagnostics.Code)
}

// A failure is this deploy's when it is new since the apply, whatever this
// machine's clock says.
func TestDeployFailureIsJudgedByEventsNotTheClock(t *testing.T) {
	event := deploy.Event{Type: "ai.blaxel.controlplane.buildimage.failed", Status: "FAILED", Time: time.Now().UTC().Format(time.RFC3339Nano), Message: "boom"}
	raw, err := json.Marshal([]deploy.Event{event})
	require.NoError(t, err)
	o := deployObservation{started: time.Now().Add(time.Hour), rollout: resourceRollout{Events: raw}, baselineEvents: map[deploy.Event]struct{}{}}
	require.True(t, o.failureIsCurrent(), "new since the baseline, although this clock is an hour ahead")
	o.baselineEvents[event] = struct{}{}
	require.False(t, o.failureIsCurrent(), "already in the baseline")
}

func TestDeployLatestAttemptSelection(t *testing.T) {
	started := time.Now()
	old := deployEventJSON(t, "buildimage.failed", "r0", "old failure", started.Add(-time.Minute))
	newest := deployEventJSON(t, "buildimage.failed", "r1", badRunDeployMessage, started.Add(time.Second))
	o := deployObservation{started: started, baseline: rolloutBaseline{revision: "r0", deployedRevision: "deployed-old", known: true}, rollout: resourceRollout{Events: old}}
	require.False(t, o.failureIsCurrent())
	o.rollout.Events = deployEventJSON(t, "deployment.failed", "deployed-old", "in-flight old failure", started.Add(time.Second))
	require.False(t, o.failureIsCurrent(), "a failure of a pre-existing revision is not this deploy's")
	o.rollout.Events = newest
	require.True(t, o.failureIsCurrent())
}

func TestDeployNextCommandsDoNotReplaySecrets(t *testing.T) {
	d := Deployment{nextType: "agent", nextName: "explicit-name", folder: "svc"}
	require.Equal(t, []string{"bl", "deploy", "--yes", "--wait", "-w", "ws", "-t", "agent", "-n", "explicit-name", "-d", "svc"}, d.redeployCommand("ws"))
}

func TestDeployWaitFlagIsOptIn(t *testing.T) {
	flags := DeployCmd().Flags()
	require.Equal(t, "false", flags.Lookup("wait").DefValue, "waiting must stay opt-in")
	require.NotNil(t, flags.Lookup("timeout"))

	commands := func() []server.PackageCommand {
		return []server.PackageCommand{{Args: []string{"deploy"}}, {Args: []string{"deploy", "-s", "A=b"}}}
	}
	plain := commands()
	forwardWaitFlags(plain, false, "5m")
	require.Equal(t, commands(), plain, "without --wait, -R packages deploy exactly as before")
	waiting := commands()
	forwardWaitFlags(waiting, true, "5m")
	require.Equal(t, []string{"deploy", "-s", "A=b", "--wait", "--timeout", "5m"}, waiting[1].Args)
}

// deployWithArchive builds a Deployment whose apply returns an upload URL, so
// the whole submit-and-upload path runs against the test server.
func deployWithArchive(t *testing.T) *Deployment {
	t.Helper()
	d := &Deployment{cwd: t.TempDir(), name: "test", timeout: time.Second, blaxelDeployments: []core.Result{{Kind: "Agent", Metadata: map[string]any{"name": "test", "labels": map[string]any{"x-blaxel-auto-generated": "true"}}, Spec: map[string]any{}}}}
	archive, err := os.CreateTemp(t.TempDir(), "archive.zip")
	require.NoError(t, err)
	_, err = archive.WriteString("archive")
	require.NoError(t, err)
	require.NoError(t, archive.Close())
	d.archive = archive
	return d
}

func TestDeployFlagMatrix(t *testing.T) {
	deployedAt := func(revision string) string {
		return fmt.Sprintf(`{"status":"DEPLOYED","events":[{"status":"DEPLOYED","revision":%q,"time":"2026-10-06T00:00:00Z"}]}`, revision)
	}
	hint := "Submitted, not finished. Follow it with `bl get agent test -w test-workspace --watch`, or add --wait next time"
	for _, wait := range []bool{false, true} {
		for _, format := range []string{"pretty", "json", "yaml"} {
			t.Run(fmt.Sprintf("wait=%v/%s", wait, format), func(t *testing.T) {
				core.ResetConfig()
				core.SetConfigType("agent")
				var applies, uploads, reads atomic.Int32
				deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
					w.Header().Set("Content-Type", "application/json")
					switch {
					case r.URL.Path == "/upload":
						uploads.Add(1)
						_, _ = io.Copy(io.Discard, r.Body)
					case r.Method == http.MethodGet:
						reads.Add(1)
						revision := "r0"
						if uploads.Load() > 0 {
							revision = "r1" // this build's rollout
						}
						_, _ = w.Write([]byte(deployedAt(revision)))
					default:
						applies.Add(1)
						w.Header().Set("X-Blaxel-Upload-Url", "http://"+r.Host+"/upload")
						_, _ = w.Write([]byte(`{"metadata":{"name":"test","url":"https://example.test"}}`))
					}
				})
				d := deployWithArchive(t)
				require.NoError(t, d.applyNonInteractive(wait))
				require.EqualValues(t, 1, applies.Load())
				require.EqualValues(t, 1, uploads.Load())
				if wait {
					require.EqualValues(t, 2, reads.Load(), "baseline before apply, then wait for this build")
					require.Len(t, d.observations, 1)
				} else {
					require.Zero(t, reads.Load(), "a plain submission reads nothing before returning")
					require.Empty(t, d.observations)
				}

				if format == "pretty" {
					output := captureDeployStdout(t, d.Ready)
					require.Contains(t, output, "Deployment applied successfully")
					if wait {
						require.NotContains(t, output, "Submitted, not finished")
					} else {
						require.Contains(t, output, hint)
						require.Equal(t, 1, strings.Count(output, "Submitted, not finished"), "exactly one hint line")
					}
					return
				}
				output := captureDeployStdout(t, func() { d.printStructuredOutput(format, time.Now(), false, nil) })
				var result struct {
					Success   bool             `json:"success" yaml:"success"`
					Resources []map[string]any `json:"resources" yaml:"resources"`
				}
				if format == "json" {
					require.NoError(t, json.Unmarshal([]byte(output), &result))
				} else {
					require.NoError(t, yaml.Unmarshal([]byte(output), &result))
				}
				require.True(t, result.Success)
				resource := result.Resources[0]
				require.NotContains(t, resource, "diagnostics")
				if wait {
					require.NotContains(t, resource, "note")
					require.Equal(t, "DEPLOYED", resource["status"])
				} else {
					require.Contains(t, resource["note"], hint)
				}
			})
		}
	}
}

func TestDeployFailedSubmissionIsAPlainErrorWithOrWithoutWait(t *testing.T) {
	core.ResetConfig()
	core.SetConfigType("agent")
	var reads atomic.Int32
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) { // every request is refused
		if r.Method == http.MethodGet {
			reads.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"Forbidden"}`))
	})
	for _, wait := range []bool{false, true} {
		d := Deployment{name: "test", cwd: t.TempDir(), blaxelDeployments: []core.Result{{Kind: "Agent", Metadata: map[string]any{"name": "test"}, Spec: map[string]any{}}}}
		err := d.applyNonInteractive(wait)
		require.ErrorContains(t, err, "failed to apply Agent/test")
		require.Empty(t, d.observations, "nothing to wait for after a failed submission")
		require.False(t, d.printFailureDiagnostics(), "the plain PrintError path is used")
		output := captureDeployStdout(t, func() { d.printStructuredOutput("json", time.Now(), true, err) })
		require.Contains(t, output, `"status": "FAILED"`)
		require.NotContains(t, output, "diagnostics")
		require.NotContains(t, output, `"note"`)
	}
	require.EqualValues(t, 1, reads.Load(), "only the wait run read the baseline")
}

func TestDeployWaitDoesNotInventBuildWhenApplyReturnsNoUpload(t *testing.T) {
	core.ResetConfig()
	core.SetConfigType("agent")
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"status":"DEPLOYED","events":[{"status":"DEPLOYED","revision":"r0","time":"2026-10-01T00:00:00Z"}]}`))
		} else {
			_, _ = w.Write([]byte(`{"metadata":{"name":"test"}}`))
		}
	})
	d := Deployment{name: "test", cwd: t.TempDir(), timeout: time.Second, blaxelDeployments: []core.Result{{Kind: "Agent", Metadata: map[string]any{"name": "test", "labels": map[string]any{"x-blaxel-auto-generated": "true"}}, Spec: map[string]any{}}}}
	require.NoError(t, d.applyNonInteractive(true))
	require.False(t, d.observations[0].autoGenerated, "no archive was submitted; use the existing non-built completion rule")
	require.Equal(t, "DEPLOYED", d.observations[0].rollout.Status)
}

func TestDeployWaitEndsOnStatusAloneWhenThereAreNoEvents(t *testing.T) {
	for _, status := range []string{"FAILED", "DEACTIVATED"} {
		t.Run(status, func(t *testing.T) {
			deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				_, _ = fmt.Fprintf(w, `{"status":%q}`, status)
			})
			d := Deployment{cwd: t.TempDir(), observations: []deployObservation{{kind: "agent", name: "test", started: time.Now(), initialStatus: "DEPLOYED"}}}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			require.Error(t, d.waitForRollouts(ctx, time.Millisecond))
			require.Equal(t, "ROLLOUT_FAILED", d.observations[0].diagnostics.Code)
		})
	}
}

// ev is one platform event, offset from base.
func ev(base time.Time, offset time.Duration, kind, status, revision, message string) deploy.Event {
	return deploy.Event{Type: "ai.blaxel.controlplane." + kind, Status: status, Revision: revision, Message: message, Time: base.Add(offset).UTC().Format(time.RFC3339Nano)}
}

func eventsJSON(t *testing.T, events []deploy.Event) string {
	t.Helper()
	data, err := json.Marshal(events)
	require.NoError(t, err)
	return string(data)
}

// redeploy returns the history of an agent deployed as r0, that history after a
// redeploy was accepted and answered by a reused image build (the platform then
// re-applies the given revision), and once that revision is rolled out and finished.
func redeploy(started time.Time, revision string) (baseline, reused, finished []deploy.Event) {
	baseline = []deploy.Event{
		ev(started, -time.Hour, "buildimage.created", "UPLOADED", "", "Starting build image"),
		ev(started, -time.Hour+time.Second, "buildimage.succeeded", "BUILT", "", "Build image succeeded"),
		ev(started, -time.Hour+2*time.Second, "deployment.ready", "DEPLOYED", "r0", "Deployment ready on a cluster"),
		ev(started, -time.Hour+3*time.Second, "deployment.succeeded", "DEPLOYED", "r0", "Deployments successfully done"),
	}
	reused = append(append([]deploy.Event{}, baseline...),
		ev(started, time.Second, "api.update", "UPDATING", "", "Update agent"),
		ev(started, 4*time.Second, "buildimage.succeeded", "BUILT", "", "Reused an identical image build"),
		ev(started, 4400*time.Millisecond, "api.update", "UPDATED", revision, "Update deployment"),
	)
	finished = append(append([]deploy.Event{}, reused...),
		ev(started, 6*time.Second, "deployment.ready", "DEPLOYED", revision, "Deployment ready on a cluster"),
		ev(started, 12*time.Second, "deployment.succeeded", "DEPLOYED", revision, "Deployments successfully done"))
	return baseline, reused, finished
}

func unchangedObservation(t *testing.T, started time.Time, baseline []deploy.Event) deployObservation {
	t.Helper()
	o := deployObservation{kind: "agent", name: "test", started: started, autoGenerated: true, baseline: rolloutBaseline{revision: "r0", deployedRevision: "r0", known: true}, baselineEvents: map[deploy.Event]struct{}{}}
	for _, e := range baseline {
		o.baselineEvents[e] = struct{}{}
	}
	return o
}

func TestDeployWaitReportsUnchangedRedeployAsSuccess(t *testing.T) {
	core.SetConfigType("agent")
	for _, tc := range []struct {
		name, revision string
		unchanged      bool
	}{
		{"unchanged source re-applies the revision", "r0", true},
		{"rollback to a cached build creates a new revision", "r1", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			started := time.Now()
			baseline, reused, finished := redeploy(started, tc.revision)
			states := [][]deploy.Event{
				reused[:len(baseline)+1], // update accepted, source not yet recognized
				reused,                   // reused, but nothing has deployed since: must not finish on the old DEPLOYED
				finished[:len(reused)+1], // rolling out: DEPLOYED is reported, but the rollout is not finished
				finished,
			}
			var reads atomic.Int32
			deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				n := min(int(reads.Add(1)), len(states))
				status := "DEPLOYED"
				if n == 1 {
					status = "DEPLOYING"
				}
				_, _ = fmt.Fprintf(w, `{"status":%q,"events":%s}`, status, eventsJSON(t, states[n-1]))
			})
			d := Deployment{name: "test", observations: []deployObservation{unchangedObservation(t, started, baseline)}}
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			require.NoError(t, d.waitForRollouts(ctx, time.Millisecond))
			require.EqualValues(t, len(states), reads.Load(), "waits for the platform's final event")
			o := d.observations[0]
			require.False(t, o.reusedAt.IsZero())
			require.Nil(t, o.diagnostics)
			if !tc.unchanged {
				require.Empty(t, o.notice, "a reused build with a new revision is not an unchanged deploy")
				return
			}
			require.Equal(t, "No new revision: the source is unchanged, so the platform reused the identical image build and re-applied revision r0 of agent/test.", o.notice)
			// Pretty output carries the notice and no "submitted" hint; structured output a note.
			d.wait = true
			pretty := captureDeployStdout(t, d.Ready)
			require.Contains(t, pretty, "No new revision")
			require.NotContains(t, pretty, "Submitted, not finished")
			var result struct {
				Resources []struct{ Note string } `json:"resources"`
			}
			require.NoError(t, json.Unmarshal([]byte(captureDeployStdout(t, func() { d.printStructuredOutput("json", time.Now(), false, nil) })), &result))
			require.Equal(t, o.notice, result.Resources[0].Note)
		})
	}
}

func TestDeployWaitStopsHoldingForTheFinalEventAfterTheGrace(t *testing.T) {
	core.SetConfigType("agent")
	original := deployFinalGrace
	deployFinalGrace = 50 * time.Millisecond
	t.Cleanup(func() { deployFinalGrace = original })
	started := time.Now()
	baseline, reused, finished := redeploy(started, "r0")
	rolling := finished[:len(reused)+1] // DEPLOYED, but "deployment.succeeded" never arrives
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"DEPLOYED","events":%s}`, eventsJSON(t, rolling))
	})
	d := Deployment{name: "test", observations: []deployObservation{unchangedObservation(t, started, baseline)}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	begin := time.Now()
	require.NoError(t, d.waitForRollouts(ctx, 10*time.Millisecond))
	require.GreaterOrEqual(t, time.Since(begin), deployFinalGrace, "held for the final event first")
	require.Less(t, time.Since(begin), 2*time.Second)
}

func TestDeployWaitReportsFailureOfReappliedRevision(t *testing.T) {
	core.SetConfigType("agent")
	started := time.Now()
	baseline, reused, _ := redeploy(started, "r0")
	failed := append(append([]deploy.Event{}, reused...), ev(started, 7*time.Second, "deployment.failed", "FAILED", "r0", "Container exited with code 1"))
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"FAILED","events":%s}`, eventsJSON(t, failed))
	})
	d := Deployment{name: "test", cwd: t.TempDir(), observations: []deployObservation{unchangedObservation(t, started, baseline)}}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.Error(t, d.waitForRollouts(ctx, time.Millisecond))
	require.Equal(t, "ROLLOUT_FAILED", d.observations[0].diagnostics.Code)
	require.Empty(t, d.observations[0].notice)
}

func TestDeployWaitStopsWhenNoBuildFollowsTheUpload(t *testing.T) {
	core.SetConfigType("agent")
	original := deployNoBuildGrace
	deployNoBuildGrace = 50 * time.Millisecond
	t.Cleanup(func() { deployNoBuildGrace = original })
	started := time.Now()
	baseline, _, _ := redeploy(started, "r0")
	// The update was accepted but no build ever started; the previous rollout's
	// events keep arriving. They are new since the baseline but not build events.
	dropped := append(append([]deploy.Event{}, baseline...),
		ev(started, time.Second, "api.update", "UPDATING", "", "Update agent"),
		ev(started, 2*time.Second, "deployment.succeeded", "DEPLOYED", "r0", "Deployments successfully done"))
	deployTestClient(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"status":"DEPLOYED","events":%s}`, eventsJSON(t, dropped))
	})
	d := Deployment{name: "test", cwd: t.TempDir(), observations: []deployObservation{unchangedObservation(t, started, baseline)}}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	begin := time.Now()
	err := d.waitForRollouts(ctx, 10*time.Millisecond)
	require.Error(t, err)
	require.Less(t, time.Since(begin), 10*time.Second, "bounded by the grace period, not by --timeout")
	require.True(t, core.IsExpectedCLIError(err))
	o := d.observations[0]
	require.True(t, o.noBuild)
	require.Equal(t, "DEPLOY_TIMEOUT", o.diagnostics.Code)
	require.Contains(t, o.diagnostics.Cause, "No build started within 50ms of the upload")
	require.Contains(t, o.diagnostics.Cause, "not confirmed")
	require.Equal(t, "DEPLOYED", o.rollout.Status, "the last observed status is kept, never relabelled")
	require.Empty(t, o.notice, "an unconfirmed deploy is never reported as success")
	require.Equal(t, []string{"bl", "get", "agent", "test", "-w", "test-workspace", "--watch"}, o.diagnostics.Next.Command)
}

func TestDeployWaitJudgesOnlyWhatItCanTell(t *testing.T) {
	started := time.Now()
	baseline, _, _ := redeploy(started, "r0")
	build := ev(started, time.Second, "buildimage.created", "UPLOADED", "", "Starting build image")
	reuse := ev(started, time.Second, "buildimage.succeeded", "BUILT", "", "Reused an identical image build")
	with := func(extra ...deploy.Event) []deploy.Event {
		return append(append([]deploy.Event{}, baseline...), extra...)
	}
	cases := []struct {
		name            string
		history, events []deploy.Event // before the apply, and now
		mutate          func(*deployObservation)
		notPicked       bool // buildNotPickedUp once the grace period has passed
		reusedSeen      bool // observeReusedBuild
	}{
		{"nothing new in a pipeline that builds", baseline, baseline, nil, true, false},
		{"a build started", baseline, with(build), nil, false, false},
		{"a build was reused", baseline, with(reuse), nil, false, true},
		{"no source build was submitted", baseline, baseline, func(o *deployObservation) { o.autoGenerated = false }, false, false},
		{"no readable baseline", baseline, with(reuse), func(o *deployObservation) { o.baselineEvents = nil }, false, false},
		{"a reuse from an earlier deploy is in the baseline", with(reuse), with(reuse), nil, true, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := unchangedObservation(t, started, tc.history)
			if tc.mutate != nil {
				tc.mutate(&o)
			}
			o.rollout = resourceRollout{Events: json.RawMessage(eventsJSON(t, tc.events))}
			require.Equal(t, tc.notPicked, o.buildNotPickedUp(deployNoBuildGrace))
			require.False(t, o.buildNotPickedUp(deployNoBuildGrace-time.Second), "within the grace period")
			o.observeReusedBuild()
			require.Equal(t, tc.reusedSeen, !o.reusedAt.IsZero())
		})
	}
}

func writeDockerfileFixture(t *testing.T, root, path, content string) {
	t.Helper()
	full := filepath.Join(root, path)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o600))
}

// runDeployProcess runs `bl deploy args...` in root as a subprocess against a stub
// API and returns its output, exit code and the entries left in its private TMPDIR.
func runDeployProcess(t *testing.T, root, apiURL string, args ...string) (stdout, stderr string, code int, tmpEntries []os.DirEntry) {
	t.Helper()
	home, tmp := t.TempDir(), t.TempDir()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], append([]string{"-test.run=^TestDeployProcessHelper$", "--", "deploy", "--yes", "--skip-version-warning", "-w", "test-workspace", "-o", "json"}, args...)...)
	cmd.Dir = root
	cmd.Env = []string{
		"BLAXEL_TEST_DEPLOY_PROCESS=1", "HOME=" + home, "USERPROFILE=" + home,
		"TMPDIR=" + tmp, "TMP=" + tmp, "TEMP=" + tmp,
		"BL_API_KEY=test-api-key", "BL_API_URL=" + apiURL, "BL_RUN_URL=" + apiURL,
	}
	var out, errOut bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &errOut
	var exitErr *exec.ExitError
	require.ErrorAs(t, cmd.Run(), &exitErr, "stdout: %s; stderr: %s", &out, &errOut)
	entries, err := os.ReadDir(tmp)
	require.NoError(t, err)
	return out.String(), errOut.String(), exitErr.ExitCode(), entries
}

func TestDeployProcessHelper(t *testing.T) {
	if os.Getenv("BLAXEL_TEST_DEPLOY_PROCESS") != "1" {
		t.Skip("helper only runs in a subprocess")
	}
	os.Args = append([]string{"bl"}, os.Args[slices.Index(os.Args, "--")+1:]...)
	if err := core.Execute("dev", "", ""); err != nil {
		core.ExitWithError(err)
	}
	os.Exit(0)
}

// An explicitly empty --dockerfile must fail before any API call or archive,
// not silently fall back to the default Dockerfile or the TOML selector.
func TestDeployDockerfileEmptyFlag(t *testing.T) {
	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	root := t.TempDir()
	writeDockerfileFixture(t, root, "blaxel.toml", "name = \"empty-flag-test\"\ntype = \"sandbox\"\n[build]\ndockerfile = \"custom\"\n")
	writeDockerfileFixture(t, root, "Dockerfile", "FROM ghcr.io/blaxel-ai/sandbox:latest\n")
	writeDockerfileFixture(t, root, "custom", "FROM ghcr.io/blaxel-ai/sandbox:latest\n")
	for _, form := range [][]string{{"--dockerfile", ""}, {"--dockerfile="}} {
		stdout, stderr, code, tmp := runDeployProcess(t, root, server.URL, append([]string{"--recursive=false"}, form...)...)
		assert.Equal(t, 1, code, form)
		assert.Empty(t, stdout, "structured stdout must remain empty")
		assert.Contains(t, stderr, "--dockerfile must not be empty")
		assert.Zero(t, requests.Load(), "validation must precede API calls")
		assert.Empty(t, tmp, "validation must precede archive generation")
	}
}

// The flag needs a source build; deploy decides that from the config and flags.
func TestDeployDockerfileNeedsSourceBuild(t *testing.T) {
	root := t.TempDir()
	writeDockerfileFixture(t, root, "custom", "FROM scratch")
	for _, config := range []core.Config{{Image: "registry/image"}, {Type: "volume-template"}, {}} {
		skipBuild := config.Image == "" && config.Type == ""
		_, err := deploy.ResolveDockerfile(root, "", "missing", config, deployBuildsSource(config, skipBuild), false)
		require.ErrorContains(t, err, "requires a source build")
	}
}

func dockerfileZipContents(t *testing.T, d *Deployment) map[string][]string {
	t.Helper()
	require.NoError(t, d.Zip())
	t.Cleanup(func() { _ = os.Remove(d.archive.Name()) })
	reader, err := zip.OpenReader(d.archive.Name())
	require.NoError(t, err)
	defer func() { _ = reader.Close() }()
	files := map[string][]string{}
	for _, entry := range reader.File {
		r, err := entry.Open()
		require.NoError(t, err)
		content, err := io.ReadAll(r)
		require.NoError(t, err)
		require.NoError(t, r.Close())
		files[entry.Name] = append(files[entry.Name], string(content))
	}
	return files
}

func TestDeployDockerfileArchive(t *testing.T) {
	for _, tt := range []struct {
		name, folder, path              string
		defaultFile, companion, ignored bool
	}{
		{"no default", "", "custom", false, false, false},
		{"conflicting default", "", "custom", true, true, false},
		{"no companion fallback", "", "custom", true, false, false},
		{"ignored nested selection", "", "nested/custom", true, true, true},
		{"subproject", "project", "nested/custom", true, true, false},
		{"explicit default", "", "Dockerfile", false, true, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			core.ResetConfig()
			t.Cleanup(core.ResetConfig)
			selectedPath := filepath.Join(tt.folder, tt.path)
			fixture := map[string]string{
				"blaxel.toml":             "name = \"test\"\ntype = \"sandbox\"\n",
				".dockerignore":           "docker-ignored.txt\n",
				"Dockerfile.dockerignore": "wrong-default\n",
				"shared.txt":              "shared COPY source",
				".blaxelignore":           "",
				selectedPath:              "FROM ghcr.io/blaxel-ai/sandbox:latest\nCOPY shared.txt /shared\n",
			}
			if tt.folder != "" {
				fixture[filepath.Join(tt.folder, "blaxel.toml")] = "name = \"subproject\"\ntype = \"sandbox\"\n"
				fixture[filepath.Join(tt.folder, "Dockerfile")] = "wrong-project-default"
			}
			if tt.defaultFile {
				fixture["Dockerfile"] = "wrong-root-default"
			}
			if tt.companion {
				fixture[selectedPath+".dockerignore"] = "selected-ignore\n"
			}
			if tt.ignored {
				fixture[".blaxelignore"] += "nested\n"
			}
			for path, content := range fixture {
				writeDockerfileFixture(t, root, path, content)
			}
			core.ReadConfigToml(tt.folder, false)
			selected, err := deploy.ResolveDockerfile(root, tt.folder, tt.path, core.GetConfig(), true, false)
			require.NoError(t, err)
			files := dockerfileZipContents(t, &Deployment{cwd: root, folder: tt.folder, dockerfile: selected})

			assert.Equal(t, []string{fixture[selectedPath]}, files["Dockerfile"], "exactly one canonical file with selected bytes")
			if tt.companion {
				assert.Equal(t, []string{fixture[selectedPath+".dockerignore"]}, files["Dockerfile.dockerignore"])
			} else {
				assert.NotContains(t, files, "Dockerfile.dockerignore", "an unrelated default companion must not control the build")
			}
			assert.Equal(t, []string{fixture[".dockerignore"]}, files[".dockerignore"])
			assert.Equal(t, []string{fixture["shared.txt"]}, files["shared.txt"])
			if tt.ignored {
				assert.NotContains(t, files, toArchivePath(selectedPath), "the alias survives .blaxelignore")
			} else if selectedPath != "Dockerfile" {
				assert.Equal(t, []string{fixture[selectedPath]}, files[toArchivePath(selectedPath)], "the original stays eligible")
			}
			if tt.folder != "" {
				assert.Equal(t, fixture[filepath.Join(tt.folder, "blaxel.toml")], files["blaxel.toml"][len(files["blaxel.toml"])-1], "retain legacy manifest promotion")
			}
			for path, content := range fixture {
				actual, err := os.ReadFile(filepath.Join(root, path))
				require.NoError(t, err)
				assert.Equal(t, sha256.Sum256([]byte(content)), sha256.Sum256(actual), "local input changed: %s", path)
			}
		})
	}
}

// A selection resolved before the interactive type picker chose a volume template
// must not put a Dockerfile into the volume content.
func TestDeployDockerfileIgnoredForVolumeTemplate(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	core.ResetConfig()
	t.Cleanup(core.ResetConfig)
	writeDockerfileFixture(t, root, "blaxel.toml", "type = \"volume-template\"\n")
	writeDockerfileFixture(t, root, "custom", "custom")
	writeDockerfileFixture(t, root, "data.txt", "data")
	core.ReadConfigToml("", false)
	files := dockerfileZipContents(t, &Deployment{cwd: root, dockerfile: &deploy.Dockerfile{Path: filepath.Join(root, "custom")}})
	assert.NotContains(t, files, "Dockerfile")
	assert.Equal(t, []string{"data"}, files["data.txt"])
}

// A deploy that did not select a file keeps the default Dockerfile and companion
// even when the TOML names a selector, as push does.
func TestDeployDockerfileDefaultArchive(t *testing.T) {
	root := t.TempDir()
	t.Chdir(root)
	core.ResetConfig()
	t.Cleanup(core.ResetConfig)
	writeDockerfileFixture(t, root, "blaxel.toml", "type = \"sandbox\"\n[build]\ndockerfile = \"custom\"\n")
	writeDockerfileFixture(t, root, "Dockerfile", "default")
	writeDockerfileFixture(t, root, "Dockerfile.dockerignore", "default-ignore")
	writeDockerfileFixture(t, root, "custom", "custom")
	core.ReadConfigToml("", false)
	files := dockerfileZipContents(t, &Deployment{cwd: root})
	assert.Equal(t, []string{"default"}, files["Dockerfile"])
	assert.Equal(t, []string{"default-ignore"}, files["Dockerfile.dockerignore"])
}

func TestDeployDockerfileValidation(t *testing.T) {
	root := t.TempDir()
	for _, tt := range []struct {
		defaultContent, customContent string
		warning                       bool
	}{
		{"FROM ghcr.io/blaxel-ai/sandbox:latest", "FROM debian:bookworm-slim", true},
		{"FROM debian:bookworm-slim", "FROM ghcr.io/blaxel-ai/sandbox:latest", false},
	} {
		writeDockerfileFixture(t, root, "Dockerfile", tt.defaultContent)
		writeDockerfileFixture(t, root, "blaxel.Dockerfile", tt.customContent)
		selected, err := deploy.ResolveDockerfile(root, "", "blaxel.Dockerfile", core.Config{}, true, false)
		require.NoError(t, err)
		d := Deployment{cwd: root, dockerfile: selected}
		assert.Equal(t, tt.warning, d.validateDeploymentConfig(core.Config{Type: "sandbox"}) != "")
		assert.Equal(t, !tt.warning, ValidateBuildConfig(root, "", core.Config{Type: "sandbox"}) != "", "push still validates the default")
	}
}

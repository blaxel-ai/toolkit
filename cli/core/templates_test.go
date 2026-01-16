package core

import (
	"os"
	"path/filepath"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateStruct(t *testing.T) {
	t.Run("template struct fields", func(t *testing.T) {
		template := Template{
			Template: blaxel.Template{
				Name:        "agent-python",
				Description: "Python agent template",
				URL:         "https://github.com/blaxel-ai/agent-python",
				Topics:      []string{"agent", "python"},
			},
			Language: "python",
			Type:     "agent",
		}

		assert.Equal(t, "agent-python", template.Name)
		assert.Equal(t, "Python agent template", template.Description)
		assert.Equal(t, "python", template.Language)
		assert.Equal(t, "agent", template.Type)
	})
}

func TestTemplatesGetLanguages(t *testing.T) {
	templates := Templates{
		{
			Template: blaxel.Template{Name: "python-agent-1"},
			Language: "python",
			Type:     "agent",
		},
		{
			Template: blaxel.Template{Name: "ts-agent-1"},
			Language: "typescript",
			Type:     "agent",
		},
		{
			Template: blaxel.Template{Name: "python-agent-2"},
			Language: "python",
			Type:     "agent",
		},
	}

	languages := templates.GetLanguages()

	// Should return unique languages
	assert.Len(t, languages, 2)
	assert.Contains(t, languages, "python")
	assert.Contains(t, languages, "typescript")
}

func TestTemplatesFilterByLanguage(t *testing.T) {
	templates := Templates{
		{
			Template: blaxel.Template{Name: "python-agent-1"},
			Language: "python",
			Type:     "agent",
		},
		{
			Template: blaxel.Template{Name: "ts-agent-1"},
			Language: "typescript",
			Type:     "agent",
		},
		{
			Template: blaxel.Template{Name: "python-agent-2"},
			Language: "python",
			Type:     "agent",
		},
	}

	t.Run("filter by python", func(t *testing.T) {
		filtered := templates.FilterByLanguage("python")
		assert.Len(t, filtered, 2)
		for _, tmpl := range filtered {
			assert.Equal(t, "python", tmpl.Language)
		}
	})

	t.Run("filter by typescript", func(t *testing.T) {
		filtered := templates.FilterByLanguage("typescript")
		assert.Len(t, filtered, 1)
		assert.Equal(t, "ts-agent-1", filtered[0].Name)
	})

	t.Run("filter by non-existent language", func(t *testing.T) {
		filtered := templates.FilterByLanguage("go")
		assert.Len(t, filtered, 0)
	})
}

func TestTemplatesFind(t *testing.T) {
	templates := Templates{
		{
			Template: blaxel.Template{Name: "python-agent"},
			Language: "python",
			Type:     "agent",
		},
		{
			Template: blaxel.Template{Name: "ts-function"},
			Language: "typescript",
			Type:     "function",
		},
	}

	t.Run("find existing template", func(t *testing.T) {
		tmpl, err := templates.Find("python-agent")
		require.NoError(t, err)
		assert.Equal(t, "python-agent", tmpl.Name)
		assert.Equal(t, "python", tmpl.Language)
	})

	t.Run("find non-existent template", func(t *testing.T) {
		_, err := templates.Find("non-existent")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "template not found")
	})
}

func TestFindNextAvailablePort(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected int
	}{
		{
			name:     "empty content",
			content:  "",
			expected: 1339,
		},
		{
			name:     "no ports used",
			content:  "[agent.test]\npath = \"./test\"",
			expected: 1339,
		},
		{
			name:     "port 1339 used",
			content:  "[agent.test]\npath = \"./test\"\nport = 1339",
			expected: 1340,
		},
		{
			name: "multiple ports used",
			content: `[agent.test1]
path = "./test1"
port = 1339

[agent.test2]
path = "./test2"
port = 1340`,
			expected: 1341,
		},
		{
			name: "non-consecutive ports",
			content: `[agent.test1]
port = 1339

[agent.test2]
port = 1342`,
			expected: 1340,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := findNextAvailablePort(tt.content)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCleanTemplate(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "clean_template_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Create the items that should be removed
	itemsToCreate := []string{
		".github",
		".devcontainer",
	}

	filesToCreate := []string{
		"icon.png",
		"icon-dark.png",
		"LICENSE",
	}

	// Create directories
	for _, dir := range itemsToCreate {
		require.NoError(t, os.MkdirAll(filepath.Join(tempDir, dir), 0755))
	}

	// Create files
	for _, file := range filesToCreate {
		require.NoError(t, os.WriteFile(filepath.Join(tempDir, file), []byte("test"), 0644))
	}

	// Create a file that should remain
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "main.py"), []byte("print('hello')"), 0644))

	// Run CleanTemplate
	CleanTemplate(tempDir)

	// Verify items were removed
	for _, dir := range itemsToCreate {
		_, err := os.Stat(filepath.Join(tempDir, dir))
		assert.True(t, os.IsNotExist(err), "%s should have been removed", dir)
	}

	for _, file := range filesToCreate {
		_, err := os.Stat(filepath.Join(tempDir, file))
		assert.True(t, os.IsNotExist(err), "%s should have been removed", file)
	}

	// Verify main.py still exists
	_, err = os.Stat(filepath.Join(tempDir, "main.py"))
	assert.NoError(t, err, "main.py should still exist")
}

func TestTemplateOptions(t *testing.T) {
	opts := TemplateOptions{
		Directory:     "/path/to/project",
		ProjectName:   "my-agent",
		ProjectPrompt: "A helpful agent",
		Language:      "python",
		TemplateName:  "agent-basic",
		Author:        "testuser",
	}

	assert.Equal(t, "/path/to/project", opts.Directory)
	assert.Equal(t, "my-agent", opts.ProjectName)
	assert.Equal(t, "A helpful agent", opts.ProjectPrompt)
	assert.Equal(t, "python", opts.Language)
	assert.Equal(t, "agent-basic", opts.TemplateName)
	assert.Equal(t, "testuser", opts.Author)
}

func TestIgnoreFileAndDir(t *testing.T) {
	ignoreFile := IgnoreFile{
		File: "test.txt",
		Skip: "*.log",
	}

	ignoreDir := IgnoreDir{
		Folder: "node_modules",
		Skip:   "*.tmp",
	}

	assert.Equal(t, "test.txt", ignoreFile.File)
	assert.Equal(t, "*.log", ignoreFile.Skip)
	assert.Equal(t, "node_modules", ignoreDir.Folder)
	assert.Equal(t, "*.tmp", ignoreDir.Skip)
}

func TestCreateDefaultTemplateOptions(t *testing.T) {
	templates := Templates{
		{
			Template: blaxel.Template{Name: "1-agent-basic"},
			Language: "python",
			Type:     "agent",
		},
		{
			Template: blaxel.Template{Name: "2-agent-advanced"},
			Language: "typescript",
			Type:     "agent",
		},
	}

	t.Run("finds template by full name", func(t *testing.T) {
		opts := CreateDefaultTemplateOptions("my-project", "1-agent-basic", templates)
		assert.Equal(t, "my-project", opts.Directory)
		assert.Equal(t, "my-project", opts.ProjectName)
		assert.Equal(t, "1-agent-basic", opts.TemplateName)
		assert.Equal(t, "python", opts.Language)
	})

	t.Run("finds template by stripped name", func(t *testing.T) {
		opts := CreateDefaultTemplateOptions("my-project", "agent-basic", templates)
		assert.Equal(t, "1-agent-basic", opts.TemplateName)
		assert.Equal(t, "python", opts.Language)
	})

	t.Run("returns empty options for non-existent template", func(t *testing.T) {
		opts := CreateDefaultTemplateOptions("my-project", "non-existent", templates)
		assert.Empty(t, opts.TemplateName)
	})
}

func TestEditBlaxelTomlInCurrentDir(t *testing.T) {
	// Create a temporary directory
	tempDir, err := os.MkdirTemp("", "edit_blaxel_test")
	require.NoError(t, err)
	defer os.RemoveAll(tempDir)

	// Change to temp directory
	originalDir, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tempDir))
	defer os.Chdir(originalDir)

	t.Run("no blaxel.toml exists", func(t *testing.T) {
		err := EditBlaxelTomlInCurrentDir("agent", "test-agent", "./agents/test-agent")
		assert.NoError(t, err)

		// File should not be created
		_, err = os.Stat("blaxel.toml")
		assert.True(t, os.IsNotExist(err))
	})

	t.Run("appends to existing blaxel.toml", func(t *testing.T) {
		// Create initial blaxel.toml
		initialContent := `type = "agent"
name = "my-agent"

[agent.existing]
path = "./agents/existing"
port = 1339
`
		require.NoError(t, os.WriteFile("blaxel.toml", []byte(initialContent), 0644))

		err := EditBlaxelTomlInCurrentDir("agent", "new-agent", "./agents/new-agent")
		assert.NoError(t, err)

		// Read and verify content
		content, err := os.ReadFile("blaxel.toml")
		require.NoError(t, err)

		assert.Contains(t, string(content), "[agent.new-agent]")
		assert.Contains(t, string(content), `path = "./agents/new-agent"`)
		assert.Contains(t, string(content), "port = 1340") // Next available port
	})
}

func TestIsCommandAvailable(t *testing.T) {
	// Test with a command that should exist
	t.Run("existing command", func(t *testing.T) {
		// 'ls' or 'echo' should be available on most systems
		result := isCommandAvailable("echo")
		assert.True(t, result)
	})

	t.Run("non-existing command", func(t *testing.T) {
		result := isCommandAvailable("non_existent_command_12345")
		assert.False(t, result)
	})
}

package core

import (
	"errors"
	"path/filepath"
	"regexp"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateRandomDirectoryName(t *testing.T) {
	t.Run("generates correct format", func(t *testing.T) {
		result := generateRandomDirectoryName("agent")

		// Should start with "agent-"
		assert.True(t, len(result) > 6)
		assert.Equal(t, "agent-", result[:6])

		// Should have exactly 5 random characters after the dash
		suffix := result[6:]
		assert.Len(t, suffix, 5)

		// All chars should be lowercase alphanumeric
		for _, c := range suffix {
			assert.True(t, (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9'))
		}
	})

	t.Run("generates unique names", func(t *testing.T) {
		names := make(map[string]bool)
		for i := 0; i < 100; i++ {
			name := generateRandomDirectoryName("test")
			names[name] = true
		}
		// Should generate at least 90 unique names out of 100
		assert.True(t, len(names) >= 90)
	})

	t.Run("works with different resource types", func(t *testing.T) {
		types := []string{"agent", "function", "job", "sandbox", "mcp"}
		for _, rt := range types {
			result := generateRandomDirectoryName(rt)
			assert.True(t, len(result) > len(rt)+1)
			assert.Equal(t, rt+"-", result[:len(rt)+1])
		}
	})
}

func TestTemplateDisplayName(t *testing.T) {
	tests := []struct {
		name     string
		template Template
		expected string
	}{
		{
			name: "removes numeric prefix",
			template: Template{
				Template: blaxel.Template{Name: "1-agent-basic"},
			},
			expected: "agent-basic",
		},
		{
			name: "removes multi-digit prefix",
			template: Template{
				Template: blaxel.Template{Name: "123-function-advanced"},
			},
			expected: "function-advanced",
		},
		{
			name: "keeps name without prefix",
			template: Template{
				Template: blaxel.Template{Name: "agent-basic"},
			},
			expected: "agent-basic",
		},
		{
			name: "keeps numbers not at start",
			template: Template{
				Template: blaxel.Template{Name: "agent-v2-basic"},
			},
			expected: "agent-v2-basic",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := templateDisplayName(tt.template)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestCreateFlowConfig(t *testing.T) {
	t.Run("struct fields", func(t *testing.T) {
		cfg := CreateFlowConfig{
			TemplateType:           "agent",
			NoTTY:                  true,
			ErrorPrefix:            "Agent creation",
			SpinnerTitle:           "Creating your blaxel agent app...",
			BlaxelTomlResourceType: "agent",
		}

		assert.Equal(t, "agent", cfg.TemplateType)
		assert.True(t, cfg.NoTTY)
		assert.Equal(t, "Agent creation", cfg.ErrorPrefix)
		assert.Equal(t, "Creating your blaxel agent app...", cfg.SpinnerTitle)
		assert.Equal(t, "agent", cfg.BlaxelTomlResourceType)
	})

	t.Run("empty BlaxelTomlResourceType", func(t *testing.T) {
		cfg := CreateFlowConfig{}

		assert.Empty(t, cfg.BlaxelTomlResourceType)
	})
}

func TestTemplateDisplayNameRegex(t *testing.T) {
	// Test the regex pattern used in templateDisplayName
	stripRe := regexp.MustCompile(`^\d+-`)

	tests := []struct {
		input    string
		expected string
	}{
		{"1-test", "test"},
		{"12-test", "test"},
		{"123-test-more", "test-more"},
		{"test-no-prefix", "test-no-prefix"},
		{"1test-no-dash", "1test-no-dash"},
		{"-test", "-test"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := stripRe.ReplaceAllString(tt.input, "")
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRunCreateFlowWithDepsReturnsErrorWhenDirectoryExists(t *testing.T) {
	existingDir := t.TempDir()

	err := runCreateFlowWithDeps(
		existingDir,
		"google-adk-py",
		CreateFlowConfig{
			TemplateType: "agent",
			NoTTY:        true,
			ErrorPrefix:  "Agent creation",
			SpinnerTitle: "Creating your blaxel agent app...",
		},
		func(directory string, templates Templates) TemplateOptions {
			t.Fatal("prompt should not run for an existing directory")
			return TemplateOptions{}
		},
		func(opts TemplateOptions) {
			t.Fatal("success should not run after an existing directory error")
		},
		createFlowDeps{},
	)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "already exists")
}

func TestRunCreateFlowWithDepsReturnsErrorWhenTemplateNotFound(t *testing.T) {
	var err error
	stdout, stderr := captureStandardStreams(t, func() {
		err = runCreateFlowWithDeps(
			filepath.Join(t.TempDir(), "new-agent"),
			"missing-template",
			CreateFlowConfig{
				TemplateType: "agent",
				NoTTY:        true,
				ErrorPrefix:  "Agent creation",
				SpinnerTitle: "Creating your blaxel agent app...",
			},
			func(directory string, templates Templates) TemplateOptions {
				t.Fatal("prompt should not run when a template flag is provided")
				return TemplateOptions{}
			},
			func(opts TemplateOptions) {
				t.Fatal("success should not run when template resolution fails")
			},
			createFlowDeps{
				RetrieveTemplates: func(templateType string, noTTY bool, errorPrefix string) (Templates, error) {
					return Templates{
						{
							Template: blaxel.Template{Name: "template-google-adk-py"},
							Language: "python",
							Type:     "agent",
						},
					}, nil
				},
			},
		)
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "template 'template-missing-template' not found")
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "Available templates:")
	assert.Contains(t, stderr, "google-adk-py")
}

func TestRunCreateFlowWithDepsReturnsCloneFailure(t *testing.T) {
	cloneErr := errors.New("dependency install failed")
	cleanCalled := false
	successCalled := false

	err := runCreateFlowWithDeps(
		filepath.Join(t.TempDir(), "new-agent"),
		"google-adk-py",
		CreateFlowConfig{
			TemplateType: "agent",
			NoTTY:        true,
			ErrorPrefix:  "Agent creation",
			SpinnerTitle: "Creating your blaxel agent app...",
		},
		func(directory string, templates Templates) TemplateOptions {
			t.Fatal("prompt should not run when a template flag is provided")
			return TemplateOptions{}
		},
		func(opts TemplateOptions) {
			successCalled = true
		},
		createFlowDeps{
			RetrieveTemplates: func(templateType string, noTTY bool, errorPrefix string) (Templates, error) {
				return Templates{
					{
						Template: blaxel.Template{Name: "template-google-adk-py"},
						Language: "python",
						Type:     "agent",
					},
				}, nil
			},
			CloneTemplate: func(opts TemplateOptions, templates Templates, noTTY bool, errorPrefix string, spinnerTitle string) error {
				return cloneErr
			},
			CleanTemplate: func(directory string) {
				cleanCalled = true
			},
		},
	)

	require.ErrorIs(t, err, cloneErr)
	assert.False(t, cleanCalled)
	assert.False(t, successCalled)
}

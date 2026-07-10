package core

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
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

func TestRunCreateFlowWithDepsRequiresJobTemplateWithYes(t *testing.T) {
	var runErr error
	stdout, stderr := captureStandardStreams(t, func() {
		runErr = runCreateFlowWithDeps(
			filepath.Join(t.TempDir(), "new-job"),
			"",
			CreateFlowConfig{
				TemplateType: "job",
				NoTTY:        true,
				ErrorPrefix:  "Job creation",
				SpinnerTitle: "Creating your blaxel job...",
			},
			func(directory string, templates Templates) TemplateOptions {
				t.Fatal("prompt should not run in non-interactive job creation")
				return TemplateOptions{}
			},
			func(opts TemplateOptions) {
				t.Fatal("success should not run without an explicit template")
			},
			createFlowDeps{
				RetrieveTemplates: func(templateType string, noTTY bool, errorPrefix string) (Templates, error) {
					t.Fatal("catalog retrieval should not run without an explicit template")
					return nil, nil
				},
				CloneTemplate: func(opts TemplateOptions, templates Templates, noTTY bool, errorPrefix string, spinnerTitle string) error {
					t.Fatal("clone should not run without an explicit template")
					return nil
				},
			},
		)
	})

	require.Error(t, runErr)
	assert.Contains(t, runErr.Error(), "--template is required")
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "bl new job --list")
}

func TestRunCreateFlowWithDepsValidatesJobTemplateBeforeDirectory(t *testing.T) {
	existingDirectory := t.TempDir()
	err := runCreateFlowWithDeps(
		existingDirectory,
		"",
		CreateFlowConfig{TemplateType: "job", NoTTY: true, ErrorPrefix: "Job creation"},
		func(directory string, templates Templates) TemplateOptions {
			t.Fatal("prompt should not run in non-interactive job creation")
			return TemplateOptions{}
		},
		func(opts TemplateOptions) {
			t.Fatal("success should not run without an explicit template")
		},
		createFlowDeps{
			RetrieveTemplates: func(templateType string, noTTY bool, errorPrefix string) (Templates, error) {
				t.Fatal("catalog retrieval should not run without an explicit template")
				return nil, nil
			},
		},
	)

	require.Error(t, err)
	assert.ErrorContains(t, err, "--template is required")
	assert.NotContains(t, err.Error(), "already exists")
}

func TestRunCreateFlowWithDepsReturnsErrorWhenGithubRunnerIsUnavailable(t *testing.T) {
	cloneCalled := false
	cleanCalled := false
	successCalled := false
	var runErr error
	stdout, stderr := captureStandardStreams(t, func() {
		runErr = runCreateFlowWithDeps(
			filepath.Join(t.TempDir(), "new-runner"),
			"github-runner",
			CreateFlowConfig{
				TemplateType: "job",
				NoTTY:        true,
				ErrorPrefix:  "Job creation",
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
					return testJobTemplates(false, ""), nil
				},
				CloneTemplate: func(opts TemplateOptions, templates Templates, noTTY bool, errorPrefix string, spinnerTitle string) error {
					cloneCalled = true
					return nil
				},
				CleanTemplate: func(directory string) {
					cleanCalled = true
				},
			},
		)
	})

	require.Error(t, runErr)
	assert.ErrorContains(t, runErr, "template 'template-github-runner' not found")
	assert.Empty(t, stdout)
	assert.Contains(t, stderr, "jobs-ts")
	assert.Contains(t, stderr, "jobs-py")
	assert.NotContains(t, stderr, "  - github-runner")
	assert.False(t, cloneCalled)
	assert.False(t, cleanCalled)
	assert.False(t, successCalled)
}

func TestRunCreateFlowWithDepsResolvesJobTemplateAliases(t *testing.T) {
	tests := []struct {
		name             string
		templateFlag     string
		expectedTemplate string
		expectedLanguage string
	}{
		{name: "python blank", templateFlag: "jobs-py", expectedTemplate: jobPythonTemplate, expectedLanguage: "python"},
		{name: "typescript blank", templateFlag: "jobs-ts", expectedTemplate: jobTypescriptTemplate, expectedLanguage: "typescript"},
		{name: "github runner", templateFlag: "github-runner", expectedTemplate: jobGithubRunnerTemplate},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "new-job")
			var clonedOpts TemplateOptions
			var successOpts TemplateOptions
			err := runCreateFlowWithDeps(
				directory,
				tt.templateFlag,
				CreateFlowConfig{TemplateType: "job", NoTTY: true, ErrorPrefix: "Job creation"},
				func(directory string, templates Templates) TemplateOptions {
					t.Fatal("prompt should not run when a template flag is provided")
					return TemplateOptions{}
				},
				func(opts TemplateOptions) {
					successOpts = opts
				},
				createFlowDeps{
					RetrieveTemplates: func(templateType string, noTTY bool, errorPrefix string) (Templates, error) {
						return testJobTemplates(true, ""), nil
					},
					CloneTemplate: func(opts TemplateOptions, templates Templates, noTTY bool, errorPrefix string, spinnerTitle string) error {
						clonedOpts = opts
						return nil
					},
					CleanTemplate: func(directory string) {},
					OutputFormat:  func() string { return "pretty" },
				},
			)

			require.NoError(t, err)
			assert.Equal(t, tt.expectedTemplate, clonedOpts.TemplateName)
			assert.Equal(t, tt.expectedLanguage, clonedOpts.Language)
			assert.Equal(t, directory, clonedOpts.Directory)
			assert.Equal(t, clonedOpts, successOpts)
		})
	}
}

func TestRunCreateFlowWithDepsRejectsInvalidJobLanguageMetadata(t *testing.T) {
	tests := []struct {
		name         string
		templateFlag string
		templates    Templates
		wantError    string
	}{
		{
			name:         "runner language",
			templateFlag: "github-runner",
			templates:    testJobTemplates(true, "python"),
			wantError:    "must not declare a language",
		},
		{
			name:         "numeric runner language",
			templateFlag: "github-runner",
			templates: Templates{
				{Template: blaxel.Template{Name: "1-" + jobGithubRunnerTemplate}, Language: "python", Type: "job"},
			},
			wantError: "must not declare a language",
		},
		{
			name:         "python blank mislabeled",
			templateFlag: "jobs-py",
			templates: Templates{
				{Template: blaxel.Template{Name: jobPythonTemplate}, Language: "typescript", Type: "job"},
			},
			wantError: "must declare language",
		},
		{
			name:         "typescript blank missing language",
			templateFlag: "jobs-ts",
			templates: Templates{
				{Template: blaxel.Template{Name: jobTypescriptTemplate}, Type: "job"},
			},
			wantError: "must declare language",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cloneCalled := false
			err := runCreateFlowWithDeps(
				filepath.Join(t.TempDir(), "new-job"),
				tt.templateFlag,
				CreateFlowConfig{TemplateType: "job", NoTTY: true, ErrorPrefix: "Job creation"},
				func(directory string, templates Templates) TemplateOptions {
					t.Fatal("prompt should not run when a template flag is provided")
					return TemplateOptions{}
				},
				func(opts TemplateOptions) {
					t.Fatal("success should not run for invalid job metadata")
				},
				createFlowDeps{
					RetrieveTemplates: func(templateType string, noTTY bool, errorPrefix string) (Templates, error) {
						return tt.templates, nil
					},
					CloneTemplate: func(opts TemplateOptions, templates Templates, noTTY bool, errorPrefix string, spinnerTitle string) error {
						cloneCalled = true
						return nil
					},
				},
			)

			require.Error(t, err)
			assert.ErrorContains(t, err, tt.wantError)
			assert.False(t, cloneCalled)
		})
	}
}

func TestRunCreateFlowWithDepsPrintsStructuredJobOutput(t *testing.T) {
	tests := []struct {
		name         string
		format       string
		templateFlag string
		templateName string
		language     string
	}{
		{name: "runner json", format: "json", templateFlag: "github-runner", templateName: jobGithubRunnerTemplate},
		{name: "blank json", format: "json", templateFlag: "jobs-py", templateName: jobPythonTemplate, language: "python"},
		{name: "runner yaml", format: "yaml", templateFlag: "github-runner", templateName: jobGithubRunnerTemplate},
		{name: "blank yaml", format: "yaml", templateFlag: "jobs-ts", templateName: jobTypescriptTemplate, language: "typescript"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "new-job")
			successCalled := false
			var runErr error
			stdout, stderr := captureStandardStreams(t, func() {
				runErr = runCreateFlowWithDeps(
					directory,
					tt.templateFlag,
					CreateFlowConfig{TemplateType: "job", NoTTY: true, ErrorPrefix: "Job creation"},
					func(directory string, templates Templates) TemplateOptions {
						t.Fatal("prompt should not run when a template flag is provided")
						return TemplateOptions{}
					},
					func(opts TemplateOptions) {
						successCalled = true
					},
					createFlowDeps{
						RetrieveTemplates: func(templateType string, noTTY bool, errorPrefix string) (Templates, error) {
							return testJobTemplates(true, ""), nil
						},
						CloneTemplate: func(opts TemplateOptions, templates Templates, noTTY bool, errorPrefix string, spinnerTitle string) error {
							return nil
						},
						CleanTemplate: func(directory string) {},
						OutputFormat:  func() string { return tt.format },
					},
				)
			})

			require.NoError(t, runErr)
			assert.Empty(t, stderr)
			assert.False(t, successCalled)

			result := map[string]string{}
			if tt.format == "json" {
				require.NoError(t, json.Unmarshal([]byte(stdout), &result))
			} else {
				require.NoError(t, yaml.Unmarshal([]byte(stdout), &result))
			}
			assert.Equal(t, map[string]string{
				"directory": directory,
				"template":  tt.templateName,
				"language":  tt.language,
				"type":      "job",
			}, result)
		})
	}
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

func TestNormalizeTemplateNameFlag(t *testing.T) {
	tests := []struct {
		name         string
		templateType string
		input        string
		expected     string
	}{
		{
			name:         "keeps non-sandbox full template name",
			templateType: "agent",
			input:        "template-google-adk-py",
			expected:     "template-google-adk-py",
		},
		{
			name:         "maps github runner job shorthand",
			templateType: "job",
			input:        "github-runner",
			expected:     jobGithubRunnerTemplate,
		},
		{
			name:         "prefixes non-sandbox shorthand",
			templateType: "agent",
			input:        "google-adk-py",
			expected:     "template-google-adk-py",
		},
		{
			name:         "maps scratch sandbox alias",
			templateType: "sandbox",
			input:        "scratch",
			expected:     sandboxScratchTemplate,
		},
		{
			name:         "maps claude code sandbox alias",
			templateType: "sandbox",
			input:        "claude-code",
			expected:     sandboxClaudeCodeTemplate,
		},
		{
			name:         "maps codex sandbox alias",
			templateType: "sandbox",
			input:        "codex",
			expected:     sandboxCodexTemplate,
		},
		{
			name:         "keeps legacy sandbox template reachable",
			templateType: "sandbox",
			input:        "sandbox-codegen",
			expected:     "template-sandbox-codegen",
		},
		{
			name:         "keeps full sandbox template name reachable",
			templateType: "sandbox",
			input:        "template-sandbox-codegen",
			expected:     "template-sandbox-codegen",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, normalizeTemplateNameFlag(tt.input, tt.templateType))
		})
	}
}

func TestSandboxTemplatesForDisplay(t *testing.T) {
	templates := Templates{
		{Template: blaxel.Template{Name: "template-sandbox-codegen"}},
		{Template: blaxel.Template{Name: sandboxCodexTemplate}},
		{Template: blaxel.Template{Name: sandboxScratchTemplate}},
		{Template: blaxel.Template{Name: sandboxClaudeCodeTemplate}},
	}

	displayTemplates := sandboxTemplatesForDisplay(templates)

	assert.Equal(t, []string{
		sandboxScratchTemplate,
		sandboxClaudeCodeTemplate,
		sandboxCodexTemplate,
	}, []string{
		displayTemplates[0].Name,
		displayTemplates[1].Name,
		displayTemplates[2].Name,
	})
}

func TestSandboxTemplatesForDisplayFallsBackToAvailableTemplates(t *testing.T) {
	templates := Templates{
		{Template: blaxel.Template{Name: "template-sandbox-codegen"}},
	}

	displayTemplates := sandboxTemplatesForDisplay(templates)

	assert.Len(t, displayTemplates, 1)
	assert.Equal(t, "template-sandbox-codegen", displayTemplates[0].Name)
}

func TestSandboxTemplateLabelsAndFlagNames(t *testing.T) {
	tests := []struct {
		name     string
		template Template
		label    string
		flagName string
	}{
		{
			name:     "scratch",
			template: Template{Template: blaxel.Template{Name: sandboxScratchTemplate}},
			label:    "Scratch",
			flagName: "scratch",
		},
		{
			name:     "claude code",
			template: Template{Template: blaxel.Template{Name: sandboxClaudeCodeTemplate}},
			label:    "Claude Code",
			flagName: "claude-code",
		},
		{
			name:     "codex",
			template: Template{Template: blaxel.Template{Name: sandboxCodexTemplate}},
			label:    "Codex",
			flagName: "codex",
		},
		{
			name:     "legacy fallback",
			template: Template{Template: blaxel.Template{Name: "template-sandbox-codegen"}},
			label:    "sandbox-codegen",
			flagName: "sandbox-codegen",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.label, sandboxTemplateLabel(tt.template))
			assert.Equal(t, tt.flagName, sandboxTemplateFlagName(tt.template))
		})
	}
}

func TestFinalizeSandboxTemplateWritesClaudeCodeRuntimeFiles(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, FinalizeSandboxTemplate(TemplateOptions{
		Directory:    tempDir,
		TemplateName: sandboxClaudeCodeTemplate,
	}))

	dockerfile, err := os.ReadFile(filepath.Join(tempDir, "Dockerfile"))
	require.NoError(t, err)
	assert.Contains(t, string(dockerfile), "npm install -g @anthropic-ai/claude-code@latest")
	assert.Contains(t, string(dockerfile), `ENV PATH="/usr/local/bin:/app/node_modules/.bin:$PATH"`)
	assert.Contains(t, string(dockerfile), "ripgrep")

	readme, err := os.ReadFile(filepath.Join(tempDir, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "bl new sandbox my-sandbox -t claude-code -y")
	assert.Contains(t, string(readme), "claude --version")

	makefile, err := os.ReadFile(filepath.Join(tempDir, "Makefile"))
	require.NoError(t, err)
	assert.Contains(t, string(makefile), "blaxel-sandbox-claude-code")
}

func TestFinalizeSandboxTemplateWritesCodexRuntimeFiles(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, FinalizeSandboxTemplate(TemplateOptions{
		Directory:    tempDir,
		TemplateName: sandboxCodexTemplate,
	}))

	dockerfile, err := os.ReadFile(filepath.Join(tempDir, "Dockerfile"))
	require.NoError(t, err)
	assert.Contains(t, string(dockerfile), "npm install -g @openai/codex@latest")
	assert.Contains(t, string(dockerfile), `ENV PATH="/usr/local/bin:/app/node_modules/.bin:$PATH"`)

	readme, err := os.ReadFile(filepath.Join(tempDir, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "bl new sandbox my-sandbox -t codex -y")
	assert.Contains(t, string(readme), "codex --version")
}

func TestFinalizeSandboxTemplateWritesScratchRuntimeFiles(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, FinalizeSandboxTemplate(TemplateOptions{
		Directory:    tempDir,
		TemplateName: sandboxScratchTemplate,
	}))

	dockerfile, err := os.ReadFile(filepath.Join(tempDir, "Dockerfile"))
	require.NoError(t, err)
	assert.NotContains(t, string(dockerfile), "@anthropic-ai/claude-code")
	assert.NotContains(t, string(dockerfile), "@openai/codex")

	readme, err := os.ReadFile(filepath.Join(tempDir, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readme), "bl new sandbox my-sandbox -t scratch -y")
	assert.NotContains(t, string(readme), "--version")
}

func TestFinalizeSandboxTemplateRejectsSymlinkRuntimeFiles(t *testing.T) {
	for _, name := range []string{"Dockerfile", "Makefile", "entrypoint.sh", "README.md"} {
		t.Run(name, func(t *testing.T) {
			tempDir := t.TempDir()
			outsideDir := t.TempDir()
			outsidePath := filepath.Join(outsideDir, "outside.txt")
			require.NoError(t, os.WriteFile(outsidePath, []byte("keep me"), 0644))

			err := os.Symlink(outsidePath, filepath.Join(tempDir, name))
			if err != nil {
				t.Skipf("symlinks are not available: %v", err)
			}

			err = FinalizeSandboxTemplate(TemplateOptions{
				Directory:    tempDir,
				TemplateName: sandboxClaudeCodeTemplate,
			})
			require.Error(t, err)
			assert.Contains(t, err.Error(), "refusing to write "+name)

			outsideContent, readErr := os.ReadFile(outsidePath)
			require.NoError(t, readErr)
			assert.Equal(t, "keep me", string(outsideContent))
		})
	}
}

func TestFinalizeSandboxTemplatePreservesLowercaseDockerfileWhenOnlyLowercaseExists(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "dockerfile"), []byte("old lower"), 0644))
	if _, err := os.Stat(filepath.Join(tempDir, "Dockerfile")); err == nil {
		t.Skip("case-insensitive filesystem treats Dockerfile and dockerfile as the same path")
	}
	require.NoError(t, FinalizeSandboxTemplate(TemplateOptions{
		Directory:    tempDir,
		TemplateName: sandboxCodexTemplate,
	}))

	lowercaseDockerfile, err := os.ReadFile(filepath.Join(tempDir, "dockerfile"))
	require.NoError(t, err)
	assert.Contains(t, string(lowercaseDockerfile), "npm install -g @openai/codex@latest")

	_, err = os.Stat(filepath.Join(tempDir, "Dockerfile"))
	assert.True(t, os.IsNotExist(err))
}

func TestFinalizeSandboxTemplateRemovesDuplicateLowercaseDockerfile(t *testing.T) {
	tempDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "Dockerfile"), []byte("old upper"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(tempDir, "dockerfile"), []byte("old lower"), 0644))
	upperInfo, err := os.Stat(filepath.Join(tempDir, "Dockerfile"))
	require.NoError(t, err)
	lowerInfo, err := os.Stat(filepath.Join(tempDir, "dockerfile"))
	require.NoError(t, err)
	if os.SameFile(upperInfo, lowerInfo) {
		t.Skip("case-insensitive filesystem treats Dockerfile and dockerfile as the same path")
	}

	require.NoError(t, FinalizeSandboxTemplate(TemplateOptions{
		Directory:    tempDir,
		TemplateName: sandboxClaudeCodeTemplate,
	}))

	_, err = os.Stat(filepath.Join(tempDir, "dockerfile"))
	assert.True(t, os.IsNotExist(err))
	dockerfile, err := os.ReadFile(filepath.Join(tempDir, "Dockerfile"))
	require.NoError(t, err)
	assert.Contains(t, string(dockerfile), "npm install -g @anthropic-ai/claude-code@latest")
}

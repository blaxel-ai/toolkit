package core

import (
	"strings"
	"testing"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/stretchr/testify/assert"
)

func testJobTemplates(includeRunner bool, runnerLanguage string) Templates {
	templates := Templates{
		{Template: blaxel.Template{Name: jobTypescriptTemplate}, Language: "typescript", Type: "job"},
		{Template: blaxel.Template{Name: jobPythonTemplate}, Language: "python", Type: "job"},
	}
	if includeRunner {
		templates = append(templates, Template{
			Template: blaxel.Template{Name: jobGithubRunnerTemplate},
			Language: runnerLanguage,
			Type:     "job",
		})
	}
	return templates
}

func templateNames(templates Templates) []string {
	names := make([]string, 0, len(templates))
	for _, template := range templates {
		names = append(names, template.Name)
	}
	return names
}

func TestJobTemplateChoices(t *testing.T) {
	templates := testJobTemplates(true, "")

	assert.Equal(t, []jobTemplateChoice{
		{label: "Blank", value: jobBlankChoice},
		{label: "GitHub Runner", value: jobGithubRunnerChoice},
	}, jobTemplateChoices(templates))
}

func TestJobTemplateChoicesRequireCatalogEntries(t *testing.T) {
	t.Run("runner is hidden until it is in the catalog", func(t *testing.T) {
		templates := Templates{
			{Template: blaxel.Template{Name: jobPythonTemplate}, Language: "python", Type: "job"},
		}

		assert.Equal(t, []jobTemplateChoice{
			{label: "Blank", value: jobBlankChoice},
		}, jobTemplateChoices(templates))
	})

	t.Run("blank is hidden without a known blank template", func(t *testing.T) {
		templates := Templates{
			{Template: blaxel.Template{Name: jobGithubRunnerTemplate}, Type: "job"},
		}

		assert.Equal(t, []jobTemplateChoice{
			{label: "GitHub Runner", value: jobGithubRunnerChoice},
		}, jobTemplateChoices(templates))
	})
}

func TestPromptJobTemplateOptionsWithDeps(t *testing.T) {
	templates := testJobTemplates(true, "")

	t.Run("blank delegates to the existing language picker", func(t *testing.T) {
		expected := TemplateOptions{Directory: "selected-dir", TemplateName: jobPythonTemplate, Language: "python"}
		opts, err := promptJobTemplateOptionsWithDeps("initial-dir", templates, jobTemplatePromptDeps{
			selectJobType: func(directory string, choices []jobTemplateChoice) (string, string, error) {
				assert.Equal(t, "initial-dir", directory)
				assert.Len(t, choices, 2)
				return jobBlankChoice, "selected-dir", nil
			},
			promptTemplates: func(directory string, templates Templates) TemplateOptions {
				assert.Equal(t, "selected-dir", directory)
				assert.Equal(t, []string{jobTypescriptTemplate, jobPythonTemplate}, templateNames(templates))
				return expected
			},
		})

		assert.NoError(t, err)
		assert.Equal(t, expected, opts)
	})

	t.Run("runner skips the language picker", func(t *testing.T) {
		opts, err := promptJobTemplateOptionsWithDeps("runner-dir", templates, jobTemplatePromptDeps{
			selectJobType: func(directory string, choices []jobTemplateChoice) (string, string, error) {
				return jobGithubRunnerChoice, directory, nil
			},
			promptTemplates: func(directory string, templates Templates) TemplateOptions {
				t.Fatal("blank picker should not run for GitHub Runner")
				return TemplateOptions{}
			},
		})

		assert.NoError(t, err)
		assert.Equal(t, "runner-dir", opts.Directory)
		assert.Equal(t, jobGithubRunnerTemplate, opts.TemplateName)
		assert.Empty(t, opts.Language)
	})

	t.Run("python-only catalog keeps the existing picker", func(t *testing.T) {
		pythonOnly := Templates{{Template: blaxel.Template{Name: jobPythonTemplate}, Language: "python", Type: "job"}}
		expected := TemplateOptions{Directory: "python-dir", TemplateName: jobPythonTemplate, Language: "python"}
		opts, err := promptJobTemplateOptionsWithDeps("python-dir", pythonOnly, jobTemplatePromptDeps{
			selectJobType: func(directory string, choices []jobTemplateChoice) (string, string, error) {
				t.Fatal("job type picker should not run with only Blank available")
				return "", "", nil
			},
			promptTemplates: func(directory string, templates Templates) TemplateOptions {
				assert.Equal(t, []string{jobPythonTemplate}, templateNames(templates))
				return expected
			},
		})

		assert.NoError(t, err)
		assert.Equal(t, expected, opts)
	})

	t.Run("runner-only catalog auto-selects runner", func(t *testing.T) {
		runnerOnly := Templates{{Template: blaxel.Template{Name: jobGithubRunnerTemplate}, Type: "job"}}
		opts, err := promptJobTemplateOptionsWithDeps("runner-dir", runnerOnly, jobTemplatePromptDeps{
			selectJobType: func(directory string, choices []jobTemplateChoice) (string, string, error) {
				t.Fatal("job type picker should not run with only GitHub Runner available")
				return "", "", nil
			},
			promptTemplates: func(directory string, templates Templates) TemplateOptions {
				t.Fatal("blank picker should not run with only GitHub Runner available")
				return TemplateOptions{}
			},
		})

		assert.NoError(t, err)
		assert.Equal(t, jobGithubRunnerTemplate, opts.TemplateName)
	})

	t.Run("custom-only catalog delegates to the generic picker", func(t *testing.T) {
		customTemplates := Templates{
			{Template: blaxel.Template{Name: "template-custom-one"}, Type: "job"},
			{Template: blaxel.Template{Name: "template-custom-two"}, Type: "job"},
		}
		expected := TemplateOptions{Directory: "custom-dir", TemplateName: "template-custom-two"}
		opts, err := promptJobTemplateOptionsWithDeps("custom-dir", customTemplates, jobTemplatePromptDeps{
			selectJobType: func(directory string, choices []jobTemplateChoice) (string, string, error) {
				t.Fatal("job type picker should not run without known job types")
				return "", "", nil
			},
			promptTemplates: func(directory string, templates Templates) TemplateOptions {
				assert.Equal(t, "custom-dir", directory)
				assert.Equal(t, templateNames(customTemplates), templateNames(templates))
				return expected
			},
		})

		assert.NoError(t, err)
		assert.Equal(t, expected, opts)
	})
}

func TestValidateJobTemplateOptions(t *testing.T) {
	tests := []struct {
		name      string
		opts      TemplateOptions
		wantError string
	}{
		{name: "runner", opts: TemplateOptions{TemplateName: jobGithubRunnerTemplate}},
		{name: "numeric runner", opts: TemplateOptions{TemplateName: "1-" + jobGithubRunnerTemplate}},
		{name: "runner with language", opts: TemplateOptions{TemplateName: jobGithubRunnerTemplate, Language: "python"}, wantError: "must not declare a language"},
		{name: "numeric runner with language", opts: TemplateOptions{TemplateName: "1-" + jobGithubRunnerTemplate, Language: "python"}, wantError: "must not declare a language"},
		{name: "python", opts: TemplateOptions{TemplateName: jobPythonTemplate, Language: "python"}},
		{name: "python mislabeled", opts: TemplateOptions{TemplateName: jobPythonTemplate, Language: "typescript"}, wantError: "must declare language"},
		{name: "typescript", opts: TemplateOptions{TemplateName: jobTypescriptTemplate, Language: "typescript"}},
		{name: "typescript missing language", opts: TemplateOptions{TemplateName: jobTypescriptTemplate}, wantError: "must declare language"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateJobTemplateOptions(tt.opts)
			if tt.wantError != "" {
				assert.ErrorContains(t, err, tt.wantError)
				return
			}
			assert.NoError(t, err)
		})
	}
}

func TestJobTemplatesForDisplay(t *testing.T) {
	t.Run("orders known templates before custom templates", func(t *testing.T) {
		templates := Templates{
			{Template: blaxel.Template{Name: jobGithubRunnerTemplate}, Type: "job"},
			{Template: blaxel.Template{Name: "template-custom-job"}, Type: "job"},
			{Template: blaxel.Template{Name: jobTypescriptTemplate}, Language: "typescript", Type: "job"},
			{Template: blaxel.Template{Name: jobPythonTemplate}, Language: "python", Type: "job"},
		}

		assert.Equal(t, []string{
			jobTypescriptTemplate,
			jobPythonTemplate,
			jobGithubRunnerTemplate,
			"template-custom-job",
		}, templateNames(jobTemplatesForDisplay(templates)))
	})

	t.Run("preserves available templates when runner is absent", func(t *testing.T) {
		assert.Equal(t, []string{
			jobTypescriptTemplate,
			jobPythonTemplate,
		}, templateNames(jobTemplatesForDisplay(testJobTemplates(false, ""))))
	})

	t.Run("preserves runner when blank is absent", func(t *testing.T) {
		templates := Templates{{Template: blaxel.Template{Name: jobGithubRunnerTemplate}, Type: "job"}}
		assert.Equal(t, []string{jobGithubRunnerTemplate}, templateNames(jobTemplatesForDisplay(templates)))
	})
}

func TestIsPortableJobName(t *testing.T) {
	assert.True(t, isPortableJobName("my-job_2"))
	assert.False(t, isPortableJobName("projects/my-job"))
	assert.False(t, isPortableJobName("-leading-dash"))
	assert.False(t, isPortableJobName("path with spaces"))
	assert.False(t, isPortableJobName("$(unsafe)"))
	assert.False(t, isPortableJobName("line\nbreak"))
}

func TestJobTemplateOptionLabelSanitizesFallbackCatalogNames(t *testing.T) {
	template := Template{Template: blaxel.Template{Name: "template-bad\nname\u202e"}, Type: "job"}
	label := jobTemplateOptionLabel(template)

	assert.Equal(t, "bad name ", label)
	assert.NotContains(t, label, "\n")
	assert.NotContains(t, label, "\u202e")
}

func TestPrintAvailableJobTemplatesSanitizesCatalogText(t *testing.T) {
	templates := Templates{{
		Template: blaxel.Template{
			Name:        "template-bad\nname\u202e",
			Description: "unsafe\x1b]0;title\a description\u2066",
		},
		Type: "job",
	}}

	stdout, stderr := captureStandardStreams(t, func() {
		printAvailableTemplates(templates, "job")
	})

	assert.Empty(t, stdout)
	assert.NotContains(t, stderr, "\nname")
	assert.NotContains(t, stderr, "\x1b")
	assert.NotContains(t, stderr, "\a")
	assert.NotContains(t, stderr, "\u202e")
	assert.NotContains(t, stderr, "\u2066")
	assert.Contains(t, stderr, "bad name")
	assert.Contains(t, stderr, "unsafe ]0;title  description")
}

func TestPrintJobCreationSuccess(t *testing.T) {
	t.Run("blank job keeps batch instructions", func(t *testing.T) {
		stdout, stderr := captureStandardStreams(t, func() {
			printJobCreationSuccess(TemplateOptions{
				Directory:    "my-job",
				TemplateName: jobPythonTemplate,
			})
		})

		assert.Empty(t, stderr)
		assert.Contains(t, stdout, "Your blaxel job has been created successfully")
		assert.Contains(t, stdout, "cd my-job")
		assert.Contains(t, stdout, "bl run job my-job --local --file batches/sample-batch.json")
		assert.NotContains(t, stdout, "GitHub App")
	})

	t.Run("unsafe directory avoids shell interpolation", func(t *testing.T) {
		stdout, stderr := captureStandardStreams(t, func() {
			printJobCreationSuccess(TemplateOptions{
				Directory:    "$(unsafe path)",
				TemplateName: jobPythonTemplate,
			})
		})

		assert.Empty(t, stderr)
		assert.NotContains(t, stdout, "$(unsafe path)")
		assert.Contains(t, stdout, "Open the created project directory")
		assert.Contains(t, stdout, "bl run job JOB_NAME --local --file batches/sample-batch.json")
	})

	t.Run("github runner prints setup instructions", func(t *testing.T) {
		stdout, stderr := captureStandardStreams(t, func() {
			printJobCreationSuccess(TemplateOptions{
				Directory:    "my-runner",
				TemplateName: jobGithubRunnerTemplate,
			})
		})

		assert.Empty(t, stderr)
		assert.Contains(t, stdout, "Your blaxel job has been created successfully")
		assert.Contains(t, stdout, "cd my-runner")
		assert.Contains(t, stdout, "owner/repo")
		assert.Contains(t, stdout, "2. Run bl deploy\n")
		assert.Contains(t, stdout, "3. Install the Blaxel GitHub App")
		assert.Contains(t, stdout, "4. Run bl deploy --skip-build")
		assert.Less(t, strings.Index(stdout, "2. Run bl deploy"), strings.Index(stdout, "3. Install"))
		assert.Less(t, strings.Index(stdout, "3. Install"), strings.Index(stdout, "4. Run bl deploy --skip-build"))
		assert.NotContains(t, stdout, "sample-batch.json")
	})
}

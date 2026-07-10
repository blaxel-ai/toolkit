package core

import (
	"fmt"
	"os"
	"strings"
	"unicode"

	"github.com/charmbracelet/huh"
)

const (
	jobPythonTemplate       = "template-jobs-py"
	jobTypescriptTemplate   = "template-jobs-ts"
	jobGithubRunnerTemplate = "template-github-runner"
	jobBlankChoice          = "blank"
	jobGithubRunnerChoice   = "github-runner"
)

type jobTemplateChoice struct {
	label string
	value string
}

type jobTemplatePromptDeps struct {
	selectJobType   func(directory string, choices []jobTemplateChoice) (string, string, error)
	promptTemplates func(directory string, templates Templates) TemplateOptions
}

func findJobTemplate(templates Templates, canonicalName string) (Template, bool) {
	for _, template := range templates {
		if canonicalTemplateName(template.Name) == canonicalName {
			return template, true
		}
	}
	return Template{}, false
}

func blankJobTemplates(templates Templates) Templates {
	blank := Templates{}
	for _, template := range templates {
		name := canonicalTemplateName(template.Name)
		if name == jobPythonTemplate || name == jobTypescriptTemplate {
			blank = append(blank, template)
		}
	}
	return blank
}

func jobTemplateChoices(templates Templates) []jobTemplateChoice {
	choices := []jobTemplateChoice{}
	if len(blankJobTemplates(templates)) > 0 {
		choices = append(choices, jobTemplateChoice{label: "Blank", value: jobBlankChoice})
	}
	if _, ok := findJobTemplate(templates, jobGithubRunnerTemplate); ok {
		choices = append(choices, jobTemplateChoice{label: "GitHub Runner", value: jobGithubRunnerChoice})
	}
	return choices
}

func jobTemplatesForDisplay(templates Templates) Templates {
	ordered := blankJobTemplates(templates)
	seen := map[string]bool{}
	for _, template := range ordered {
		seen[template.Name] = true
	}

	if runner, ok := findJobTemplate(templates, jobGithubRunnerTemplate); ok {
		ordered = append(ordered, runner)
		seen[runner.Name] = true
	}

	for _, template := range templates {
		if !seen[template.Name] {
			ordered = append(ordered, template)
		}
	}
	return ordered
}

func validateJobTemplateOptions(opts TemplateOptions) error {
	switch canonicalTemplateName(opts.TemplateName) {
	case jobGithubRunnerTemplate:
		if opts.Language != "" {
			return fmt.Errorf("github runner template must not declare a language, got %q", opts.Language)
		}
	case jobPythonTemplate:
		if opts.Language != "python" {
			return fmt.Errorf("python job template must declare language %q, got %q", "python", opts.Language)
		}
	case jobTypescriptTemplate:
		if opts.Language != "typescript" {
			return fmt.Errorf("typescript job template must declare language %q, got %q", "typescript", opts.Language)
		}
	}
	return nil
}

func PromptJobTemplateOptions(directory string, templates Templates) TemplateOptions {
	opts, err := promptJobTemplateOptionsWithDeps(directory, templates, jobTemplatePromptDeps{
		selectJobType: selectJobType,
		promptTemplates: func(directory string, templates Templates) TemplateOptions {
			return promptTemplateOptions(directory, templates, "job", true, 5, jobTemplateOptionLabel)
		},
	})
	if err != nil {
		os.Exit(0)
	}
	return opts
}

func promptJobTemplateOptionsWithDeps(directory string, templates Templates, deps jobTemplatePromptDeps) (TemplateOptions, error) {
	choices := jobTemplateChoices(templates)
	if len(choices) == 0 {
		return deps.promptTemplates(directory, templates), nil
	}
	if len(choices) == 1 {
		if choices[0].value == jobBlankChoice {
			return deps.promptTemplates(directory, blankJobTemplates(templates)), nil
		}
		return CreateDefaultTemplateOptions(directory, jobGithubRunnerTemplate, templates), nil
	}

	selectedType, selectedDirectory, err := deps.selectJobType(directory, choices)
	if err != nil {
		return TemplateOptions{}, err
	}
	if selectedType == jobBlankChoice {
		return deps.promptTemplates(selectedDirectory, blankJobTemplates(templates)), nil
	}
	if selectedType == jobGithubRunnerChoice {
		return CreateDefaultTemplateOptions(selectedDirectory, jobGithubRunnerTemplate, templates), nil
	}
	return TemplateOptions{}, fmt.Errorf("unknown job type %q", selectedType)
}

func selectJobType(directory string, choices []jobTemplateChoice) (string, string, error) {
	selectedType := choices[0].value
	fields := []huh.Field{}
	if directory == "" {
		fields = append(fields, huh.NewInput().
			Title("Name").
			Description("Name of your job").
			Value(&directory),
		)
	}

	typeOptions := make([]huh.Option[string], 0, len(choices))
	for _, choice := range choices {
		typeOptions = append(typeOptions, huh.NewOption(choice.label, choice.value))
	}
	fields = append(fields, huh.NewSelect[string]().
		Title("Job type").
		Description("Choose the job to scaffold.").
		Options(typeOptions...).
		Value(&selectedType),
	)

	form := huh.NewForm(huh.NewGroup(fields...))
	form.WithTheme(GetHuhTheme())
	if err := form.Run(); err != nil {
		return "", directory, err
	}
	return selectedType, directory, nil
}

func sanitizeJobTerminalText(value string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || isBidiControl(r) {
			return ' '
		}
		return r
	}, value)
}

func isBidiControl(r rune) bool {
	return r == '\u061c' || r == '\u200e' || r == '\u200f' ||
		(r >= '\u202a' && r <= '\u202e') ||
		(r >= '\u2066' && r <= '\u2069')
}

func printJobTemplateLine(t Template) {
	name := sanitizeJobTerminalText(strings.TrimPrefix(templateDisplayName(t), "template-"))
	description := sanitizeJobTerminalText(t.Description)
	if description != "" {
		PrintDiagnostic(fmt.Sprintf("  - %-30s %s", name, description))
		return
	}
	PrintDiagnostic(fmt.Sprintf("  - %s", name))
}

func jobTemplateOptionLabel(t Template) string {
	return sanitizeJobTerminalText(templateOptionLabel(t))
}

func isPortableJobName(value string) bool {
	if value == "" || strings.HasPrefix(value, "-") {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._-", r) {
			continue
		}
		return false
	}
	return true
}

func printJobCreationSuccess(opts TemplateOptions) {
	PrintSuccess("Your blaxel job has been created successfully!")
	changeDirectory := "  Open the created project directory."
	runJob := "  Run the sample locally with your job name:\n    bl run job JOB_NAME --local --file batches/sample-batch.json"
	if isPortableJobName(opts.Directory) {
		changeDirectory = "  cd " + opts.Directory
		runJob = "  bl run job " + opts.Directory + " --local --file batches/sample-batch.json"
	}
	if canonicalTemplateName(opts.TemplateName) == jobGithubRunnerTemplate {
		fmt.Printf(`Configure your GitHub Runner:
%s
  1. Set the allowed owner/repo values in [githubRunner] in blaxel.toml
  2. Run bl deploy
  3. Install the Blaxel GitHub App from the job settings in the Blaxel console
  4. Run bl deploy --skip-build
`, changeDirectory)
		return
	}

	fmt.Printf(`Start working on it:
%s
%s
`, changeDirectory, runJob)
}

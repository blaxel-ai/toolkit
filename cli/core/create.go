package core

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/user"
	"regexp"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
	"gopkg.in/yaml.v3"
)

// RandomString generates a random alphanumeric string of the given length.
func RandomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"

	randomStr := make([]byte, length)
	for i := range randomStr {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			panic(fmt.Sprintf("failed to generate random string: %v", err))
		}
		randomStr[i] = charset[n.Int64()]
	}

	return string(randomStr)
}

// generateRandomDirectoryName creates a random directory name with the resource type prefix
// Format: {resourceType}-{5-random-chars}
func generateRandomDirectoryName(resourceType string) string {
	return fmt.Sprintf("%s-%s", resourceType, RandomString(5))
}

// CreateFlowConfig captures the knobs that differ between create commands.
type CreateFlowConfig struct {
	// Template topic used to filter templates: e.g., "agent", "job", "mcp", "sandbox".
	TemplateType string
	// Non-interactive flag; true means skip TTY UI and use defaults.
	NoTTY bool
	// Name used for progress/error messages, like "Agent creation".
	ErrorPrefix string
	// One-line spinner title while cloning, like "Creating your blaxel agent app...".
	SpinnerTitle string
	// Optional: when set, append a section to blaxel.toml with this resource type (e.g., "agent" or "function").
	BlaxelTomlResourceType string
}

type createFlowDeps struct {
	RetrieveTemplates func(templateType string, noTTY bool, errorPrefix string) (Templates, error)
	CloneTemplate     func(opts TemplateOptions, templates Templates, noTTY bool, errorPrefix string, spinnerTitle string) error
	CleanTemplate     func(directory string)
	EditBlaxelToml    func(resourceType string, projectName string, directory string) error
	OutputFormat      func() string
}

func defaultCreateFlowDeps() createFlowDeps {
	return createFlowDeps{
		RetrieveTemplates: RetrieveTemplatesWithSpinner,
		CloneTemplate:     CloneTemplateWithSpinner,
		CleanTemplate:     CleanTemplate,
		EditBlaxelToml:    EditBlaxelTomlInCurrentDir,
		OutputFormat:      GetOutputFormat,
	}
}

func fillCreateFlowDeps(deps createFlowDeps) createFlowDeps {
	defaults := defaultCreateFlowDeps()
	if deps.RetrieveTemplates == nil {
		deps.RetrieveTemplates = defaults.RetrieveTemplates
	}
	if deps.CloneTemplate == nil {
		deps.CloneTemplate = defaults.CloneTemplate
	}
	if deps.CleanTemplate == nil {
		deps.CleanTemplate = defaults.CleanTemplate
	}
	if deps.EditBlaxelToml == nil {
		deps.EditBlaxelToml = defaults.EditBlaxelToml
	}
	if deps.OutputFormat == nil {
		deps.OutputFormat = defaults.OutputFormat
	}
	return deps
}

// runCreateFlow centralizes the common steps for all create-* commands while
// preserving each command's specific behavior via the config and callbacks.
//
// Parameters:
// - dirArg: optional directory argument provided on CLI
// - templateNameFlag: optional template name provided via --template
// - promptFunc: called to interactively collect TemplateOptions when needed
// - successFunc: called at the end to print any command-specific instructions
func runCreateFlow(
	dirArg string,
	templateNameFlag string,
	cfg CreateFlowConfig,
	promptFunc func(directory string, templates Templates) TemplateOptions,
	successFunc func(opts TemplateOptions),
) {
	if err := runCreateFlowWithDeps(dirArg, templateNameFlag, cfg, promptFunc, successFunc, defaultCreateFlowDeps()); err != nil {
		ExitWithError(err)
	}
}

func runCreateFlowWithDeps(
	dirArg string,
	templateNameFlag string,
	cfg CreateFlowConfig,
	promptFunc func(directory string, templates Templates) TemplateOptions,
	successFunc func(opts TemplateOptions),
	deps createFlowDeps,
) error {
	deps = fillCreateFlowDeps(deps)

	if templateNameFlag != "" {
		templateNameFlag = normalizeTemplateNameFlag(templateNameFlag, cfg.TemplateType)
	}
	if dirArg == "" {
		dirArg = generateRandomDirectoryName(cfg.TemplateType)
	}

	// If directory arg provided, ensure it doesn't already exist
	if dirArg != "" {
		if _, err := os.Stat(dirArg); !os.IsNotExist(err) {
			createErr := fmt.Errorf("directory '%s' already exists", dirArg)
			PrintError(cfg.ErrorPrefix, createErr)
			return createErr
		}
	}

	// Retrieve templates (with or without spinner)
	templates, err := deps.RetrieveTemplates(cfg.TemplateType, cfg.NoTTY, cfg.ErrorPrefix)
	if err != nil {
		return err
	}

	// Resolve options
	var opts TemplateOptions
	switch {
	case templateNameFlag != "":
		// Template provided via flag. Determine directory (dirArg or templateName) and ensure it's free
		selectedDir := dirArg
		if selectedDir == "" {
			selectedDir = templateNameFlag
		}
		if _, err := os.Stat(selectedDir); !os.IsNotExist(err) {
			createErr := fmt.Errorf("directory '%s' already exists", selectedDir)
			PrintError(cfg.ErrorPrefix, createErr)
			return createErr
		}
		opts = CreateDefaultTemplateOptions(selectedDir, templateNameFlag, templates)
		if opts.TemplateName == "" {
			createErr := fmt.Errorf("template '%s' not found", templateNameFlag)
			PrintError(cfg.ErrorPrefix, createErr)
			printAvailableTemplates(templates, cfg.TemplateType)
			return createErr
		}
	case cfg.NoTTY && cfg.TemplateType == "mcp":
		// Special-case retained behavior: for MCP with --yes but no template we require directory and pick default
		if dirArg == "" {
			createErr := fmt.Errorf("directory name is required")
			PrintError(cfg.ErrorPrefix, createErr)
			return createErr
		}
		opts = CreateDefaultTemplateOptions(dirArg, "", templates)
	default:
		// Interactive prompt
		opts = promptFunc(dirArg, templates)
		// Safety checks
		if opts.Directory == "" {
			createErr := fmt.Errorf("directory name is required")
			PrintError(cfg.ErrorPrefix, createErr)
			return createErr
		}
		if _, err := os.Stat(opts.Directory); !os.IsNotExist(err) {
			createErr := fmt.Errorf("directory '%s' already exists", opts.Directory)
			PrintError(cfg.ErrorPrefix, createErr)
			return createErr
		}
	}

	// Clone template using the unified helper
	if err := deps.CloneTemplate(opts, templates, cfg.NoTTY, cfg.ErrorPrefix, cfg.SpinnerTitle); err != nil {
		return err
	}

	deps.CleanTemplate(opts.Directory)

	if cfg.TemplateType == "sandbox" {
		if err := FinalizeSandboxTemplate(opts); err != nil {
			PrintError(cfg.ErrorPrefix, err)
			return err
		}
	}

	// Optionally update blaxel.toml (only for those commands that did previously)
	if cfg.BlaxelTomlResourceType != "" {
		if err := deps.EditBlaxelToml(cfg.BlaxelTomlResourceType, opts.ProjectName, opts.Directory); err != nil {
			PrintError(cfg.ErrorPrefix, err)
			return err
		}
	}

	// If structured output is requested, print JSON/YAML and skip the regular success message
	outputFmt := deps.OutputFormat()
	if outputFmt == "json" || outputFmt == "yaml" {
		result := map[string]string{
			"directory": opts.Directory,
			"template":  opts.TemplateName,
			"language":  opts.Language,
			"type":      cfg.TemplateType,
		}
		switch outputFmt {
		case "json":
			data, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(data))
		case "yaml":
			data, _ := yaml.Marshal(result)
			fmt.Print(string(data))
		}
		return nil
	}

	// Let the caller print specific success instructions
	successFunc(opts)
	return nil
}

func normalizeTemplateNameFlag(templateNameFlag string, templateType string) string {
	if templateType == "sandbox" {
		if templateName, ok := sandboxTemplateAlias(templateNameFlag); ok {
			return templateName
		}
	}
	if !strings.HasPrefix(templateNameFlag, "template-") {
		return "template-" + templateNameFlag
	}
	return templateNameFlag
}

func templateDisplayName(t Template) string {
	return regexp.MustCompile(`^\d+-`).ReplaceAllString(t.Name, "")
}

func printAvailableTemplates(templates Templates, templateType string) {
	PrintDiagnostic("Available templates:")
	if templateType == "sandbox" {
		for _, t := range sandboxTemplatesForDisplay(templates) {
			printSandboxTemplateLine(t)
		}
		return
	}

	langs := templates.GetLanguages()
	if len(langs) == 0 {
		for _, t := range templates {
			printTemplateLine(t)
		}
		return
	}
	for _, lang := range langs {
		hasTemplates := false
		for _, t := range templates {
			if t.Language != lang {
				continue
			}
			if !hasTemplates {
				PrintDiagnostic(fmt.Sprintf("- %s:", lang))
				hasTemplates = true
			}
			printTemplateLine(t)
		}
	}
}

func printTemplateLine(t Template) {
	name := strings.TrimPrefix(templateDisplayName(t), "template-")
	if t.Description != "" {
		PrintDiagnostic(fmt.Sprintf("  - %-30s %s", name, t.Description))
	} else {
		PrintDiagnostic(fmt.Sprintf("  - %s", name))
	}
}

// PromptTemplateOptions presents a unified interactive form to collect
// TemplateOptions. It can optionally include a language selector and will
// always include a template selector. The name field is included only when
// directory is empty.
// resource is used in messages, e.g. "agent app", "job", "mcp server".
func PromptTemplateOptions(directory string, templates Templates, resource string, includeLanguage bool, templateHeight int) TemplateOptions {
	options := TemplateOptions{
		ProjectName:  directory,
		Directory:    directory,
		TemplateName: "",
	}

	currentUser, err := user.Current()
	if err == nil {
		options.Author = currentUser.Username
	} else {
		options.Author = "blaxel"
	}

	stripRe := regexp.MustCompile(`^\d+-`)
	isBlank := func(t Template) bool {
		name := strings.ToLower(stripRe.ReplaceAllString(t.Name, ""))
		return strings.Contains(name, "template-blank") || strings.HasPrefix(name, "blank-") || name == "blank"
	}

	totalTemplates := len(templates)
	languages := templates.GetLanguages()

	// If there is only one template overall, we can auto-select everything
	if totalTemplates == 1 {
		// Ask for name if needed
		if directory == "" {
			form := huh.NewForm(huh.NewGroup(
				huh.NewInput().
					Title("Name").
					Description("Name of your " + resource).
					Value(&options.Directory),
			))
			form.WithTheme(GetHuhTheme())
			if err := form.Run(); err != nil {
				os.Exit(0)
			}
		}
		options.Language = templates[0].Language
		options.TemplateName = templates[0].Name
		options.ProjectName = options.Directory
		return options
	}

	// First form: name (if needed) and language if multiple languages
	initialFields := []huh.Field{}
	if directory == "" {
		initialFields = append(initialFields, huh.NewInput().
			Title("Name").
			Description("Name of your "+resource).
			Value(&options.Directory),
		)
	}
	if includeLanguage && len(languages) > 1 {
		langOptions := []huh.Option[string]{}
		for _, lang := range languages {
			langOptions = append(langOptions, huh.NewOption(lang, lang))
		}
		initialFields = append(initialFields, huh.NewSelect[string]().
			Title("Language").
			Description("Language to use for your "+resource).
			Options(langOptions...).
			Value(&options.Language),
		)
	}

	// Decide if any blank exists globally to include start choice in the first form
	anyHasBlank := false
	for _, t := range templates {
		if isBlank(t) {
			anyHasBlank = true
			break
		}
	}
	startChoice := "template"
	if anyHasBlank {
		startChoice = "plain"
		initialFields = append(initialFields, huh.NewSelect[string]().
			Title("Start from").
			Description("Choose how to create your "+resource).
			Options(
				huh.NewOption("From scratch", "plain"),
				huh.NewOption("From a template", "template"),
			).
			Value(&startChoice),
		)
	}

	if len(initialFields) > 0 {
		form := huh.NewForm(huh.NewGroup(initialFields...))
		form.WithTheme(GetHuhTheme())
		if err := form.Run(); err != nil {
			os.Exit(0)
		}
	}

	if options.Language == "" && len(languages) > 0 {
		options.Language = languages[0]
	}

	// Determine templates for selected language
	filtered := templates.FilterByLanguage(options.Language)

	// If exactly one template for language, auto-select it
	if len(filtered) == 1 {
		options.TemplateName = filtered[0].Name
		options.ProjectName = options.Directory
		return options
	}

	// Multiple templates for language
	var blankTemplate *Template
	for idx := range filtered {
		if isBlank(filtered[idx]) {
			blankTemplate = &filtered[idx]
			break
		}
	}

	if startChoice == "plain" && blankTemplate != nil {
		options.TemplateName = blankTemplate.Name
		options.ProjectName = options.Directory
		return options
	}

	// Second form: pick a non-blank template with loader
	var templateOptions []huh.Option[string]
	_ = spinner.New().
		Title("Loading templates...").
		Action(func() {
			for _, t := range filtered {
				if isBlank(t) {
					continue
				}
				key := stripRe.ReplaceAllString(t.Name, "")
				key = strings.TrimPrefix(key, "template-")
				templateOptions = append(templateOptions, huh.NewOption(key, t.Name))
			}
		}).
		Run()
	tmplSelect := huh.NewSelect[string]().
		Title("Template").
		Description("Template to use for your " + resource).
		Options(templateOptions...).
		Value(&options.TemplateName)
	// Only set Height when there are enough options to need scrolling;
	// setting Height triggers a huh viewport bug that hides earlier options.
	if len(templateOptions) > templateHeight {
		tmplSelect = tmplSelect.Height(templateHeight)
	}
	pick := huh.NewForm(huh.NewGroup(tmplSelect))
	pick.WithTheme(GetHuhTheme())
	if err := pick.Run(); err != nil {
		os.Exit(0)
	}
	options.ProjectName = options.Directory
	return options
}

// RunSandboxCreation is a reusable wrapper that executes the sandbox creation flow.
func RunSandboxCreation(dirArg string, templateName string, noTTY bool) {
	runCreateFlow(
		dirArg,
		templateName,
		CreateFlowConfig{
			TemplateType: "sandbox",
			NoTTY:        noTTY,
			ErrorPrefix:  "Sandbox creation",
			SpinnerTitle: "Creating your blaxel sandbox...",
		},
		func(directory string, templates Templates) TemplateOptions {
			return PromptTemplateOptions(directory, templates, "sandbox", false, 5)
		},
		func(opts TemplateOptions) {
			PrintSuccess("Your blaxel sandbox has been created successfully!")
			fmt.Printf(`Start working on it:
  cd %s
  bl deploy
`, opts.Directory)
		},
	)
}

// RunAgentAppCreation is a reusable wrapper that executes the agent creation flow.
// It can be called by both the dedicated command and the unified `bl new` command.
func RunAgentAppCreation(dirArg string, templateName string, noTTY bool) {
	runCreateFlow(
		dirArg,
		templateName,
		CreateFlowConfig{
			TemplateType:           "agent",
			NoTTY:                  noTTY,
			ErrorPrefix:            "Agent creation",
			SpinnerTitle:           "Creating your blaxel agent app...",
			BlaxelTomlResourceType: "agent",
		},
		func(directory string, templates Templates) TemplateOptions {
			return PromptTemplateOptions(directory, templates, "agent app", true, 12)
		},
		func(opts TemplateOptions) {
			PrintSuccess("Your blaxel agent app has been created successfully!")
			fmt.Printf(`Start working on it:
  cd %s
  bl serve --hotreload
`, opts.Directory)
		},
	)
}

// RunJobCreation is a reusable wrapper that executes the job creation flow.
func RunJobCreation(dirArg string, templateName string, noTTY bool) {
	runCreateFlow(
		dirArg,
		templateName,
		CreateFlowConfig{
			TemplateType: "job",
			NoTTY:        noTTY,
			ErrorPrefix:  "Job creation",
			SpinnerTitle: "Creating your blaxel job...",
		},
		func(directory string, templates Templates) TemplateOptions {
			return PromptTemplateOptions(directory, templates, "job", true, 5)
		},
		func(opts TemplateOptions) {
			PrintSuccess("Your blaxel job has been created successfully!")
			fmt.Printf(`Start working on it:
  cd %s
  bl run job %s --local --file batches/sample-batch.json
`, opts.Directory, opts.Directory)
		},
	)
}

// RunMCPCreation is a reusable wrapper that executes the MCP server creation flow.
func RunMCPCreation(dirArg string, templateName string, noTTY bool) {
	runCreateFlow(
		dirArg,
		templateName,
		CreateFlowConfig{
			TemplateType:           "mcp",
			NoTTY:                  noTTY,
			ErrorPrefix:            "MCP Server creation",
			SpinnerTitle:           "Creating your blaxel mcp server...",
			BlaxelTomlResourceType: "function",
		},
		func(directory string, templates Templates) TemplateOptions {
			return PromptTemplateOptions(directory, templates, "mcp server", true, 5)
		},
		func(opts TemplateOptions) {
			PrintSuccess("Your blaxel MCP server has been created successfully!")
			fmt.Printf(`Start working on it:
  cd %s
  bl serve --hotreload
`, opts.Directory)
		},
	)
}

// RunVolumeTemplateCreation is a reusable wrapper that executes the volume template creation flow.
func RunVolumeTemplateCreation(dirArg string, templateName string, noTTY bool) {
	runCreateFlow(
		dirArg,
		templateName,
		CreateFlowConfig{
			TemplateType: "volume-template",
			NoTTY:        noTTY,
			ErrorPrefix:  "Volume template creation",
			SpinnerTitle: "Creating your blaxel volume template...",
		},
		func(directory string, templates Templates) TemplateOptions {
			return PromptTemplateOptions(directory, templates, "volume template", false, 5)
		},
		func(opts TemplateOptions) {
			PrintSuccess("Your blaxel volume template has been created successfully!")
			fmt.Printf(`Start working on it:
  cd %s
  bl deploy
`, opts.Directory)
		},
	)
}

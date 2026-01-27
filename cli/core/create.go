package core

import (
	"fmt"
	"math/rand"
	"os"
	"os/user"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
)

// generateRandomDirectoryName creates a random directory name with the resource type prefix
// Format: {resourceType}-{5-random-chars}
func generateRandomDirectoryName(resourceType string) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	const length = 5

	// Initialize random seed
	rnd := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Generate random string
	randomStr := make([]byte, length)
	for i := range randomStr {
		randomStr[i] = charset[rnd.Intn(len(charset))]
	}

	return fmt.Sprintf("%s-%s", resourceType, string(randomStr))
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
	// Accept shorthand template names without the "template-" prefix
	if templateNameFlag != "" && !strings.HasPrefix(templateNameFlag, "template-") {
		templateNameFlag = "template-" + templateNameFlag
	}
	if dirArg == "" {
		dirArg = generateRandomDirectoryName(cfg.TemplateType)
	}

	// If directory arg provided, ensure it doesn't already exist
	if dirArg != "" {
		if _, err := os.Stat(dirArg); !os.IsNotExist(err) {
			PrintError(cfg.ErrorPrefix, fmt.Errorf("directory '%s' already exists", dirArg))
			return
		}
	}

	// Retrieve templates (with or without spinner)
	templates, err := RetrieveTemplatesWithSpinner(cfg.TemplateType, cfg.NoTTY, cfg.ErrorPrefix)
	if err != nil {
		ExitWithError(err)
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
			PrintError(cfg.ErrorPrefix, fmt.Errorf("directory '%s' already exists", selectedDir))
			return
		}
		opts = CreateDefaultTemplateOptions(selectedDir, templateNameFlag, templates)
		if opts.TemplateName == "" {
			PrintError(cfg.ErrorPrefix, fmt.Errorf("template '%s' not found", templateNameFlag))
			fmt.Println("Available templates:")
			langs := templates.GetLanguages()
			for _, lang := range langs {
				names := []string{}
				for _, t := range templates {
					if t.Language != lang {
						continue
					}
					name := strings.TrimPrefix(templateDisplayName(t), "template-")
					names = append(names, name)
				}
				if len(names) == 0 {
					continue
				}
				fmt.Printf("- %s:\n", lang)
				for _, n := range names {
					fmt.Printf("  - %s\n", n)
				}
			}
			return
		}
	case cfg.NoTTY && cfg.TemplateType == "mcp":
		// Special-case retained behavior: for MCP with --yes but no template we require directory and pick default
		if dirArg == "" {
			PrintError(cfg.ErrorPrefix, fmt.Errorf("directory name is required"))
			return
		}
		opts = CreateDefaultTemplateOptions(dirArg, "", templates)
	default:
		// Interactive prompt
		opts = promptFunc(dirArg, templates)
		// Safety checks
		if opts.Directory == "" {
			PrintError(cfg.ErrorPrefix, fmt.Errorf("directory name is required"))
			return
		}
		if _, err := os.Stat(opts.Directory); !os.IsNotExist(err) {
			PrintError(cfg.ErrorPrefix, fmt.Errorf("directory '%s' already exists", opts.Directory))
			return
		}
	}

	// Clone template using the unified helper
	if err := CloneTemplateWithSpinner(opts, templates, cfg.NoTTY, cfg.ErrorPrefix, cfg.SpinnerTitle); err != nil {
		return
	}

	CleanTemplate(opts.Directory)

	// Optionally update blaxel.toml (only for those commands that did previously)
	if cfg.BlaxelTomlResourceType != "" {
		_ = EditBlaxelTomlInCurrentDir(cfg.BlaxelTomlResourceType, opts.ProjectName, opts.Directory)
	}

	// Let the caller print specific success instructions
	successFunc(opts)
}

func templateDisplayName(t Template) string {
	return regexp.MustCompile(`^\d+-`).ReplaceAllString(t.Name, "")
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
			Height(5).
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
	pick := huh.NewForm(huh.NewGroup(
		huh.NewSelect[string]().
			Title("Template").
			Description("Template to use for your " + resource).
			Height(templateHeight).
			Options(templateOptions...).
			Value(&options.TemplateName),
	))
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

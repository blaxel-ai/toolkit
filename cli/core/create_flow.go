package core

import (
	"fmt"
	"os"
	"os/user"
	"regexp"

	"github.com/charmbracelet/huh"
)

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

// RunCreateFlow centralizes the common steps for all create-* commands while
// preserving each command's specific behavior via the config and callbacks.
//
// Parameters:
// - dirArg: optional directory argument provided on CLI
// - templateNameFlag: optional template name provided via --template
// - promptFunc: called to interactively collect TemplateOptions when needed
// - successFunc: called at the end to print any command-specific instructions
func RunCreateFlow(
	dirArg string,
	templateNameFlag string,
	cfg CreateFlowConfig,
	promptFunc func(directory string, templates Templates) TemplateOptions,
	successFunc func(opts TemplateOptions),
) {
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
		os.Exit(1)
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
			for _, t := range templates {
				key := templateDisplayName(t)
				fmt.Printf("  %s (%s)\n", key, t.Language)
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
	return regexp.MustCompile(`^\d+-`).ReplaceAllString(*t.Name, "")
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

	fields := []huh.Field{}
	if directory == "" {
		fields = append(fields, huh.NewInput().
			Title("Name").
			Description("Name of your "+resource).
			Value(&options.Directory),
		)
	}

	if includeLanguage {
		languagesOptions := []huh.Option[string]{}
		for _, language := range templates.GetLanguages() {
			languagesOptions = append(languagesOptions, huh.NewOption(language, language))
		}
		fields = append(fields,
			huh.NewSelect[string]().
				Title("Language").
				Description("Language to use for your "+resource).
				Height(5).
				Options(languagesOptions...).
				Value(&options.Language),
		)
		fields = append(fields,
			huh.NewSelect[string]().
				Title("Template").
				Description("Template to use for your "+resource).
				Height(templateHeight).
				OptionsFunc(func() []huh.Option[string] {
					filtered := templates.FilterByLanguage(options.Language)
					if len(filtered) == 0 {
						return []huh.Option[string]{}
					}
					result := []huh.Option[string]{}
					for _, t := range filtered {
						key := regexp.MustCompile(`^\\d+-`).ReplaceAllString(*t.Name, "")
						result = append(result, huh.NewOption(key, *t.Name))
					}
					return result
				}, &options).
				Value(&options.TemplateName),
		)
	} else {
		fields = append(fields,
			huh.NewSelect[string]().
				Title("Template").
				Description("Template to use for your "+resource).
				Height(templateHeight).
				OptionsFunc(func() []huh.Option[string] {
					if len(templates) == 0 {
						return []huh.Option[string]{}
					}
					result := []huh.Option[string]{}
					for _, t := range templates {
						key := regexp.MustCompile(`^\\d+-`).ReplaceAllString(*t.Name, "")
						result = append(result, huh.NewOption(key, *t.Name))
					}
					return result
				}, &options).
				Value(&options.TemplateName),
		)
	}

	form := huh.NewForm(huh.NewGroup(fields...))
	form.WithTheme(GetHuhTheme())
	err = form.Run()
	if err != nil {
		fmt.Println("Cancel create blaxel " + resource)
		os.Exit(0)
	}

	options.ProjectName = options.Directory
	return options
}

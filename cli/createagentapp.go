package cli

import (
	"fmt"
	"os"
	"os/user"
	"regexp"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("create-agent-app", func() *cobra.Command {
		return CreateAgentAppCmd()
	})
}

// CreateAgentAppCmd returns a cobra.Command that implements the 'create-agent-app' CLI command.
// The command creates a new Blaxel agent app in the specified directory after collecting
// necessary configuration through an interactive prompt.
// Usage: bl create-agent-app directory [--template template-name]
func CreateAgentAppCmd() *cobra.Command {
	var templateName string
	var noTTY bool

	cmd := &cobra.Command{
		Use:     "create-agent-app directory",
		Args:    cobra.MaximumNArgs(2),
		Aliases: []string{"ca", "caa"},
		Short:   "Create a new blaxel agent app",
		Long:    "Create a new blaxel agent app",
		Example: `
bl create-agent-app my-agent-app
bl create-agent-app my-agent-app --template template-google-adk-py
bl create-agent-app my-agent-app --template template-google-adk-py -y`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				core.PrintError("Agent creation", fmt.Errorf("directory name is required"))
				return
			}
			// Check if directory already exists
			if _, err := os.Stat(args[0]); !os.IsNotExist(err) {
				core.PrintError("Agent creation", fmt.Errorf("directory '%s' already exists", args[0]))
				return
			}

			// Retrieve templates using the new reusable function
			templates, err := core.RetrieveTemplatesWithSpinner("agent", noTTY, "Agent creation")
			if err != nil {
				os.Exit(1)
			}

			var opts core.TemplateOptions
			// If template is specified via flag or skip prompts is enabled, skip interactive prompt
			if templateName != "" {
				opts = core.CreateDefaultTemplateOptions(args[0], templateName, templates)
				if opts.TemplateName == "" {
					core.PrintError("Agent creation", fmt.Errorf("template '%s' not found", templateName))
					fmt.Println("Available templates:")
					for _, template := range templates {
						key := regexp.MustCompile(`^\d+-`).ReplaceAllString(*template.Name, "")
						fmt.Printf("  %s (%s)\n", key, template.Language)
					}
					return
				}
			} else {
				opts = promptCreateAgentApp(args[0], templates)
			}

			// Clone template using the new reusable function
			if err := core.CloneTemplateWithSpinner(opts, templates, noTTY, "Agent creation", "Creating your blaxel agent app..."); err != nil {
				return
			}

			core.CleanTemplate(opts.Directory)
			_ = core.EditBlaxelTomlInCurrentDir("agent", opts.ProjectName, opts.Directory)

			// Success message with colors
			core.PrintSuccess("Your blaxel agent app has been created successfully!")
			fmt.Printf(`Start working on it:
  cd %s
  bl serve --hotreload
`, opts.Directory)
		},
	}

	cmd.Flags().StringVarP(&templateName, "template", "t", "", "Template to use for the agent app (skips interactive prompt)")
	cmd.Flags().BoolVarP(&noTTY, "yes", "y", false, "Skip interactive prompts and use defaults")
	return cmd
}

// promptCreateAgentApp displays an interactive form to collect user input for creating a new agent app.
// It prompts for project name, model selection, template, author, license, and additional features.
// Takes a directory string parameter and returns a CreateAgentAppOptions struct with the user's selections.
func promptCreateAgentApp(directory string, templates core.Templates) core.TemplateOptions {
	agentAppOptions := core.TemplateOptions{
		ProjectName:  directory,
		Directory:    directory,
		TemplateName: "",
	}
	currentUser, err := user.Current()
	if err == nil {
		agentAppOptions.Author = currentUser.Username
	} else {
		agentAppOptions.Author = "blaxel"
	}
	languagesOptions := []huh.Option[string]{}
	for _, language := range templates.GetLanguages() {
		languagesOptions = append(languagesOptions, huh.NewOption(language, language))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project Name").
				Description("Name of your agent app").
				Value(&agentAppOptions.ProjectName),
			huh.NewSelect[string]().
				Title("Language").
				Description("Language to use for your agent app").
				Height(5).
				Options(languagesOptions...).
				Value(&agentAppOptions.Language),
			huh.NewSelect[string]().
				Title("Template").
				Description("Template to use for your agent app").
				Height(12).
				OptionsFunc(func() []huh.Option[string] {
					templates := templates.FilterByLanguage(agentAppOptions.Language)
					if len(templates) == 0 {
						return []huh.Option[string]{}
					}
					options := []huh.Option[string]{}
					for _, template := range templates {
						key := regexp.MustCompile(`^\d+-`).ReplaceAllString(*template.Name, "")
						options = append(options, huh.NewOption(key, *template.Name))
					}
					return options
				}, &agentAppOptions).
				Value(&agentAppOptions.TemplateName),
		),
	)
	form.WithTheme(core.GetHuhTheme())
	err = form.Run()
	if err != nil {
		fmt.Println("Cancel create blaxel agent app")
		os.Exit(0)
	}
	return agentAppOptions
}

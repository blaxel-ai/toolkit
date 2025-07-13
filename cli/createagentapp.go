package cli

import (
	"fmt"
	"os"
	"os/user"
	"regexp"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/huh/spinner"
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
				fmt.Println("Please provide a directory name")
				return
			}
			// Check if directory already exists
			if _, err := os.Stat(args[0]); !os.IsNotExist(err) {
				fmt.Printf("Error: %s already exists\n", args[0])
				return
			}

			var templateError error
			var templates core.Templates
			if noTTY {
				templates, templateError = core.RetrieveTemplates("agent")
				if templateError != nil {
					fmt.Println("Error creating agent app", templateError)
					os.Exit(0)
				}
			} else {
				spinnerErr := spinner.New().
					Title("Retrieving templates...").
					Action(func() {
						templates, templateError = core.RetrieveTemplates("agent")
					}).
					Run()
				if spinnerErr != nil {
					fmt.Println("Error creating agent app", spinnerErr)
					return
				}
				if templateError != nil {
					fmt.Println("Error creating agent app", templateError)
					os.Exit(0)
				}
			}

			if len(templates) == 0 {
				fmt.Println("No templates found")
				os.Exit(0)
			}

			var opts core.TemplateOptions
			// If template is specified via flag or skip prompts is enabled, skip interactive prompt
			if templateName != "" {
				opts = core.CreateDefaultTemplateOptions(args[0], templateName, templates)
				if opts.TemplateName == "" {
					fmt.Printf("Error: template '%s' not found\n", templateName)
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

			if noTTY {
				template, err := templates.Find(opts.TemplateName)
				if err != nil {
					fmt.Println("Error finding template", err)
					return
				}
				cloneError := template.Clone(opts)
				if cloneError != nil {
					fmt.Println("Error creating agent app", cloneError)
					_ = os.RemoveAll(opts.Directory)
					return
				}
			} else {
				var cloneError error
				spinnerErr := spinner.New().
					Title("Creating your blaxel agent app...").
					Action(func() {
						template, err := templates.Find(opts.TemplateName)
						if err != nil {
							fmt.Println("Error finding template", err)
							return
						}
						cloneError = template.Clone(opts)
					}).
					Run()
				if spinnerErr != nil {
					fmt.Println("Error creating agent app", spinnerErr)
					return
				}
				if cloneError != nil {
					fmt.Println("Error creating agent app", cloneError)
					_ = os.RemoveAll(opts.Directory)
					return
				}
			}
			core.CleanTemplate(opts.Directory)
			_ = core.EditBlaxelTomlInCurrentDir("agent", opts.ProjectName, opts.Directory)
			fmt.Printf(`Your blaxel agent app has been created. Start working on it:
cd %s;
bl serve --hotreload;
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

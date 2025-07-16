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
	core.RegisterCommand("create-sandbox", func() *cobra.Command {
		return CreateSandboxCmd()
	})
}

// CreateSandboxCmd returns a cobra.Command that implements the 'create-sandbox' CLI command.
// The command creates a new Blaxel sandbox in the specified directory after collecting
// necessary configuration through an interactive prompt.
// Usage: bl create-sandbox directory [--template template-name]
func CreateSandboxCmd() *cobra.Command {
	var templateName string
	var noTTY bool

	cmd := &cobra.Command{
		Use:     "create-sandbox directory",
		Args:    cobra.MaximumNArgs(2),
		Aliases: []string{"cs"},
		Short:   "Create a new blaxel sandbox",
		Long:    "Create a new blaxel sandbox",
		Example: `
bl create-sandbox my-sandbox
bl create-sandbox my-sandbox --template template-sandbox-ts
bl create-sandbox my-sandbox --template template-sandbox-ts -y`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				core.PrintError("Sandbox creation", fmt.Errorf("directory name is required"))
				return
			}
			// Check if directory already exists
			if _, err := os.Stat(args[0]); !os.IsNotExist(err) {
				core.PrintError("Sandbox creation", fmt.Errorf("directory '%s' already exists", args[0]))
				return
			}

			// Retrieve templates using the new reusable function
			templates, err := core.RetrieveTemplatesWithSpinner("sandbox", noTTY, "Sandbox creation")
			if err != nil {
				os.Exit(1)
			}

			var opts core.TemplateOptions
			// If template is specified via flag or skip prompts is enabled, skip interactive prompt
			if templateName != "" {
				opts = core.CreateDefaultTemplateOptions(args[0], templateName, templates)
				if opts.TemplateName == "" {
					core.PrintError("Sandbox creation", fmt.Errorf("template '%s' not found", templateName))
					fmt.Println("Available templates:")
					for _, template := range templates {
						key := regexp.MustCompile(`^\d+-`).ReplaceAllString(*template.Name, "")
						fmt.Printf("  %s (%s)\n", key, template.Language)
					}
					return
				}
			} else {
				opts = promptCreateSandbox(args[0], templates)
			}

			// Clone template using the new reusable function
			if err := core.CloneTemplateWithSpinner(opts, templates, noTTY, "Sandbox creation", "Creating your blaxel sandbox..."); err != nil {
				return
			}

			core.CleanTemplate(opts.Directory)

			// Success message with colors
			core.PrintSuccess("Your blaxel sandbox has been created successfully!")
			fmt.Printf(`Start working on it:
  cd %s
  bl deploy
`, opts.Directory)
		},
	}

	cmd.Flags().StringVarP(&templateName, "template", "t", "", "Template to use for the sandbox (skips interactive prompt)")
	cmd.Flags().BoolVarP(&noTTY, "yes", "y", false, "Skip interactive prompts and use defaults")
	return cmd
}

// promptCreateSandbox displays an interactive form to collect user input for creating a new sandbox.
// It prompts for project name, language selection, template, author, license, and additional features.
// Takes a directory string parameter and returns a TemplateOptions struct with the user's selections.
func promptCreateSandbox(directory string, templates core.Templates) core.TemplateOptions {
	sandboxOptions := core.TemplateOptions{
		ProjectName:  directory,
		Directory:    directory,
		TemplateName: "",
	}
	currentUser, err := user.Current()
	if err == nil {
		sandboxOptions.Author = currentUser.Username
	} else {
		sandboxOptions.Author = "blaxel"
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project Name").
				Description("Name of your sandbox").
				Value(&sandboxOptions.ProjectName),
			huh.NewSelect[string]().
				Title("Template").
				Description("Template to use for your sandbox").
				Height(5).
				OptionsFunc(func() []huh.Option[string] {
					if len(templates) == 0 {
						return []huh.Option[string]{}
					}
					options := []huh.Option[string]{}
					for _, template := range templates {
						key := regexp.MustCompile(`^\d+-`).ReplaceAllString(*template.Name, "")
						options = append(options, huh.NewOption(key, *template.Name))
					}
					return options
				}, &sandboxOptions).
				Value(&sandboxOptions.TemplateName),
		),
	)
	form.WithTheme(core.GetHuhTheme())
	err = form.Run()
	if err != nil {
		fmt.Println("Cancel create blaxel sandbox")
		os.Exit(0)
	}
	return sandboxOptions
}

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
	core.RegisterCommand("create-mcp-server", func() *cobra.Command {
		return CreateMCPServerCmd()
	})
}

// CreateMCPServerCmd returns a cobra.Command that implements the 'create-mcpserver' CLI command.
// The command creates a new Blaxel mcp server in the specified directory after collecting
// necessary configuration through an interactive prompt.
// Usage: bl create-mcp-server directory [--template template-name]
func CreateMCPServerCmd() *cobra.Command {
	var templateName string
	var noTTY bool

	cmd := &cobra.Command{
		Use:     "create-mcp-server directory",
		Args:    cobra.MaximumNArgs(2),
		Aliases: []string{"cm", "cms"},
		Short:   "Create a new blaxel mcp server",
		Long:    "Create a new blaxel mcp server",
		Example: `
bl create-mcp-server my-mcp-server
bl create-mcp-server my-mcp-server --template template-mcp-hello-world-py
bl create-mcp-server my-mcp-server --template template-mcp-hello-world-py -y`,
		Run: func(cmd *cobra.Command, args []string) {
			if len(args) < 1 {
				core.PrintError("MCP Server creation", fmt.Errorf("directory name is required"))
				return
			}
			// Check if directory already exists
			if _, err := os.Stat(args[0]); !os.IsNotExist(err) {
				core.PrintError("MCP Server creation", fmt.Errorf("directory '%s' already exists", args[0]))
				return
			}

			// Retrieve templates using the new reusable function
			templates, err := core.RetrieveTemplatesWithSpinner("mcp", noTTY, "MCP Server creation")
			if err != nil {
				os.Exit(1)
			}

			var opts core.TemplateOptions
			// If template is specified via flag or skip prompts is enabled, skip interactive prompt
			if templateName != "" {
				opts = core.CreateDefaultTemplateOptions(args[0], templateName, templates)
				if opts.TemplateName == "" {
					core.PrintError("MCP Server creation", fmt.Errorf("template '%s' not found", templateName))
					fmt.Println("Available templates:")
					for _, template := range templates {
						key := regexp.MustCompile(`^\d+-`).ReplaceAllString(*template.Name, "")
						fmt.Printf("  %s (%s)\n", key, template.Language)
					}
					return
				}
			} else if noTTY {
				// When noTTY is true but no template specified, use first available template
				opts = core.CreateDefaultTemplateOptions(args[0], "", templates)
			} else {
				opts = promptCreateMCPServer(args[0], templates)
			}

			// Clone template using the new reusable function
			if err := core.CloneTemplateWithSpinner(opts, templates, noTTY, "MCP Server creation", "Creating your blaxel mcp server..."); err != nil {
				return
			}

			core.CleanTemplate(opts.Directory)
			_ = core.EditBlaxelTomlInCurrentDir("function", opts.ProjectName, opts.Directory)

			// Success message with colors
			core.PrintSuccess("Your blaxel MCP server has been created successfully!")
			fmt.Printf(`Start working on it:
  cd %s
  bl serve --hotreload
`, opts.Directory)
		},
	}

	cmd.Flags().StringVarP(&templateName, "template", "t", "", "Template to use for the mcp server (skips interactive prompt)")
	cmd.Flags().BoolVarP(&noTTY, "yes", "y", false, "Skip interactive prompts and use defaults")
	return cmd
}

// promptCreateMCPServer displays an interactive form to collect user input for creating a new mcp server.
// It prompts for project name, language selection, template, author, license, and additional features.
// Takes a directory string parameter and templates, returns a CreateMCPServerOptions struct with the user's selections.
func promptCreateMCPServer(directory string, templates core.Templates) core.TemplateOptions {
	mcpserverOptions := core.TemplateOptions{
		ProjectName:  directory,
		Directory:    directory,
		TemplateName: "",
	}
	currentUser, err := user.Current()
	if err == nil {
		mcpserverOptions.Author = currentUser.Username
	} else {
		mcpserverOptions.Author = "blaxel"
	}
	languagesOptions := []huh.Option[string]{}
	for _, language := range templates.GetLanguages() {
		languagesOptions = append(languagesOptions, huh.NewOption(language, language))
	}
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Project Name").
				Description("Name of your mcp server").
				Value(&mcpserverOptions.ProjectName),
			huh.NewSelect[string]().
				Title("Language").
				Description("Language to use for your mcp server").
				Height(5).
				Options(languagesOptions...).
				Value(&mcpserverOptions.Language),
			huh.NewSelect[string]().
				Title("Template").
				Description("Template to use for your mcp server").
				Height(5).
				OptionsFunc(func() []huh.Option[string] {
					if len(templates) == 0 {
						return []huh.Option[string]{}
					}
					options := []huh.Option[string]{}
					for _, template := range templates.FilterByLanguage(mcpserverOptions.Language) {
						key := regexp.MustCompile(`^\d+-`).ReplaceAllString(*template.Name, "")
						options = append(options, huh.NewOption(key, *template.Name))
					}
					return options
				}, &mcpserverOptions).
				Value(&mcpserverOptions.TemplateName),
		),
	)
	form.WithTheme(core.GetHuhTheme())
	err = form.Run()
	if err != nil {
		fmt.Println("Cancel create blaxel mcp server")
		os.Exit(0)
	}
	return mcpserverOptions
}

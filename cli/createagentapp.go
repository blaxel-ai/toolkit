package cli

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
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
		Use:     "create-agent-app [directory]",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"ca", "caa"},
		Short:   "Create a new blaxel agent app",
		Long:    "Create a new blaxel agent app",
		Example: `
bl create-agent-app my-agent-app
bl create-agent-app my-agent-app --template template-google-adk-py
bl create-agent-app my-agent-app --template template-google-adk-py -y`,
		Run: func(cmd *cobra.Command, args []string) {
			dirArg := ""
			if len(args) >= 1 {
				dirArg = args[0]
			}

			core.RunCreateFlow(
				dirArg,
				templateName,
				core.CreateFlowConfig{
					TemplateType:           "agent",
					NoTTY:                  noTTY,
					ErrorPrefix:            "Agent creation",
					SpinnerTitle:           "Creating your blaxel agent app...",
					BlaxelTomlResourceType: "agent",
				},
				func(directory string, templates core.Templates) core.TemplateOptions {
					return promptCreateAgentApp(directory, templates)
				},
				func(opts core.TemplateOptions) {
					core.PrintSuccess("Your blaxel agent app has been created successfully!")
					fmt.Printf(`Start working on it:
  cd %s
  bl serve --hotreload
`, opts.Directory)
				},
			)
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
	return core.PromptTemplateOptions(directory, templates, "agent app", true, 12)
}

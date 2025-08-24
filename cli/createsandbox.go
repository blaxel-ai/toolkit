package cli

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
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
		Use:     "create-sandbox [directory]",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"cs"},
		Short:   "Create a new blaxel sandbox",
		Long:    "Create a new blaxel sandbox",
		Example: `
bl create-sandbox my-sandbox
bl create-sandbox my-sandbox --template template-sandbox-ts
bl create-sandbox my-sandbox --template template-sandbox-ts -y`,
		Run: func(cmd *cobra.Command, args []string) {
			dirArg := ""
			if len(args) >= 1 {
				dirArg = args[0]
			}

			core.RunCreateFlow(
				dirArg,
				templateName,
				core.CreateFlowConfig{
					TemplateType: "sandbox",
					NoTTY:        noTTY,
					ErrorPrefix:  "Sandbox creation",
					SpinnerTitle: "Creating your blaxel sandbox...",
				},
				func(directory string, templates core.Templates) core.TemplateOptions {
					return promptCreateSandbox(directory, templates)
				},
				func(opts core.TemplateOptions) {
					core.PrintSuccess("Your blaxel sandbox has been created successfully!")
					fmt.Printf(`Start working on it:
  cd %s
  bl deploy
`, opts.Directory)
				},
			)
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
	return core.PromptTemplateOptions(directory, templates, "sandbox", false, 5)
}

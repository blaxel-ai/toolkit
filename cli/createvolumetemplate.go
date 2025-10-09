package cli

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("create-volume-template", func() *cobra.Command {
		return CreateVolumeTemplateCmd()
	})
}

// CreateVolumeTemplateCmd returns a cobra.Command that implements the 'create-volume-template' CLI command.
// The command creates a new Blaxel volume template in the specified directory after collecting
// necessary configuration through an interactive prompt.
// Usage: bl create-volume-template directory [--template template-name]
func CreateVolumeTemplateCmd() *cobra.Command {
	var templateName string
	var noTTY bool

	cmd := &cobra.Command{
		Use:     "create-volume-template [directory]",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"cvt", "vt"},
		Short:   "Create a new blaxel volume template",
		Long:    "Create a new blaxel volume template",
		Example: `
bl create-volume-template my-volume-template
bl create-volume-template my-volume-template --template template-volume-template-py
bl create-volume-template my-volume-template --template template-volume-template-py -y`,
		Run: func(cmd *cobra.Command, args []string) {
			dirArg := ""
			if len(args) >= 1 {
				dirArg = args[0]
			}

			RunVolumeTemplateCreation(dirArg, templateName, noTTY)
		},
	}

	cmd.Flags().StringVarP(&templateName, "template", "t", "", "Template to use for the volume template (skips interactive prompt)")
	cmd.Flags().BoolVarP(&noTTY, "yes", "y", false, "Skip interactive prompts and use defaults")
	return cmd
}

// promptCreateVolumeTemplate displays an interactive form to collect user input for creating a new volume template.
// It prompts for project name, language selection, template, author, license, and additional features.
// Takes a directory string parameter and returns a TemplateOptions struct with the user's selections.
func promptCreateVolumeTemplate(directory string, templates core.Templates) core.TemplateOptions {
	return core.PromptTemplateOptions(directory, templates, "volume template", false, 5)
}

// RunVolumeTemplateCreation is a reusable wrapper that executes the volume template creation flow.
func RunVolumeTemplateCreation(dirArg string, templateName string, noTTY bool) {
	core.RunCreateFlow(
		dirArg,
		templateName,
		core.CreateFlowConfig{
			TemplateType: "volume-template",
			NoTTY:        noTTY,
			ErrorPrefix:  "Volume template creation",
			SpinnerTitle: "Creating your blaxel volume template...",
		},
		func(directory string, templates core.Templates) core.TemplateOptions {
			return promptCreateVolumeTemplate(directory, templates)
		},
		func(opts core.TemplateOptions) {
			core.PrintSuccess("Your blaxel volume template has been created successfully!")
			fmt.Printf(`Start working on it:
  cd %s
  bl deploy
`, opts.Directory)
		},
	)
}

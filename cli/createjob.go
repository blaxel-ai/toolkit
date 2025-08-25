package cli

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("create-job", func() *cobra.Command {
		return CreateJobCmd()
	})
}

// CreateJobCmd returns a cobra.Command that implements the 'create-job' CLI command.
// The command creates a new Blaxel job in the specified directory after collecting
// necessary configuration through an interactive prompt.
// Usage: bl create-job directory [--template template-name]
func CreateJobCmd() *cobra.Command {
	var templateName string
	var noTTY bool

	cmd := &cobra.Command{
		Use:     "create-job [directory]",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"cj", "cjob"},
		Short:   "Create a new blaxel job",
		Long:    "Create a new blaxel job",
		Example: `
bl create-job my-job
bl create-job my-job --template template-jobs-ts
bl create-job my-job --template template-jobs-ts -y`,
		Run: func(cmd *cobra.Command, args []string) {
			dirArg := ""
			if len(args) >= 1 {
				dirArg = args[0]
			}

			RunJobCreation(dirArg, templateName, noTTY)
		},
	}

	cmd.Flags().StringVarP(&templateName, "template", "t", "", "Template to use for the job (skips interactive prompt)")
	cmd.Flags().BoolVarP(&noTTY, "yes", "y", false, "Skip interactive prompts and use defaults")
	return cmd
}

// promptCreateJob displays an interactive form to collect user input for creating a new job.
// It prompts for project name, language selection, template, author, license, and additional features.
// Takes a directory string parameter and returns a CreateJobOptions struct with the user's selections.
func promptCreateJob(directory string, templates core.Templates) core.TemplateOptions {
	return core.PromptTemplateOptions(directory, templates, "job", true, 5)
}

// RunJobCreation is a reusable wrapper that executes the job creation flow.
func RunJobCreation(dirArg string, templateName string, noTTY bool) {
	core.RunCreateFlow(
		dirArg,
		templateName,
		core.CreateFlowConfig{
			TemplateType: "job",
			NoTTY:        noTTY,
			ErrorPrefix:  "Job creation",
			SpinnerTitle: "Creating your blaxel job...",
		},
		func(directory string, templates core.Templates) core.TemplateOptions {
			return promptCreateJob(directory, templates)
		},
		func(opts core.TemplateOptions) {
			core.PrintSuccess("Your blaxel job has been created successfully!")
			fmt.Printf(`Start working on it:
  cd %s
  bl run job %s --local --file batches/sample-batch.json
`, opts.Directory, opts.Directory)
		},
	)
}

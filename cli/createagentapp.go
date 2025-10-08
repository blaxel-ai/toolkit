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
		Aliases: []string{"ca", "caa", "ag"},
		Short:   "Create a new blaxel agent app",
		Long: `Create a new AI agent application from templates.

An agent is a conversational AI system that can interact with users, access tools,
maintain context across conversations, and integrate with external services.

Common use cases:
- Customer support chatbots
- Data analysis assistants
- Code review helpers
- Personal productivity assistants
- Domain-specific expert systems

The command scaffolds a complete agent project with configuration, dependencies,
and example code. You can choose from multiple templates supporting different
frameworks (Google ADK, LangChain, custom, etc.) and languages (Python, TypeScript).

After creation: cd into the directory, run 'bl serve --hotreload' for local
development, then 'bl deploy' when ready to deploy.

Note: Prefer using 'bl new agent' which provides a unified creation experience.`,
		Example: `  # Interactive creation
  bl create-agent-app my-agent

  # With specific template
  bl create-agent-app my-agent --template template-google-adk-py

  # Non-interactive with defaults
  bl create-agent-app my-agent --template template-google-adk-py -y

  # Recommended: Use unified 'new' command instead
  bl new agent my-agent`,
		Run: func(cmd *cobra.Command, args []string) {
			dirArg := ""
			if len(args) >= 1 {
				dirArg = args[0]
			}

			RunAgentAppCreation(dirArg, templateName, noTTY)
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

// RunAgentAppCreation is a reusable wrapper that executes the agent creation flow.
// It can be called by both the dedicated command and the unified `bl new` command.
func RunAgentAppCreation(dirArg string, templateName string, noTTY bool) {
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
}

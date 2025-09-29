package cli

import (
	"fmt"

	"github.com/blaxel-ai/toolkit/cli/core"
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
		Use:     "create-mcp-server [directory]",
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"cm", "cms"},
		Short:   "Create a new blaxel mcp server",
		Long:    "Create a new blaxel mcp server",
		Example: `
bl create-mcp-server my-mcp-server
bl create-mcp-server my-mcp-server --template template-mcp-hello-world-py
bl create-mcp-server my-mcp-server --template template-mcp-hello-world-py -y`,
		Run: func(cmd *cobra.Command, args []string) {
			dirArg := ""
			if len(args) >= 1 {
				dirArg = args[0]
			}

			RunMCPCreation(dirArg, templateName, noTTY)
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
	return core.PromptTemplateOptions(directory, templates, "mcp server", true, 5)
}

// RunMCPCreation is a reusable wrapper that executes the MCP server creation flow.
func RunMCPCreation(dirArg string, templateName string, noTTY bool) {
	core.RunCreateFlow(
		dirArg,
		templateName,
		core.CreateFlowConfig{
			TemplateType:           "mcp",
			NoTTY:                  noTTY,
			ErrorPrefix:            "MCP Server creation",
			SpinnerTitle:           "Creating your blaxel mcp server...",
			BlaxelTomlResourceType: "function",
		},
		func(directory string, templates core.Templates) core.TemplateOptions {
			return promptCreateMCPServer(directory, templates)
		},
		func(opts core.TemplateOptions) {
			core.PrintSuccess("Your blaxel MCP server has been created successfully!")
			fmt.Printf(`Start working on it:
  cd %s
  bl serve --hotreload
`, opts.Directory)
		},
	)
}

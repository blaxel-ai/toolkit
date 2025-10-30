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
		Long: `Create a new Model Context Protocol (MCP) server.

MCP servers extend agent capabilities by providing custom tools, data sources,
and integrations. They expose functions that agents can call to perform actions
or retrieve information.

Common use cases:
- Custom API integrations (GitHub, Jira, CRM systems)
- Database connectors and query tools
- File system operations
- Data transformation and analysis tools
- External service orchestration

MCP is a standard protocol for agent-tool communication. Your MCP server can
be used by any agent that supports the protocol, making it reusable across
multiple agents and projects.

The command scaffolds a complete MCP server project with:
- Server setup and configuration
- Example tool implementations
- Protocol handling code
- Testing utilities

After creation: cd into the directory, implement your tools, test with
'bl serve --hotreload', then 'bl deploy' to make available to agents.

Note: Prefer using 'bl new mcp' which provides a unified creation experience.`,
		Example: `  # Interactive creation
  bl create-mcp-server my-tools

  # With specific template
  bl create-mcp-server my-tools --template template-mcp-hello-world-py

  # Non-interactive with defaults
  bl create-mcp-server my-tools -y

  # Recommended: Use unified 'new' command instead
  bl new mcp my-tools`,
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

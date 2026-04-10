package cli

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/charmbracelet/huh"
	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type newType string

const (
	newTypeAgent          newType = "agent"
	newTypeMCP            newType = "mcp"
	newTypeSandbox        newType = "sandbox"
	newTypeJob            newType = "job"
	newTypeVolumeTemplate newType = "volumetemplate"
)

func init() {
	core.RegisterCommand("new", func() *cobra.Command { return NewCmd() })
}

// NewCmd implements `bl new` which unifies create commands under one entrypoint.
// Usage:
//
//	bl new [type] [directory] [-t template] [-y]
//
// Examples:
//
//	bl new agent my-app -t template-google-adk-py
//	bl new mcp my-func -y
//	bl new
func NewCmd() *cobra.Command {
	var templateName string
	var noTTY bool
	var listTemplates bool

	cmd := &cobra.Command{
		Use:               "new [type] [directory]",
		Args:              cobra.RangeArgs(0, 2),
		Short:             "Scaffold a new project from a template (agent, mcp, sandbox, job, volume-template)",
		ValidArgsFunction: GetNewValidArgsFunction(),
		Long: `Create a new Blaxel resource from templates.

This command scaffolds a new project with the necessary configuration files,
dependencies, and example code to get you started quickly.

Resource Types:
  agent     - AI agent application that can chat, use tools, and access data
              Use cases: Customer support bots, coding assistants, data analysts

  mcp       - Model Context Protocol server that extends agent capabilities
              Use cases: Custom tools, API integrations, database connectors

  sandbox   - Isolated execution environment for testing and running code
              Use cases: Code execution, testing, isolated workloads

  job       - Batch processing task that runs on-demand or on schedule
              Use cases: ETL pipelines, data processing, scheduled workflows

  volumetemplate - Pre-configured volume template for creating volumes
              		Use cases: Persistent storage templates, data volume configurations

Template Discovery:
Use --list to see all available templates with descriptions before creating.
Combine with a type argument to filter: 'bl new agent --list'

Interactive Mode (Recommended):
When called without arguments, the CLI guides you through:
1. Choosing a resource type
2. Selecting a template (language/framework)
3. Naming your project directory
4. Setting up initial configuration

Non-Interactive Mode:
Use --template and --yes flags for automation and CI/CD workflows.

After Creation:
1. cd into your new directory
2. Review and customize the generated blaxel.toml configuration
3. Develop your resource locally with 'bl serve --hotreload'
4. Test it works as expected
5. Deploy to Blaxel with 'bl deploy'`,
		Run: func(cmd *cobra.Command, args []string) {
			// Handle --list flag
			if listTemplates {
				filterType := ""
				if len(args) >= 1 {
					t := parseNewType(args[0])
					if t != "" {
						filterType = string(t)
					}
				}
				listAvailableTemplates(filterType, core.GetOutputFormat())
				return
			}

			var t newType
			dirArg := ""

			if len(args) >= 1 {
				t = parseNewType(args[0])
				if len(args) >= 2 {
					dirArg = args[1]
				}
			}

			if t == "" {
				if noTTY {
					core.PrintError("New", fmt.Errorf("type is required when using --yes. Allowed: agent | mcp | sandbox | job | volumetemplate"))
					return
				}
				// Prompt for type using huh
				var selected string
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewSelect[string]().
							Title("What do you want to create?").
							Options(
								huh.NewOption("Agent app", string(newTypeAgent)),
								huh.NewOption("MCP server", string(newTypeMCP)),
								huh.NewOption("Sandbox", string(newTypeSandbox)),
								huh.NewOption("Job", string(newTypeJob)),
								huh.NewOption("Volume template", string(newTypeVolumeTemplate)),
							).
							Value(&selected),
					),
				)
				form.WithTheme(core.GetHuhTheme())
				if err := form.Run(); err != nil {
					return
				}
				t = parseNewType(selected)
			}

			// Dispatch to existing flows with appropriate config and prompt
			switch t {
			case newTypeAgent:
				core.RunAgentAppCreation(dirArg, templateName, noTTY)
			case newTypeMCP:
				core.RunMCPCreation(dirArg, templateName, noTTY)
			case newTypeSandbox:
				core.RunSandboxCreation(dirArg, templateName, noTTY)
			case newTypeJob:
				core.RunJobCreation(dirArg, templateName, noTTY)
			case newTypeVolumeTemplate:
				core.RunVolumeTemplateCreation(dirArg, templateName, noTTY)
			default:
				core.PrintError("New", fmt.Errorf("unknown type '%s'. Allowed: agent | mcp | sandbox | job | volumetemplate", t))
			}
		},
	}

	cmd.Flags().StringVarP(&templateName, "template", "t", "", "Template to use (skips interactive prompt)")
	cmd.Flags().BoolVarP(&noTTY, "yes", "y", false, "Skip interactive prompts and use defaults")
	cmd.Flags().BoolVarP(&listTemplates, "list", "l", false, "List available templates with descriptions")

	cmd.Example = `  # Interactive creation (recommended for beginners)
  bl new

  # Create agent interactively
  bl new agent

  # Create agent with specific template
  bl new agent my-agent -t google-adk-py

  # Create MCP server with default template (non-interactive)
  bl new mcp my-mcp-server -y -t mcp-py

  # Create job with specific template
  bl new job my-batch-job -t jobs-py

  # List all available templates
  bl new --list

  # List agent templates only
  bl new agent --list

  # List templates as JSON (for machine parsing)
  bl new --list -o json

  # Full workflow example:
  bl new agent my-assistant
  cd my-assistant
  bl serve --hotreload    # Test locally
  bl deploy               # Deploy to Blaxel
  bl chat my-assistant    # Chat with deployed agent`
	return cmd
}

func parseNewType(s string) newType {
	switch strings.ToLower(s) {
	case string(newTypeAgent), "ag":
		return newTypeAgent
	case string(newTypeMCP):
		return newTypeMCP
	case string(newTypeSandbox), "sbx":
		return newTypeSandbox
	case string(newTypeJob), "jb":
		return newTypeJob
	case string(newTypeVolumeTemplate), "vt", "volume-template":
		return newTypeVolumeTemplate
	default:
		return ""
	}
}

type templateInfo struct {
	Name        string `json:"name"`
	FullName    string `json:"fullName"`
	Type        string `json:"type"`
	Language    string `json:"language"`
	Description string `json:"description"`
	URL         string `json:"url"`
}

// listAvailableTemplates retrieves and displays templates in the requested format.
// filterType narrows results to a specific template type (e.g. "agent", "mcp").
func listAvailableTemplates(filterType string, outputFormat string) {
	types := []string{"agent", "mcp", "sandbox", "job", "volume-template"}
	if filterType != "" {
		types = []string{filterType}
	}

	stripRe := regexp.MustCompile(`^\d+-`)

	var results []templateInfo
	for _, t := range types {
		templates, err := core.RetrieveTemplates(t)
		if err != nil {
			continue
		}
		for _, tmpl := range templates {
			displayName := stripRe.ReplaceAllString(tmpl.Name, "")
			displayName = strings.TrimPrefix(displayName, "template-")
			results = append(results, templateInfo{
				Name:        displayName,
				FullName:    tmpl.Name,
				Type:        t,
				Language:    tmpl.Language,
				Description: tmpl.Description,
				URL:         tmpl.URL,
			})
		}
	}

	if len(results) == 0 {
		core.PrintInfo("No templates found")
		return
	}

	switch outputFormat {
	case "json":
		data, _ := json.MarshalIndent(results, "", "  ")
		fmt.Println(string(data))
	case "yaml":
		data, _ := yaml.Marshal(results)
		fmt.Print(string(data))
	default:
		printTemplateTable(results)
	}
}

func printTemplateTable(templates []templateInfo) {
	t := table.NewWriter()
	t.SetStyle(table.StyleLight)
	t.AppendHeader(table.Row{"NAME", "TYPE", "LANGUAGE", "DESCRIPTION"})
	for _, tmpl := range templates {
		desc := tmpl.Description
		if len(desc) > 60 {
			desc = desc[:57] + "..."
		}
		t.AppendRow(table.Row{tmpl.Name, tmpl.Type, tmpl.Language, desc})
	}
	fmt.Println(t.Render())
}

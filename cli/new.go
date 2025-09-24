package cli

import (
	"fmt"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

type newType string

const (
	newTypeAgent   newType = "agent"
	newTypeMCP     newType = "mcp"
	newTypeSandbox newType = "sandbox"
	newTypeJob     newType = "job"
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

	cmd := &cobra.Command{
		Use:   "new [type] [directory]",
		Args:  cobra.RangeArgs(0, 2),
		Short: "Create a new blaxel resource (agent, mcp, sandbox, job)",
		Long:  "Create a new blaxel resource (agent, mcp, sandbox, job) with a unified command",
		Run: func(cmd *cobra.Command, args []string) {
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
					core.PrintError("New", fmt.Errorf("type is required when using --yes. Allowed: agent | mcp | sandbox | job"))
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
				RunAgentAppCreation(dirArg, templateName, noTTY)
			case newTypeMCP:
				RunMCPCreation(dirArg, templateName, noTTY)
			case newTypeSandbox:
				RunSandboxCreation(dirArg, templateName, noTTY)
			case newTypeJob:
				RunJobCreation(dirArg, templateName, noTTY)
			default:
				core.PrintError("New", fmt.Errorf("unknown type '%s'. Allowed: agent | mcp | sandbox | job", t))
			}
		},
	}

	cmd.Flags().StringVarP(&templateName, "template", "t", "", "Template to use (skips interactive prompt)")
	cmd.Flags().BoolVarP(&noTTY, "yes", "y", false, "Skip interactive prompts and use defaults")
	return cmd
}

func parseNewType(s string) newType {
	switch strings.ToLower(s) {
	case string(newTypeAgent):
		return newTypeAgent
	case string(newTypeMCP):
		return newTypeMCP
	case string(newTypeSandbox):
		return newTypeSandbox
	case string(newTypeJob):
		return newTypeJob
	default:
		return ""
	}
}

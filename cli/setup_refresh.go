package cli

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"
)

// setupRefreshEnv is the installer/upgrade handoff to the existing setup command.
// Refresh never runs the setup screens, authentication, or tracking tasks.
const setupRefreshEnv = "BL_INSTALL_REFRESH"

const setupRefreshCapability = "blaxel-setup-refresh-v1"

func automaticSetupOffers(env func(string) string) (skills, mcp bool) {
	if envDisabled(env, "BL_INSTALL_SETUP") {
		return false, false
	}
	return !skillsInstallDisabled(env), !automaticInstallDisabled(env, mcpInstallEnv)
}

func refreshSetup(options setupOptions) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, "Blaxel setup refresh could not find the home directory; retry with bl setup.")
		return
	}
	bl, err := blCommandPath(os.Executable)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Blaxel setup refresh could not locate bl; retry with bl setup.")
		return
	}
	options.home, options.env, options.out = home, os.Getenv, os.Stderr
	options.mcp = newMCPEnv(home)
	options.resourceServer, options.documentsServer = resourceMCPServer(bl), docsMCPServer()
	options.installSkills = installSkillsForSafely
	runSetupRefresh(context.Background(), options)
}

// runSetupRefresh shares setup's detection and conservative MCP writes. Each
// component is best effort, so an unavailable skills archive does not stop MCP.
func runSetupRefresh(ctx context.Context, options setupOptions) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	agents := detectedSetupAgents(options.home, options.env)
	skills, mcp := automaticSetupOffers(options.env)
	if state, err := readSkillsUpdateState(options.home); err != nil || skillsUpdateDisabled(state, options.env) != "" {
		skills = false
	}
	skills = skills && !options.skipSkills
	mcp = mcp && !options.skipMCP
	parts := []string{"skills skipped", "MCP skipped"}
	problems := 0
	if skills {
		var selected []skillsAgent
		for _, agent := range agents {
			if !isMCPOnlyAgent(agent.id) {
				selected = append(selected, agent)
			}
		}
		result, err := options.installSkills(ctx, selected)
		if err != nil {
			parts[0] = "skills unavailable"
			problems++
		} else {
			parts[0] = fmt.Sprintf("%d skills refreshed", len(result.skills))
			if len(result.preserved) > 0 {
				parts[0] += fmt.Sprintf(", %d externally managed kept", len(result.preserved))
			}
			if len(result.backups) > 0 {
				parts[0] += fmt.Sprintf(", %d copies backed up outside skills folders", len(result.backups))
			}
		}
	}
	if mcp {
		added, updated, kept, plugin := 0, 0, 0, 0
		for _, agent := range agents {
			if target, ok := mcpTargets[agent.id]; ok {
				result := configureAgentMCP(ctx, options.mcp, target, []mcpServer{options.resourceServer, options.documentsServer})
				added += len(result.added)
				updated += len(result.updated)
				kept += len(result.existing)
				plugin += len(result.plugin)
				if result.err != nil {
					problems++
				}
			}
		}
		parts[1] = fmt.Sprintf("MCP %d added, %d migrated, %d kept, %d from plugins", added, updated, kept, plugin)
	}
	if problems > 0 {
		parts = append(parts, fmt.Sprintf("%d %s; retry with bl setup", problems, plural(problems, "problem", "problems")))
	}
	_, _ = fmt.Fprintln(options.out, "Blaxel setup refresh: "+strings.Join(parts, "; ")+".")
}

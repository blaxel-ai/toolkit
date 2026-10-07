package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// skillsAgent describes a coding agent the skills installer can target.
// Universal agents read the shared ~/.agents/skills directory, so they are
// covered by the "universal" target and only listed for the summary.
type skillsAgent struct {
	id        string
	name      string
	universal bool
	homes     func(skillsAgentPaths) []string
}

type skillsAgentPaths struct {
	home, config string
	env          func(string) string
}

func (p skillsAgentPaths) homeDir(elem ...string) string {
	return filepath.Join(append([]string{p.home}, elem...)...)
}

func (p skillsAgentPaths) configDir(elem ...string) string {
	return filepath.Join(append([]string{p.config}, elem...)...)
}

func (p skillsAgentPaths) envOr(key string, fallback string) string {
	if value := strings.TrimSpace(p.env(key)); value != "" {
		return value
	}
	return fallback
}

// Detection and folders mirror the skills package (npx skills), so both
// installers put the skills in the same places. Only detected or chosen agents
// are set up, never every agent the skills package knows.
var skillsAgents = []skillsAgent{
	{"claude-code", "Claude Code", false, func(p skillsAgentPaths) []string {
		return []string{p.envOr("CLAUDE_CONFIG_DIR", p.homeDir(".claude"))}
	}},
	{"codex", "Codex", true, func(p skillsAgentPaths) []string {
		return []string{p.envOr("CODEX_HOME", p.homeDir(".codex"))}
	}},
	{"cursor", "Cursor", true, func(p skillsAgentPaths) []string { return []string{p.homeDir(".cursor")} }},
	{"gemini-cli", "Gemini CLI", true, func(p skillsAgentPaths) []string { return []string{p.homeDir(".gemini")} }},
	{"github-copilot", "GitHub Copilot", true, func(p skillsAgentPaths) []string { return []string{p.homeDir(".copilot")} }},
	{"opencode", "OpenCode", true, func(p skillsAgentPaths) []string { return []string{p.configDir("opencode")} }},
	{"amp", "Amp", true, func(p skillsAgentPaths) []string { return []string{p.configDir("amp")} }},
	{"cline", "Cline", true, func(p skillsAgentPaths) []string { return []string{p.homeDir(".cline")} }},
	{"windsurf", "Windsurf", false, func(p skillsAgentPaths) []string { return []string{p.homeDir(".codeium", "windsurf")} }},
	{"goose", "Goose", false, func(p skillsAgentPaths) []string { return []string{p.configDir("goose")} }},
	{"kiro-cli", "Kiro CLI", false, func(p skillsAgentPaths) []string { return []string{p.homeDir(".kiro")} }},
	{"roo", "Roo Code", false, func(p skillsAgentPaths) []string { return []string{p.homeDir(".roo")} }},
	{"continue", "Continue", false, func(p skillsAgentPaths) []string { return []string{p.homeDir(".continue")} }},
	{"augment", "Augment", false, func(p skillsAgentPaths) []string { return []string{p.homeDir(".augment")} }},
	{"junie", "Junie", false, func(p skillsAgentPaths) []string { return []string{p.homeDir(".junie")} }},
	{"trae", "Trae", false, func(p skillsAgentPaths) []string { return []string{p.homeDir(".trae")} }},
	{"qwen-code", "Qwen Code", false, func(p skillsAgentPaths) []string { return []string{p.homeDir(".qwen")} }},
	{"openhands", "OpenHands", false, func(p skillsAgentPaths) []string { return []string{p.homeDir(".openhands")} }},
	{"pi", "Pi", false, func(p skillsAgentPaths) []string { return []string{p.homeDir(".pi", "agent")} }},
	{"crush", "Crush", false, func(p skillsAgentPaths) []string { return []string{p.homeDir(".config", "crush")} }},
	{"devin", "Devin", false, func(p skillsAgentPaths) []string { return []string{p.configDir("devin")} }},
	{"openclaw", "OpenClaw", false, func(p skillsAgentPaths) []string {
		return []string{p.homeDir(".openclaw"), p.homeDir(".clawdbot"), p.homeDir(".moltbot")}
	}},
}

// mcpOnlyAgents take MCP servers but have no skills folder, so setup adds
// only the MCP servers to them.
var mcpOnlyAgents = []skillsAgent{
	{"claude-desktop", "Claude Desktop", false, func(p skillsAgentPaths) []string { return []string{claudeDesktopDir(p)} }},
}

// claudeDesktopDir is where the Claude Desktop app keeps its configuration.
func claudeDesktopDir(p skillsAgentPaths) string {
	switch runtime.GOOS {
	case "darwin":
		return p.homeDir("Library", "Application Support", "Claude")
	case "windows":
		return filepath.Join(p.envOr("APPDATA", p.homeDir("AppData", "Roaming")), "Claude")
	}
	return p.configDir("Claude")
}

func isMCPOnlyAgent(id string) bool {
	return slices.ContainsFunc(mcpOnlyAgents, func(a skillsAgent) bool { return a.id == id })
}

// setupAgents is every agent setup knows: those that take skills, then those
// that take MCP servers only.
func setupAgents() []skillsAgent {
	return slices.Concat(skillsAgents, mcpOnlyAgents)
}

// detectedSetupAgents returns the agents setup finds on this machine.
func detectedSetupAgents(home string, env func(string) string) []skillsAgent {
	return detectAgents(setupAgents(), home, env)
}

// detectSkillsAgents returns the installer's --agent values and the detected
// agent names. "universal" (~/.agents/skills) is always targeted.
func detectSkillsAgents(home string, env func(string) string) (targets []string, names []string) {
	return skillsTargets(detectedSkillsAgents(home, env))
}

func newSkillsAgentPaths(home string, env func(string) string) skillsAgentPaths {
	paths := skillsAgentPaths{home: home, config: filepath.Join(home, ".config"), env: env}
	if xdg := strings.TrimSpace(env("XDG_CONFIG_HOME")); xdg != "" {
		paths.config = xdg
	}
	return paths
}

// skillsAgentDir is the global skills folder of an agent that does not read
// ~/.agents/skills: the skills folder of its first existing home, as in the
// skills package (for example ~/.claude/skills or ~/.pi/agent/skills).
func skillsAgentDir(agent skillsAgent, paths skillsAgentPaths) string {
	homes := agent.homes(paths)
	for _, dir := range homes {
		if _, err := os.Stat(dir); err == nil {
			return filepath.Join(dir, "skills")
		}
	}
	return filepath.Join(homes[0], "skills")
}

// detectedSkillsAgents returns the coding agents configured on this machine
// that take skills.
func detectedSkillsAgents(home string, env func(string) string) []skillsAgent {
	return detectAgents(skillsAgents, home, env)
}

func detectAgents(agents []skillsAgent, home string, env func(string) string) []skillsAgent {
	paths := newSkillsAgentPaths(home, env)
	var detected []skillsAgent
	for _, agent := range agents {
		for _, dir := range agent.homes(paths) {
			if _, err := os.Stat(dir); err == nil {
				detected = append(detected, agent)
				break
			}
		}
	}
	return detected
}

// skillsTargets returns the installer's --agent values for the given agents.
func skillsTargets(agents []skillsAgent) (targets []string, names []string) {
	targets = []string{"universal"}
	for _, agent := range agents {
		if !agent.universal {
			targets = append(targets, agent.id)
		}
		names = append(names, agent.name)
	}
	return targets, names
}

func findSkillsAgent(id string) (skillsAgent, bool) {
	for _, agent := range setupAgents() {
		if agent.id == id {
			return agent, true
		}
	}
	return skillsAgent{}, false
}

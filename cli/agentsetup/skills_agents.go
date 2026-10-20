package agentsetup

import (
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// SkillsAgent describes a coding agent the skills installer can target.
// Universal agents read the shared ~/.agents/skills directory, so they are
// covered by the "universal" target and only listed for the summary.
type SkillsAgent struct {
	ID        string
	Name      string
	universal bool
	homes     func(SkillsAgentPaths) []string
}

type SkillsAgentPaths struct {
	home, config string
	env          func(string) string
}

func (p SkillsAgentPaths) homeDir(elem ...string) string {
	return filepath.Join(append([]string{p.home}, elem...)...)
}

func (p SkillsAgentPaths) configDir(elem ...string) string {
	return filepath.Join(append([]string{p.config}, elem...)...)
}

func (p SkillsAgentPaths) envOr(key string, fallback string) string {
	if value := strings.TrimSpace(p.env(key)); value != "" {
		return value
	}
	return fallback
}

// Detection and folders mirror the skills package (npx skills), so both
// installers put the skills in the same places. Only detected or chosen agents
// are set up, never every agent the skills package knows.
var SkillsAgents = []SkillsAgent{
	{"claude-code", "Claude Code", false, func(p SkillsAgentPaths) []string {
		return []string{p.envOr("CLAUDE_CONFIG_DIR", p.homeDir(".claude"))}
	}},
	{"codex", "Codex", true, func(p SkillsAgentPaths) []string {
		return []string{p.envOr("CODEX_HOME", p.homeDir(".codex"))}
	}},
	{"cursor", "Cursor", true, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".cursor")} }},
	{"gemini-cli", "Gemini CLI", true, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".gemini")} }},
	{"github-copilot", "GitHub Copilot", true, func(p SkillsAgentPaths) []string {
		return []string{p.envOr("COPILOT_HOME", p.homeDir(".copilot"))}
	}},
	{"opencode", "OpenCode", true, func(p SkillsAgentPaths) []string { return []string{p.configDir("opencode")} }},
	{"amp", "Amp", true, func(p SkillsAgentPaths) []string { return []string{p.configDir("amp")} }},
	{"cline", "Cline CLI", true, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".cline")} }},
	{"windsurf", "Windsurf", false, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".codeium", "windsurf")} }},
	{"goose", "Goose", false, func(p SkillsAgentPaths) []string { return []string{p.configDir("goose")} }},
	{"kiro-cli", "Kiro CLI", false, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".kiro")} }},
	{"roo", "Roo Code", false, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".roo")} }},
	{"continue", "Continue", false, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".continue")} }},
	{"augment", "Augment", false, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".augment")} }},
	{"junie", "Junie", false, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".junie")} }},
	{"trae", "Trae", false, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".trae")} }},
	{"qwen-code", "Qwen Code", false, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".qwen")} }},
	{"openhands", "OpenHands", false, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".openhands")} }},
	{"pi", "Pi", false, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".pi", "agent")} }},
	{"crush", "Crush", false, func(p SkillsAgentPaths) []string { return []string{p.homeDir(".config", "crush")} }},
	{"devin", "Devin", false, func(p SkillsAgentPaths) []string { return []string{p.configDir("devin")} }},
	{"openclaw", "OpenClaw", false, func(p SkillsAgentPaths) []string {
		return []string{p.homeDir(".openclaw"), p.homeDir(".clawdbot"), p.homeDir(".moltbot")}
	}},
	{"vscode", "GitHub Copilot (VS Code)", true, func(p SkillsAgentPaths) []string { return []string{vscodeUserDir(p)} }},
}

// mcpOnlyAgents take MCP servers but have no skills folder, so setup adds
// only the MCP servers to them.
var mcpOnlyAgents = []SkillsAgent{
	{"claude-desktop", "Claude Desktop", false, func(p SkillsAgentPaths) []string { return []string{ClaudeDesktopDir(p)} }},
}

// vscodeUserDir is the default VS Code profile's user configuration folder.
func vscodeUserDir(p SkillsAgentPaths) string {
	switch runtime.GOOS {
	case "darwin":
		return p.homeDir("Library", "Application Support", "Code", "User")
	case "windows":
		return filepath.Join(p.envOr("APPDATA", p.homeDir("AppData", "Roaming")), "Code", "User")
	}
	return p.configDir("Code", "User")
}

func gooseConfigDir(p SkillsAgentPaths) string {
	if root := strings.TrimSpace(p.env("GOOSE_PATH_ROOT")); filepath.IsAbs(root) {
		return filepath.Join(root, "config")
	}
	if runtime.GOOS == "windows" {
		return filepath.Join(p.envOr("APPDATA", p.homeDir("AppData", "Roaming")), "Block", "goose", "config")
	}
	return p.configDir("goose")
}

// ClaudeDesktopDir is where the Claude Desktop app keeps its configuration.
func ClaudeDesktopDir(p SkillsAgentPaths) string {
	switch runtime.GOOS {
	case "darwin":
		return p.homeDir("Library", "Application Support", "Claude")
	case "windows":
		return filepath.Join(p.envOr("APPDATA", p.homeDir("AppData", "Roaming")), "Claude")
	}
	return p.configDir("Claude")
}

func IsMCPOnlyAgent(id string) bool {
	return slices.ContainsFunc(mcpOnlyAgents, func(a SkillsAgent) bool { return a.ID == id })
}

// SetupAgents is every agent setup knows: those that take skills, then those
// that take MCP servers only.
func SetupAgents() []SkillsAgent {
	return slices.Concat(SkillsAgents, mcpOnlyAgents)
}

// DetectedSetupAgents returns the agents setup finds on this machine.
func DetectedSetupAgents(home string, env func(string) string) []SkillsAgent {
	return detectAgents(SetupAgents(), home, env)
}

// detectSkillsAgents returns the installer's --agent values and the detected
// agent names. "universal" (~/.agents/skills) is always targeted.
func detectSkillsAgents(home string, env func(string) string) (targets []string, names []string) {
	return SkillsTargets(detectedSkillsAgents(home, env))
}

func NewSkillsAgentPaths(home string, env func(string) string) SkillsAgentPaths {
	paths := SkillsAgentPaths{home: home, config: filepath.Join(home, ".config"), env: env}
	if xdg := strings.TrimSpace(env("XDG_CONFIG_HOME")); xdg != "" {
		paths.config = xdg
	}
	return paths
}

// skillsAgentDir is the global skills folder of an agent that does not read
// ~/.agents/skills: the skills folder of its first existing home, as in the
// skills package (for example ~/.claude/skills or ~/.pi/agent/skills).
func skillsAgentDir(agent SkillsAgent, paths SkillsAgentPaths) string {
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
func detectedSkillsAgents(home string, env func(string) string) []SkillsAgent {
	return detectAgents(SkillsAgents, home, env)
}

func detectAgents(agents []SkillsAgent, home string, env func(string) string) []SkillsAgent {
	paths := NewSkillsAgentPaths(home, env)
	var detected []SkillsAgent
	for _, agent := range agents {
		dirs := agent.homes(paths)
		if agent.ID == "goose" {
			dirs = append(dirs, gooseConfigDir(paths))
		}
		if agent.ID == "cline" {
			dirs = append(dirs, clineMCPSettingsFile(MCPEnv{Home: home, Env: env}))
		}
		if agent.ID == "openclaw" {
			dirs = append(dirs, openclawConfigFile(MCPEnv{Home: home, Env: env}))
		}
		if agent.ID == "continue" {
			dirs = append(dirs, paths.envOr("CONTINUE_GLOBAL_DIR", paths.homeDir(".continue")))
		}
		if agent.ID == "crush" {
			dirs = append(dirs, crushConfigDir(paths))
		}
		for _, dir := range dirs {
			if _, err := os.Stat(dir); err == nil {
				detected = append(detected, agent)
				break
			}
		}
	}
	return detected
}

// SkillsTargets returns the installer's --agent values for the given agents.
func SkillsTargets(agents []SkillsAgent) (targets []string, names []string) {
	targets = []string{"universal"}
	for _, agent := range agents {
		if !agent.universal {
			targets = append(targets, agent.ID)
		}
		names = append(names, agent.Name)
	}
	return targets, names
}

func FindSkillsAgent(id string) (SkillsAgent, bool) {
	for _, agent := range SetupAgents() {
		if agent.ID == id {
			return agent, true
		}
	}
	return SkillsAgent{}, false
}

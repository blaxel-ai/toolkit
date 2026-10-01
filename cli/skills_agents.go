package cli

import (
	"os"
	"path/filepath"
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

// Detection mirrors the pinned skills package (cli/skillsinstaller). Passing an
// explicit agent list prevents its fallback of writing into every one of its
// ~80 supported agent directories when it detects none.
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

// detectSkillsAgents returns the installer's --agent values and the detected
// agent names. "universal" (~/.agents/skills) is always targeted.
func detectSkillsAgents(home string, env func(string) string) (targets []string, names []string) {
	paths := skillsAgentPaths{home: home, config: filepath.Join(home, ".config"), env: env}
	if xdg := strings.TrimSpace(env("XDG_CONFIG_HOME")); xdg != "" {
		paths.config = xdg
	}
	targets = []string{"universal"}
	for _, agent := range skillsAgents {
		for _, dir := range agent.homes(paths) {
			if _, err := os.Stat(dir); err == nil {
				if !agent.universal {
					targets = append(targets, agent.id)
				}
				names = append(names, agent.name)
				break
			}
		}
	}
	return targets, names
}

package core

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// codingAgents lists, first match wins, the environment variables coding
// agents set for the commands they run, with the agent id the CLI reports.
// Ids match the ones bl setup reports in detected_agents. install.sh (is_agent)
// and install.ps1 (Test-AgentRun) check a similar list to choose the agent
// install flow.
var codingAgents = []struct{ env, id string }{
	{"CLAUDECODE", "claude-code"},
	{"CODEX_THREAD_ID", "codex"},
	{"CODEX_SANDBOX", "codex"},
	{"CURSOR_AGENT", "cursor"},
	{"GEMINI_CLI", "gemini-cli"},
	{"QWEN_CODE", "qwen-code"},
	{"OPENCODE_TERMINAL", "opencode"},
	{"OPENCODE", "opencode"},
	{"GOOSE_TERMINAL", "goose"},
	{"AUGMENT_AGENT", "augment"},
}

// codingAgentNames maps the agent names found in the generic AI_AGENT and
// AGENT variables to the same ids. Any other AI_AGENT name is reported as
// "other".
var codingAgentNames = map[string]string{
	"claude-code":        "claude-code",
	"claude":             "claude-code",
	"codex":              "codex",
	"cursor":             "cursor",
	"cursor-cli":         "cursor",
	"gemini-cli":         "gemini-cli",
	"gemini":             "gemini-cli",
	"qwen-code":          "qwen-code",
	"opencode":           "opencode",
	"goose":              "goose",
	"augment":            "augment",
	"augment-cli":        "augment",
	"crush":              "crush",
	"github-copilot":     "github-copilot",
	"github-copilot-cli": "github-copilot",
}

// agentName is the lowercase agent name in an AI_AGENT or AGENT value.
func agentName(value string) string {
	name := strings.ToLower(strings.TrimSpace(value))
	// Claude Code sets AI_AGENT=claude-code_<version>_agent: keep the name.
	if end := strings.IndexAny(name, "_/@ "); end > 0 {
		name = name[:end]
	}
	return name
}

// detectCodingAgent returns the id of the coding agent whose environment the
// CLI runs in, "other" for an agent without a known name, or "" for none.
func detectCodingAgent(getenv func(string) string) string {
	for _, agent := range codingAgents {
		if strings.TrimSpace(getenv(agent.env)) != "" {
			return agent.id
		}
	}
	if name := agentName(getenv("AI_AGENT")); name != "" {
		if id, ok := codingAgentNames[name]; ok {
			return id
		}
		return "other"
	}
	// AGENT is a generic name, so only a known agent name counts.
	return codingAgentNames[agentName(getenv("AGENT"))]
}

// codingAgentOptedOut reports an explicit opt-out of the agent token:
// DO_NOT_TRACK set to 1 or true (as the SDKs read it), BL_INSTALL_TRACKING=false
// or BL_SKIP_TELEMETRY=1. The saved tracking setting is not read: it is an
// opt-in for usage and error reports, and bl login saves it as false until the
// user turns it on.
func codingAgentOptedOut() bool {
	if strings.EqualFold(strings.TrimSpace(os.Getenv("BL_INSTALL_TRACKING")), "false") || os.Getenv("BL_SKIP_TELEMETRY") == "1" {
		return true
	}
	doNotTrack := strings.ToLower(strings.TrimSpace(os.Getenv("DO_NOT_TRACK")))
	return doNotTrack == "1" || doNotTrack == "true"
}

// cliUserAgent is the User-Agent of API requests. When a coding agent runs the
// CLI it ends with an "agent/<id>" token, unless the user opted out.
func cliUserAgent(version, commit string, getenv func(string) string) string {
	commitHash := "unknown"
	if commit != "" {
		commitHash = commit
		if len(commitHash) > 7 {
			commitHash = commitHash[:7]
		}
	}
	userAgent := fmt.Sprintf("blaxel/cli/golang/%s (%s/%s) blaxel/%s", version, runtime.GOOS, runtime.GOARCH, commitHash)
	if agent := detectCodingAgent(getenv); agent != "" && !codingAgentOptedOut() {
		userAgent += " agent/" + agent
	}
	return userAgent
}

// UserAgent is the User-Agent of this CLI's API requests, for requests the
// SDK client does not send, such as bl mcp's.
func UserAgent() string {
	return cliUserAgent(version, commit, os.Getenv)
}

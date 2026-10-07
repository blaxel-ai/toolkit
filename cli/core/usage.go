package core

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"

	blaxel "github.com/blaxel-ai/sdk-go"
)

// UsageDisclosureURL must be published with the matching CLI usage disclosure
// before a release containing these hooks is distributed.
const UsageDisclosureURL = "https://docs.blaxel.ai/Security/Data-collection-and-privacy"

func usageTrackingEnabled() bool {
	if os.Getenv("DO_NOT_TRACK") != "" || strings.EqualFold(strings.TrimSpace(os.Getenv("BL_INSTALL_TRACKING")), "false") || os.Getenv("BL_SKIP_TELEMETRY") == "1" {
		return false
	}
	for _, name := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "CIRCLECI", "TRAVIS", "JENKINS_URL", "BUILDKITE", "TEAMCITY_VERSION"} {
		if os.Getenv(name) != "" {
			return false
		}
	}
	return blaxel.IsTrackingEnabled()
}

// UsageEventsEnabled reports whether this build and the user's consent allow
// usage events at all, so callers can skip work that only feeds them.
func UsageEventsEnabled() bool {
	return PosthogAPIKey != "" && usageTrackingEnabled()
}

var usageVersion = regexp.MustCompile(`^v?\d{1,6}\.\d{1,6}\.\d{1,6}(?:-(?:alpha|beta|rc)\.?\d{0,6})?$`)
var usageID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

// usageProperties constructs the entire property payload from finite categories.
// It never forwards a caller's arbitrary keys, strings, or error messages.
func usageProperties(event string, input map[string]any) map[string]any {
	properties := map[string]any{
		"$process_person_profile": false,
		"$geoip_disable":          true,
		"os":                      allowedValue(runtime.GOOS, "darwin", "linux", "windows"),
		"architecture":            allowedValue(runtime.GOARCH, "amd64", "arm64", "386", "arm"),
		"install_method":          usageInstallMethod(),
	}
	if usageVersion.MatchString(GetVersion()) {
		properties["cli_version"] = GetVersion()
	}
	for _, key := range []string{"version", "old_version", "new_version"} {
		if value, ok := input[key].(string); ok && usageVersion.MatchString(value) && ((event == "Installed CLI" && key == "version") || (event == "Upgraded CLI" && key != "version")) {
			properties[key] = value
		}
	}
	if event == "Setup CLI" || event == "Login CLI" {
		status, _ := input["status"].(string)
		properties["status"] = allowedValue(status, "started", "success", "failure", "cancelled")
		if category, ok := input["failure_category"].(string); ok {
			properties["failure_category"] = allowedValue(category, "usage", "validation", "authentication", "not_found", "conflict", "operational", "internal", "panic")
		}
	}
	if event == "Setup CLI" {
		properties["detected_agents"] = allowedList(input["detected_agents"], "claude-code", "codex", "cursor", "gemini-cli", "github-copilot", "opencode", "amp", "cline", "windsurf", "goose", "kiro-cli", "roo", "continue", "augment", "junie", "trae", "qwen-code", "openhands", "pi", "crush", "devin", "openclaw")
		properties["components"] = allowedList(input["components"], "skills", "mcp", "docs", "login", "tracking")
	}
	if event == "First Resource CLI" {
		kind, _ := input["resource_category"].(string)
		properties["resource_category"] = allowedValue(kind, "Agent", "Function", "Sandbox", "Job", "Model", "Volume", "Drive")
	}
	return properties
}

func allowedValue(value string, allowed ...string) string {
	if slices.Contains(allowed, value) {
		return value
	}
	return "unknown"
}

func allowedList(input any, allowed ...string) []string {
	values, _ := input.([]string)
	result := []string{}
	for _, value := range values {
		if slices.Contains(allowed, value) && !slices.Contains(result, value) {
			result = append(result, value)
		}
	}
	slices.Sort(result)
	return result
}

func usageInstallMethod() string {
	if os.Getenv("BL_INSTALLER") == "1" {
		if runtime.GOOS == "windows" {
			return "powershell"
		}
		return "shell"
	}
	executable, err := os.Executable()
	if err == nil {
		if resolved, err := filepath.EvalSymlinks(executable); err == nil {
			bin := filepath.Dir(resolved)
			rack := filepath.Dir(filepath.Dir(bin))
			if filepath.Base(bin) == "bin" && filepath.Base(rack) == "blaxel" && filepath.Base(filepath.Dir(rack)) == "Cellar" {
				return "homebrew"
			}
		}
	}
	return "unknown"
}

// TrackCLISetup reports an attempt's outcome after setup has applied consent.
func TrackCLISetup(agents, components []string, status string, err error) {
	properties := map[string]any{"status": status, "detected_agents": agents, "components": components}
	if err != nil {
		properties["failure_category"] = string(classifyCLIError(err).category)
	}
	capturePosthogEvent("Setup CLI", properties, func(bool) {})
}

// TrackCLILogin records only a phase and a sanitized error category.
func TrackCLILogin(status string, err error) {
	properties := map[string]any{"status": status}
	if err != nil {
		properties["failure_category"] = string(classifyCLIError(err).category)
	}
	capturePosthogEvent("Login CLI", properties, func(bool) {})
}

// TrackCLIFirstResource records the first eligible successful CLI create for
// this local random ID. It cannot establish an account's first resource.
func TrackCLIFirstResource(kind string) {
	if !usageTrackingEnabled() || PosthogAPIKey == "" || allowedValue(kind, "Agent", "Function", "Sandbox", "Job", "Model", "Volume", "Drive") == "unknown" {
		return
	}
	const key = "first-resource"
	telemetryMu.Lock()
	state := loadTelemetryState()
	_, pending := pendingCLIEvents[key]
	if state.FirstResource || pending {
		telemetryMu.Unlock()
		return
	}
	pendingCLIEvents[key] = struct{}{}
	telemetryMu.Unlock()
	started := capturePosthogEvent("First Resource CLI", map[string]any{"resource_category": kind}, func(success bool) {
		telemetryMu.Lock()
		defer telemetryMu.Unlock()
		delete(pendingCLIEvents, key)
		if success {
			state.FirstResource = true
			// Persist only the marker. The cached "cli" can be older than a
			// version another CLI process recorded since this one loaded, and
			// writing it back would make that version send "Installed CLI" again.
			marker := *state
			marker.CLI = ""
			saveTelemetryState(&marker)
		}
	})
	if !started {
		telemetryMu.Lock()
		delete(pendingCLIEvents, key)
		telemetryMu.Unlock()
	}
}

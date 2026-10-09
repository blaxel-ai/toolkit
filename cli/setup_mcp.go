package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

// mcpServer is a Blaxel MCP server that setup adds to coding agents: a hosted
// server at url, or a local command the agent starts.
type mcpServer struct {
	name    string
	url     string
	command []string
	// plugin reports whether the Blaxel agent plugin already provides this server.
	plugin bool
}

func (s mcpServer) local() bool { return len(s.command) > 0 }

const docsMCPURL = "https://docs.blaxel.ai/mcp"

// resourceMCPServer manages workspace resources through bl mcp, which signs
// in with the bl login: agents need no sign-in of their own, and no token is
// stored in their configuration.
func resourceMCPServer(bl string) mcpServer {
	return mcpServer{name: "blaxel", command: []string{bl, "mcp"}, plugin: true}
}

func docsMCPServer() mcpServer {
	return mcpServer{name: "blaxel-docs", url: docsMCPURL}
}

// isHostedResourceMCPURL recognizes the hosted server that setup added to
// agents before bl mcp, which it now replaces.
func isHostedResourceMCPURL(value string) bool {
	value = strings.TrimSuffix(strings.TrimSpace(value), "/")
	return value == "https://api.blaxel.ai/v0/mcp" || value == "https://api.blaxel.dev/v0/mcp"
}

// blCommandPath is the bl binary agents start for bl mcp. It is absolute,
// because desktop apps do not get the shell's PATH, and survives upgrades:
// Homebrew's bin link rather than the versioned keg.
func blCommandPath(executable func() (string, error)) (string, error) {
	path, err := executable()
	if err != nil || path == "" {
		return "", errors.New("cannot locate the bl executable; run setup from an installed bl or blaxel binary")
	}
	if resolved, err := filepath.EvalSymlinks(path); err == nil {
		if prefix, _ := homebrewSkillsLocation(resolved); prefix != "" {
			return filepath.Join(prefix, "bin", "bl"), nil
		}
	}
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve the absolute bl executable path: %w", err)
	}
	return absolute, nil
}

// mcpEnv is everything MCP configuration needs from the machine, so tests can
// point it at a temporary home and fake agent CLIs.
type mcpEnv struct {
	home, config string
	env          func(string) string
	lookPath     func(string) (string, error)
	run          func(ctx context.Context, name string, args ...string) ([]byte, error)
	// exists reports whether a file exists, to notice a bl that moved.
	exists func(string) bool
}

func newMCPEnv(home string) mcpEnv {
	paths := newSkillsAgentPaths(home, os.Getenv)
	return mcpEnv{
		home: home, config: paths.config, env: os.Getenv, lookPath: exec.LookPath,
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Stdin = nil
			cmd.WaitDelay = time.Second
			return cmd.CombinedOutput()
		},
		exists: func(path string) bool { _, err := os.Stat(path); return err == nil },
	}
}

func (e mcpEnv) envOr(key, fallback string) string {
	if value := strings.TrimSpace(e.env(key)); value != "" {
		return value
	}
	return fallback
}

func (e mcpEnv) paths() skillsAgentPaths {
	return skillsAgentPaths{home: e.home, config: e.config, env: e.env}
}

// mcpEntryState compares a configured server with the one setup writes.
type mcpEntryState int

const (
	mcpEntryAbsent   mcpEntryState = iota
	mcpEntryCurrent                // what setup writes, or the same server
	mcpEntryOutdated               // one setup wrote before, which it replaces
	mcpEntryCustom                 // someone else's entry, left alone
)

// classifyMCPEntry decides what setup does with an existing entry. Only
// exact minimal known shapes are repairable; this is not proof of ownership.
// Custom arguments, env, headers, unknown keys and disabled entries stay intact.
func classifyMCPEntry(e mcpEnv, server mcpServer, entry map[string]any) mcpEntryState {
	if entry == nil {
		return mcpEntryAbsent
	}
	if !server.local() {
		return mcpEntryCurrent
	}
	// Never re-enable a disabled server or overwrite customization.
	if disabled, _ := entry["disabled"].(bool); disabled {
		return mcpEntryCustom
	}
	if enabled, ok := entry["enabled"]; ok && enabled != true {
		return mcpEntryCustom
	}
	if command, args := entryCommand(entry); command != "" {
		name := strings.TrimSuffix(strings.ToLower(filepath.Base(strings.ReplaceAll(command, `\`, "/"))), ".exe")
		if (name != "bl" && name != "blaxel") || len(args) != 1 || args[0] != "mcp" {
			return mcpEntryCustom
		}
		for key, value := range entry {
			switch key {
			case "command", "args", "enabled":
			case "env":
				// claude mcp add writes an empty one: not a customization.
				if env, ok := value.(map[string]any); !ok || len(env) > 0 {
					return mcpEntryCustom
				}
			case "type":
				if value != "stdio" && value != "local" {
					return mcpEntryCustom
				}
			default:
				return mcpEntryCustom
			}
		}
		if filepath.IsAbs(command) && e.exists != nil && !e.exists(command) {
			return mcpEntryOutdated
		}
		return mcpEntryCurrent
	}
	hosted := false
	for key, value := range entry {
		switch key {
		case "url", "httpUrl", "serverUrl":
			text, _ := value.(string)
			if !isHostedResourceMCPURL(text) {
				return mcpEntryCustom
			}
			hosted = true
		case "type":
			if value != "http" && value != "remote" && value != "streamable-http" {
				return mcpEntryCustom
			}
		case "enabled":
		default:
			return mcpEntryCustom
		}
	}
	if !hosted {
		return mcpEntryCustom
	}
	return mcpEntryOutdated
}

// entryCommand reads a local server's command and arguments, in the common
// form ("command" and "args") or OpenCode's (one "command" list).
func entryCommand(entry map[string]any) (string, []string) {
	var parts []string
	switch command := entry["command"].(type) {
	case string:
		parts = append(parts, command)
	case []any:
		for _, part := range command {
			text, _ := part.(string)
			parts = append(parts, text)
		}
	}
	if args, ok := entry["args"].([]any); ok {
		for _, arg := range args {
			text, _ := arg.(string)
			parts = append(parts, text)
		}
	}
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

// mcpTarget knows how to add an MCP server to one coding agent.
type mcpTarget struct {
	// file is the configuration file setup edits, shown in messages.
	file func(mcpEnv) string
	// entry returns the configured server with that name, or nil.
	entry func(e mcpEnv, name string) map[string]any
	// write adds the server, replacing the existing entry when replace is set.
	write func(ctx context.Context, e mcpEnv, server mcpServer, replace bool) error
	// hasPlugin reports whether the Blaxel plugin is installed in this agent,
	// and pluginDirs where its files are.
	hasPlugin  func(mcpEnv) bool
	pluginDirs func(mcpEnv) []string
	// localOnly targets run local servers only, so they get bl mcp alone.
	localOnly bool
}

// pluginServes reports whether the installed Blaxel plugin supplies the blaxel
// server itself. Setup then adds none, because the agent would have two. A
// plugin without an MCP server (skills only) leaves that to setup.
func (t mcpTarget) pluginServes(e mcpEnv) bool {
	if t.hasPlugin == nil || !t.hasPlugin(e) {
		return false
	}
	dirs := t.pluginDirs(e)
	if len(dirs) == 0 {
		return true // installed, but its files cannot be found
	}
	for _, dir := range dirs {
		for _, name := range []string{".mcp.json", "mcp.json"} {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			var config struct {
				Servers map[string]json.RawMessage `json:"mcpServers"`
			}
			if err != nil || json.Unmarshal(data, &config) != nil || config.Servers["blaxel"] != nil {
				return true
			}
		}
	}
	return false
}

// takes reports whether the target can run the server.
func (t mcpTarget) takes(server mcpServer) bool { return server.local() || !t.localOnly }

type mcpChange int

const (
	mcpUnchanged mcpChange = iota
	mcpAdded
	mcpReplaced
)

// addMCPServer adds the server unless an entry is already there, and
// replaces only an outdated entry setup wrote before.
func addMCPServer(ctx context.Context, e mcpEnv, target mcpTarget, server mcpServer) (mcpChange, error) {
	switch classifyMCPEntry(e, server, target.entry(e, server.name)) {
	case mcpEntryCurrent, mcpEntryCustom:
		return mcpUnchanged, nil
	case mcpEntryOutdated:
		if err := target.write(ctx, e, server, true); err != nil {
			if errors.Is(err, errMCPServerExists) {
				return mcpUnchanged, nil // a layout setup cannot rewrite is left as it is
			}
			return mcpUnchanged, err
		}
		return mcpReplaced, nil
	}
	if err := target.write(ctx, e, server, false); err != nil {
		if errors.Is(err, errMCPServerExists) {
			return mcpUnchanged, nil
		}
		return mcpUnchanged, err
	}
	return mcpAdded, nil
}

// errMCPServerExists is returned by agent CLIs that already have the server.
var errMCPServerExists = errors.New("the server is already configured")

// jsonServerTarget keeps servers under container in a JSON configuration file.
func jsonServerTarget(file func(mcpEnv) string, container string, entry func(mcpServer) any) mcpTarget {
	return mcpTarget{file: file,
		entry: func(e mcpEnv, name string) map[string]any { return jsonConfigEntry(file(e), container, name) },
		write: func(_ context.Context, e mcpEnv, server mcpServer, replace bool) error {
			_, err := upsertJSONConfig(file(e), container, server.name, entry(server), replace)
			return err
		},
	}
}

// commandOrURL writes a local server in the common form, and a hosted one
// with the agent's URL key.
func commandOrURL(urlKey string) func(mcpServer) any {
	return func(s mcpServer) any {
		if s.local() {
			return localServerEntry{Command: s.command[0], Args: s.command[1:]}
		}
		return map[string]string{urlKey: s.url}
	}
}

// localServerEntry is the common JSON form of a local server, command first.
type localServerEntry struct {
	Type    string            `json:"type,omitempty"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env,omitempty"`
}

// mcpTargets maps agent IDs to their MCP configuration. Agents without an
// entry receive the skills only.
var mcpTargets = map[string]mcpTarget{
	"claude-code": {
		file: claudeConfigFile,
		entry: func(e mcpEnv, name string) map[string]any {
			return jsonConfigEntry(claudeConfigFile(e), "mcpServers", name)
		},
		write:     writeClaudeMCPServer,
		hasPlugin: func(e mcpEnv) bool { return len(installedClaudePlugins(e)) > 0 },
		pluginDirs: func(e mcpEnv) []string {
			var dirs []string
			for _, raw := range installedClaudePlugins(e) {
				var installs []struct {
					InstallPath string `json:"installPath"`
				}
				_ = json.Unmarshal(raw, &installs)
				for _, install := range installs {
					if install.InstallPath != "" {
						dirs = append(dirs, install.InstallPath)
					}
				}
			}
			return dirs
		},
	},
	"codex": {
		file: codexConfigFile,
		entry: func(e mcpEnv, name string) map[string]any {
			var config struct {
				MCPServers map[string]map[string]any `toml:"mcp_servers"`
			}
			if _, err := toml.DecodeFile(codexConfigFile(e), &config); err != nil {
				return nil
			}
			return config.MCPServers[name]
		},
		write: func(_ context.Context, e mcpEnv, server mcpServer, replace bool) error {
			if replace {
				return replaceCodexMCPServer(codexConfigFile(e), server)
			}
			_, err := appendCodexMCPServer(codexConfigFile(e), server)
			return err
		},
		hasPlugin: func(e mcpEnv) bool {
			var config struct {
				Plugins map[string]struct {
					Enabled *bool `toml:"enabled"`
				} `toml:"plugins"`
			}
			if _, err := toml.DecodeFile(codexConfigFile(e), &config); err != nil {
				return false
			}
			for id, plugin := range config.Plugins {
				if strings.HasPrefix(id, "blaxel@") && (plugin.Enabled == nil || *plugin.Enabled) {
					return true
				}
			}
			return false
		},
		pluginDirs: func(e mcpEnv) []string {
			dirs, _ := filepath.Glob(filepath.Join(filepath.Dir(codexConfigFile(e)), "plugins", "cache", "*", "blaxel", "*"))
			return dirs
		},
	},
	"cursor": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.home, ".cursor", "mcp.json") },
		"mcpServers", commandOrURL("url")),
	"gemini-cli": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.home, ".gemini", "settings.json") },
		"mcpServers", commandOrURL("httpUrl")),
	"opencode": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.config, "opencode", "opencode.json") },
		"mcp", func(s mcpServer) any {
			if s.local() {
				return map[string]any{"type": "local", "command": s.command, "enabled": true}
			}
			return map[string]any{"type": "remote", "url": s.url, "enabled": true}
		}),
	"github-copilot": copilotMCPTarget(),
	"vscode": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(vscodeUserDir(e.paths()), "mcp.json") },
		"servers", func(s mcpServer) any {
			if s.local() {
				return localServerEntry{Type: "stdio", Command: s.command[0], Args: s.command[1:]}
			}
			return map[string]string{"type": "http", "url": s.url}
		}),
	"amp":   jsonServerTarget(ampConfigFile, "amp.mcpServers", commandOrURL("url")),
	"goose": gooseMCPTarget(),
	"kiro-cli": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.home, ".kiro", "settings", "mcp.json") },
		"mcpServers", commandOrURL("url")),
	"qwen-code": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.home, ".qwen", "settings.json") },
		"mcpServers", commandOrURL("httpUrl")),
	"cline":    clineMCPTarget(),
	"continue": continueMCPTarget(),
	"junie": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.home, ".junie", "mcp", "mcp.json") },
		"mcpServers", commandOrURL("url")),
	"augment": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.home, ".augment", "settings.json") },
		"mcpServers", typedCommandOrURL("", "http")),
	"openhands": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.home, ".openhands", "mcp.json") },
		"mcpServers", commandOrURL("url")),
	"crush":    crushMCPTarget(),
	"openclaw": openclawMCPTarget(),
	"windsurf": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.home, ".codeium", "windsurf", "mcp_config.json") },
		"mcpServers", commandOrURL("serverUrl")),
	"devin": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.config, "devin", "mcp_config.json") },
		"mcpServers", commandOrURL("serverUrl")),
	// Claude Desktop's configuration file runs local servers only; hosted
	// ones are connectors, added in the app.
	"claude-desktop": func() mcpTarget {
		target := jsonServerTarget(func(e mcpEnv) string {
			return filepath.Join(claudeDesktopDir(e.paths()), "claude_desktop_config.json")
		}, "mcpServers", commandOrURL("url"))
		target.localOnly = true
		return target
	}(),
}

// installedClaudePlugins returns the installed Blaxel plugins, by plugin id.
func installedClaudePlugins(e mcpEnv) map[string]json.RawMessage {
	data, err := os.ReadFile(filepath.Join(claudeConfigDir(e), "plugins", "installed_plugins.json"))
	if err != nil {
		return nil
	}
	var installed struct {
		Plugins map[string]json.RawMessage `json:"plugins"`
	}
	_ = json.Unmarshal(data, &installed)
	for id := range installed.Plugins {
		if !strings.HasPrefix(id, "blaxel@") {
			delete(installed.Plugins, id)
		}
	}
	return installed.Plugins
}

func claudeConfigDir(e mcpEnv) string {
	return e.envOr("CLAUDE_CONFIG_DIR", filepath.Join(e.home, ".claude"))
}

// Claude Code keeps user-scoped MCP servers in .claude.json: in the home
// directory by default, or inside CLAUDE_CONFIG_DIR when that is set.
func claudeConfigFile(e mcpEnv) string {
	if dir := strings.TrimSpace(e.env("CLAUDE_CONFIG_DIR")); dir != "" {
		return filepath.Join(dir, ".claude.json")
	}
	return filepath.Join(e.home, ".claude.json")
}

func codexConfigFile(e mcpEnv) string {
	return filepath.Join(e.envOr("CODEX_HOME", filepath.Join(e.home, ".codex")), "config.toml")
}

// writeClaudeMCPServer prefers the claude CLI, which owns .claude.json, and
// edits the file directly only when the CLI cannot be found. A file it cannot
// read (such as one with comments) is left to the CLI.
func writeClaudeMCPServer(ctx context.Context, e mcpEnv, server mcpServer, replace bool) error {
	file := claudeConfigFile(e)
	_, _, readErr := readJSONConfig(file)
	claude, err := e.lookPath("claude")
	if err != nil {
		// Only fixed home locations: never run a binary from a directory an
		// environment variable such as CLAUDE_CONFIG_DIR selects.
		for _, candidate := range []string{filepath.Join(e.home, ".local", "bin", "claude"), filepath.Join(e.home, ".claude", "local", "claude")} {
			if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
				claude, err = candidate, nil
				break
			}
		}
	}
	if err != nil {
		if readErr != nil {
			return readErr
		}
		var entry any = map[string]string{"type": "http", "url": server.url}
		if server.local() {
			entry = localServerEntry{Type: "stdio", Command: server.command[0], Args: server.command[1:]}
		}
		_, err := upsertJSONConfig(file, "mcpServers", server.name, entry, replace)
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	var previous map[string]any
	if replace {
		previous = jsonConfigEntry(file, "mcpServers", server.name)
		if previous == nil || readErr != nil {
			return errors.New("cannot safely back up the existing Claude MCP entry")
		}
		if _, err = e.run(ctx, claude, "mcp", "remove", "--scope", "user", server.name); err != nil {
			return fmt.Errorf("claude mcp remove: %w", err)
		}
	}
	args := []string{"mcp", "add", "--scope", "user", "--transport", "http", server.name, server.url}
	if server.local() {
		args = append([]string{"mcp", "add", "--scope", "user", server.name, "--"}, server.command...)
	}
	output, err := e.run(ctx, claude, args...)
	if err != nil {
		if previous != nil {
			if _, restoreErr := upsertJSONConfig(file, "mcpServers", server.name, previous, true); restoreErr != nil {
				return fmt.Errorf("claude mcp add failed (%w); restoring the old entry also failed: %v", err, restoreErr)
			}
		}
		if strings.Contains(string(output), "already exists") {
			return errMCPServerExists
		}
		return fmt.Errorf("claude mcp add: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return nil
}

// codexServerTable is the [mcp_servers.NAME] table setup writes.
func codexServerTable(server mcpServer) (string, error) {
	if !server.local() {
		return fmt.Sprintf("[mcp_servers.%s]\nurl = %s\n", server.name, tomlString(server.url)), nil
	}
	args, err := json.Marshal(server.command[1:])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("[mcp_servers.%s]\ncommand = %s\nargs = %s\n", server.name, tomlString(server.command[0]), args), nil
}

// tomlString quotes a string as a TOML basic string. JSON string escapes are
// valid TOML, so Windows paths keep their backslashes.
func tomlString(value string) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

// appendCodexMCPServer appends an [mcp_servers.NAME] table, which keeps the
// user's comments and formatting. `codex mcp add` is not used because it
// starts an interactive OAuth sign-in for servers that support it.
func appendCodexMCPServer(file string, server mcpServer) (bool, error) {
	data, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	var config struct {
		MCPServers map[string]any `toml:"mcp_servers"`
	}
	if _, err := toml.Decode(string(data), &config); err != nil {
		return false, fmt.Errorf("not valid TOML: %w", err)
	}
	if _, exists := config.MCPServers[server.name]; exists {
		return false, nil
	}
	// TOML forbids adding tables to an inline table, and Codex refuses to start
	// with such a file, although the Go parser accepts it.
	if codexInlineMCPServers.Match(topLevelTOML(data)) {
		return false, errors.New("mcp_servers is an inline table, which cannot be extended")
	}
	table, err := codexServerTable(server)
	if err != nil {
		return false, err
	}
	var updated bytes.Buffer
	updated.Write(data)
	if len(data) > 0 {
		if !bytes.HasSuffix(data, []byte("\n")) {
			updated.WriteByte('\n')
		}
		updated.WriteByte('\n')
	}
	updated.WriteString(table)
	// An inline mcp_servers table, for example, cannot be extended this way.
	if _, err := toml.Decode(updated.String(), &config); err != nil {
		return false, fmt.Errorf("cannot add an [mcp_servers.%s] table: %w", server.name, err)
	}
	return true, writeConfigFile(file, updated.Bytes())
}

// replaceCodexMCPServer rewrites the body of an existing [mcp_servers.NAME]
// table and keeps every other line of the file as it was.
func replaceCodexMCPServer(file string, server mcpServer) error {
	data, err := os.ReadFile(file)
	if err != nil {
		return err
	}
	header := regexp.MustCompile(`^[ \t]*\[[ \t]*mcp_servers[ \t]*\.[ \t]*("?)` + regexp.QuoteMeta(server.name) + `("?)[ \t]*\][ \t]*(#.*)?$`)
	lines := strings.SplitAfter(string(data), "\n")
	start, end := -1, len(lines)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if start < 0 {
			if header.MatchString(strings.TrimRight(line, "\r\n")) {
				start = i
			}
			continue
		}
		if strings.HasPrefix(trimmed, "[") {
			end = i
			break
		}
	}
	if start < 0 {
		// An inline table or dotted keys: not a layout to rewrite line by line.
		return errMCPServerExists
	}
	table, err := codexServerTable(server)
	if err != nil {
		return err
	}
	// Keep the blank lines that separated the table from the next one.
	trailing := ""
	for i := end - 1; i > start && strings.TrimSpace(lines[i]) == ""; i-- {
		trailing += lines[i]
	}
	updated := strings.Join(lines[:start], "") + table + trailing + strings.Join(lines[end:], "")
	var config struct {
		MCPServers map[string]map[string]any `toml:"mcp_servers"`
	}
	if _, err := toml.Decode(updated, &config); err != nil {
		return fmt.Errorf("cannot replace the [mcp_servers.%s] table: %w", server.name, err)
	}
	if _, ok := config.MCPServers[server.name]["command"]; server.local() && !ok {
		return fmt.Errorf("cannot replace the [mcp_servers.%s] table", server.name)
	}
	return writeConfigFile(file, []byte(updated))
}

var codexInlineMCPServers = regexp.MustCompile(`(?m)^[ \t]*(mcp_servers|"mcp_servers"|'mcp_servers')[ \t]*=`)

// topLevelTOML returns the lines before the first table header, where a
// top-level key such as mcp_servers can be assigned directly.
func topLevelTOML(data []byte) []byte {
	for offset := 0; offset < len(data); {
		end := bytes.IndexByte(data[offset:], '\n')
		if end < 0 {
			end = len(data) - offset
		}
		if bytes.HasPrefix(bytes.TrimSpace(data[offset:offset+end]), []byte("[")) {
			return data[:offset]
		}
		offset += end + 1
	}
	return data
}

// jsonConfigEntry returns container.name from a JSON configuration file, or
// nil when it is missing or the file cannot be read.
func jsonConfigEntry(file, container, name string) map[string]any {
	_, members, err := readJSONConfig(file)
	if err != nil {
		return nil
	}
	index := findJSONMember(members, container)
	if index < 0 {
		return nil
	}
	servers, err := parseJSONObject(members[index].value)
	if err != nil {
		return nil
	}
	if found := findJSONMember(servers, name); found >= 0 {
		var entry map[string]any
		if json.Unmarshal(servers[found].value, &entry) == nil && entry != nil {
			return entry
		}
		return map[string]any{}
	}
	return nil
}

// jsonMember is one member of a JSON object. Values are kept verbatim so
// numbers, nested key order and unknown settings survive an edit.
type jsonMember struct {
	key   string
	value json.RawMessage
}

func parseJSONObject(data []byte) ([]jsonMember, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	if token, err := decoder.Token(); err != nil || token != json.Delim('{') {
		return nil, errors.New("expected a JSON object")
	}
	var members []jsonMember
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			return nil, err
		}
		key, ok := token.(string)
		if !ok {
			return nil, errors.New("expected an object key")
		}
		var value json.RawMessage
		if err := decoder.Decode(&value); err != nil {
			return nil, err
		}
		members = append(members, jsonMember{key, value})
	}
	if _, err := decoder.Token(); err != nil {
		return nil, err
	}
	if _, err := decoder.Token(); err != io.EOF {
		return nil, errors.New("unexpected content after the JSON object")
	}
	return members, nil
}

func encodeJSONObject(members []jsonMember) ([]byte, error) {
	var compact bytes.Buffer
	compact.WriteByte('{')
	for i, member := range members {
		if i > 0 {
			compact.WriteByte(',')
		}
		key, err := json.Marshal(member.key)
		if err != nil {
			return nil, err
		}
		compact.Write(key)
		compact.WriteByte(':')
		compact.Write(member.value)
	}
	compact.WriteByte('}')
	var indented bytes.Buffer
	if err := json.Indent(&indented, compact.Bytes(), "", "  "); err != nil {
		return nil, err
	}
	indented.WriteByte('\n')
	return indented.Bytes(), nil
}

func findJSONMember(members []jsonMember, key string) int {
	for i, member := range members {
		if member.key == key {
			return i
		}
	}
	return -1
}

func readJSONConfig(file string) ([]byte, []jsonMember, error) {
	data, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	members, err := parseJSONObject(data)
	if err != nil {
		// Comments (JSONC) or invalid JSON: leave the file to the user.
		return nil, nil, fmt.Errorf("not plain JSON: %w", err)
	}
	return data, members, nil
}

// upsertJSONConfig adds container.name = entry to a JSON configuration file.
// An existing entry is kept, unless replace is set. It returns whether the
// file changed.
func upsertJSONConfig(file, container, name string, entry any, replace bool) (bool, error) {
	_, members, err := readJSONConfig(file)
	if err != nil {
		return false, err
	}
	updated, changed, err := mergeJSONServer(members, container, name, entry, replace)
	if err != nil || !changed {
		return false, err
	}
	return true, writeConfigFile(file, updated)
}

func mergeJSONServer(members []jsonMember, container, name string, entry any, replace bool) ([]byte, bool, error) {
	var servers []jsonMember
	var err error
	index := findJSONMember(members, container)
	if index >= 0 {
		if servers, err = parseJSONObject(members[index].value); err != nil {
			return nil, false, fmt.Errorf("%q is not a JSON object: %w", container, err)
		}
	}
	existing := findJSONMember(servers, name)
	if existing >= 0 && !replace {
		return nil, false, nil
	}
	var value bytes.Buffer
	encoder := json.NewEncoder(&value)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(entry); err != nil {
		return nil, false, err
	}
	if existing >= 0 {
		// In place, so the servers keep their order.
		servers[existing].value = bytes.TrimSpace(value.Bytes())
	} else {
		servers = append(servers, jsonMember{name, bytes.TrimSpace(value.Bytes())})
	}
	encodedServers, err := encodeJSONObject(servers)
	if err != nil {
		return nil, false, err
	}
	if index >= 0 {
		members[index].value = encodedServers
	} else {
		members = append(members, jsonMember{container, encodedServers})
	}
	updated, err := encodeJSONObject(members)
	if err != nil {
		return nil, false, err
	}
	return updated, true, nil
}

// writeConfigFile replaces a configuration file atomically, keeping its mode.
// A symlinked file (for example one managed in a dotfiles repository) is
// updated at its target, so the link survives.
func writeConfigFile(file string, data []byte) error {
	if resolved, err := filepath.EvalSymlinks(file); err == nil {
		file = resolved
	}
	mode := os.FileMode(0600)
	if info, err := os.Stat(file); err == nil {
		mode = info.Mode().Perm()
	}
	if err := os.MkdirAll(filepath.Dir(file), 0755); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(file), "."+filepath.Base(file)+".blaxel-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	if _, err := temp.Write(data); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(temp.Name(), file)
}

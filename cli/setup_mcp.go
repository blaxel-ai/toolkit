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
	blaxel "github.com/blaxel-ai/sdk-go"
)

// mcpServer is a hosted Blaxel MCP server that setup adds to coding agents.
type mcpServer struct {
	name string
	url  string
	// plugin reports whether the Blaxel agent plugin already provides this server.
	plugin bool
}

const docsMCPURL = "https://docs.blaxel.ai/mcp"

// resourceMCPServer manages workspace resources. It follows BL_ENV, like the
// rest of the CLI, and signs in with OAuth from the agent on first use.
func resourceMCPServer() mcpServer {
	return mcpServer{name: "blaxel", url: strings.TrimSuffix(blaxel.GetBaseURL(), "/") + "/mcp", plugin: true}
}

func docsMCPServer() mcpServer {
	return mcpServer{name: "blaxel-docs", url: docsMCPURL}
}

// mcpEnv is everything MCP configuration needs from the machine, so tests can
// point it at a temporary home and fake agent CLIs.
type mcpEnv struct {
	home, config string
	env          func(string) string
	lookPath     func(string) (string, error)
	run          func(ctx context.Context, name string, args ...string) ([]byte, error)
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
	}
}

func (e mcpEnv) envOr(key, fallback string) string {
	if value := strings.TrimSpace(e.env(key)); value != "" {
		return value
	}
	return fallback
}

// mcpTarget knows how to add a remote MCP server to one coding agent.
// add returns false when a server with that name is already configured.
type mcpTarget struct {
	// file is the configuration file setup edits, shown in messages.
	file func(mcpEnv) string
	add  func(context.Context, mcpEnv, mcpServer) (bool, error)
	// has reports whether a server with that name is configured.
	has func(e mcpEnv, name string) bool
	// hasPlugin reports whether the Blaxel plugin is installed in this agent.
	hasPlugin func(mcpEnv) bool
	// signIn tells the user how to sign in to the blaxel MCP server.
	signIn string
}

// jsonServerTarget adds servers under container in a JSON configuration file.
func jsonServerTarget(file func(mcpEnv) string, container, signIn string, entry func(mcpServer) any) mcpTarget {
	return mcpTarget{file: file, signIn: signIn,
		add: func(_ context.Context, e mcpEnv, server mcpServer) (bool, error) {
			return upsertJSONConfig(file(e), container, server.name, entry(server))
		},
		has: func(e mcpEnv, name string) bool {
			found, _ := jsonConfigHas(file(e), container, name)
			return found
		},
	}
}

// mcpTargets maps skills agent IDs to their MCP configuration. Agents without
// an entry receive the skills only.
var mcpTargets = map[string]mcpTarget{
	"claude-code": {
		file:   claudeConfigFile,
		add:    addClaudeMCPServer,
		signIn: "run /mcp, then select blaxel",
		has: func(e mcpEnv, name string) bool {
			found, _ := jsonConfigHas(claudeConfigFile(e), "mcpServers", name)
			return found
		},
		hasPlugin: func(e mcpEnv) bool {
			data, err := os.ReadFile(filepath.Join(claudeConfigDir(e), "plugins", "installed_plugins.json"))
			if err != nil {
				return false
			}
			var installed struct {
				Plugins map[string]json.RawMessage `json:"plugins"`
			}
			if json.Unmarshal(data, &installed) != nil {
				return false
			}
			for id := range installed.Plugins {
				if strings.HasPrefix(id, "blaxel@") {
					return true
				}
			}
			return false
		},
	},
	"codex": {
		file:   codexConfigFile,
		signIn: "codex mcp login blaxel",
		has: func(e mcpEnv, name string) bool {
			var config struct {
				MCPServers map[string]any `toml:"mcp_servers"`
			}
			_, err := toml.DecodeFile(codexConfigFile(e), &config)
			_, found := config.MCPServers[name]
			return err == nil && found
		},
		add: func(_ context.Context, e mcpEnv, server mcpServer) (bool, error) {
			return appendCodexMCPServer(codexConfigFile(e), server)
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
	},
	"cursor": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.home, ".cursor", "mcp.json") },
		"mcpServers", "cursor-agent mcp login blaxel",
		func(s mcpServer) any { return map[string]string{"url": s.url} }),
	"gemini-cli": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.home, ".gemini", "settings.json") },
		"mcpServers", "run /mcp auth blaxel",
		func(s mcpServer) any { return map[string]string{"httpUrl": s.url} }),
	"opencode": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.config, "opencode", "opencode.json") },
		"mcp", "opencode mcp auth blaxel", func(s mcpServer) any {
			return struct {
				Type    string `json:"type"`
				URL     string `json:"url"`
				Enabled bool   `json:"enabled"`
			}{"remote", s.url, true}
		}),
	"windsurf": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.home, ".codeium", "windsurf", "mcp_config.json") },
		"mcpServers", "refresh MCP servers in Cascade", func(s mcpServer) any { return map[string]string{"serverUrl": s.url} }),
	"devin": jsonServerTarget(func(e mcpEnv) string { return filepath.Join(e.config, "devin", "mcp_config.json") },
		"mcpServers", "refresh MCP servers in Devin", func(s mcpServer) any { return map[string]string{"serverUrl": s.url} }),
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

// addClaudeMCPServer prefers the claude CLI, which owns .claude.json, and edits
// the file directly only when the CLI cannot be found. A file it cannot read
// (such as one with comments) is left to the CLI.
func addClaudeMCPServer(ctx context.Context, e mcpEnv, server mcpServer) (bool, error) {
	file := claudeConfigFile(e)
	exists, readErr := jsonConfigHas(file, "mcpServers", server.name)
	if readErr == nil && exists {
		return false, nil
	}
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
			return false, readErr
		}
		return upsertJSONConfig(file, "mcpServers", server.name, map[string]string{"type": "http", "url": server.url})
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	output, err := e.run(ctx, claude, "mcp", "add", "--scope", "user", "--transport", "http", server.name, server.url)
	if err != nil {
		if strings.Contains(string(output), "already exists") {
			return false, nil
		}
		return false, fmt.Errorf("claude mcp add: %w: %s", err, strings.TrimSpace(string(output)))
	}
	return true, nil
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
	var updated bytes.Buffer
	updated.Write(data)
	if len(data) > 0 {
		if !bytes.HasSuffix(data, []byte("\n")) {
			updated.WriteByte('\n')
		}
		updated.WriteByte('\n')
	}
	fmt.Fprintf(&updated, "[mcp_servers.%s]\nurl = %q\n", server.name, server.url)
	// An inline mcp_servers table, for example, cannot be extended this way.
	if _, err := toml.Decode(updated.String(), &config); err != nil {
		return false, fmt.Errorf("cannot add an [mcp_servers.%s] table: %w", server.name, err)
	}
	return true, writeConfigFile(file, updated.Bytes())
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

func jsonConfigHas(file, container, name string) (bool, error) {
	_, members, err := readJSONConfig(file)
	if err != nil {
		return false, err
	}
	index := findJSONMember(members, container)
	if index < 0 {
		return false, nil
	}
	servers, err := parseJSONObject(members[index].value)
	if err != nil {
		return false, fmt.Errorf("%q is not a JSON object: %w", container, err)
	}
	return findJSONMember(servers, name) >= 0, nil
}

// upsertJSONConfig adds container.name = entry to a JSON configuration file
// unless the name already exists. It returns whether the file changed.
func upsertJSONConfig(file, container, name string, entry any) (bool, error) {
	_, members, err := readJSONConfig(file)
	if err != nil {
		return false, err
	}
	var servers []jsonMember
	index := findJSONMember(members, container)
	if index >= 0 {
		if servers, err = parseJSONObject(members[index].value); err != nil {
			return false, fmt.Errorf("%q is not a JSON object: %w", container, err)
		}
		if findJSONMember(servers, name) >= 0 {
			return false, nil
		}
	}
	var value bytes.Buffer
	encoder := json.NewEncoder(&value)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(entry); err != nil {
		return false, err
	}
	servers = append(servers, jsonMember{name, bytes.TrimSpace(value.Bytes())})
	encodedServers, err := encodeJSONObject(servers)
	if err != nil {
		return false, err
	}
	if index >= 0 {
		members[index].value = encodedServers
	} else {
		members = append(members, jsonMember{container, encodedServers})
	}
	updated, err := encodeJSONObject(members)
	if err != nil {
		return false, err
	}
	return true, writeConfigFile(file, updated)
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

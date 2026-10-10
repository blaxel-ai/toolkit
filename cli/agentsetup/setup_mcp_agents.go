package agentsetup

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func typedCommandOrURL(localType, remoteType string) func(MCPServer) any {
	return func(s MCPServer) any {
		if s.local() {
			return localServerEntry{Type: localType, Command: s.Command[0], Args: s.Command[1:]}
		}
		return map[string]string{"type": remoteType, "url": s.URL}
	}
}

func crushConfigDir(p SkillsAgentPaths) string {
	return p.envOr("CRUSH_GLOBAL_CONFIG", p.configDir("crush"))
}

func crushMCPTarget() MCPTarget {
	file := func(e MCPEnv) string { return filepath.Join(crushConfigDir(e.paths()), "crush.json") }
	target := jsonServerTarget(file, "mcp", typedCommandOrURL("stdio", "http"))
	write := target.write
	target.write = func(ctx context.Context, e MCPEnv, s MCPServer, replace bool) error {
		if _, err := os.Stat(filepath.Join(filepath.Dir(file(e)), "crushrc")); !errors.Is(err, os.ErrNotExist) {
			return errors.New("configure MCP in crushrc; setup cannot safely merge shell configuration")
		}
		return write(ctx, e, s, replace)
	}
	return target
}

// clineMCPSettingsFile resolves the file the Cline CLI reads:
// CLINE_MCP_SETTINGS_PATH, else the data folder from CLINE_DATA_DIR, CLINE_DIR
// or ~/.cline.
func clineMCPSettingsFile(e MCPEnv) string {
	data := e.envOr("CLINE_DATA_DIR", filepath.Join(e.envOr("CLINE_DIR", filepath.Join(e.Home, ".cline")), "data"))
	return e.envOr("CLINE_MCP_SETTINGS_PATH", filepath.Join(data, "settings", "cline_mcp_settings.json"))
}

func clineMCPTarget() MCPTarget {
	file := clineMCPSettingsFile
	target := jsonServerTarget(file, "mcpServers", typedCommandOrURL("", "streamableHttp"))
	target.Entry = func(e MCPEnv, name string) map[string]any {
		entry := jsonConfigEntry(file(e), "mcpServers", name)
		if entry["type"] == "streamableHttp" {
			entry["type"] = "http"
		}
		return entry
	}
	return target
}

func continueMCPTarget() MCPTarget {
	file := func(e MCPEnv) string {
		return filepath.Join(e.envOr("CONTINUE_GLOBAL_DIR", filepath.Join(e.Home, ".continue")), "mcpServers", "blaxel.json")
	}
	target := jsonServerTarget(file, "mcpServers", typedCommandOrURL("stdio", "http"))
	owns := func(e MCPEnv, name string) (bool, error) {
		// A blaxel.json holding a single server is the user's own "blaxel"
		// server, so setup merges nothing into it.
		if data, err := os.ReadFile(file(e)); err == nil && singleServerJSON(data) {
			if name == strings.TrimSuffix(filepath.Base(file(e)), ".json") {
				return true, nil
			}
			return false, fmt.Errorf("%s is your own server file; setup leaves it alone", file(e))
		} else if err != nil && !errors.Is(err, os.ErrNotExist) {
			return false, err
		}
		root := filepath.Dir(filepath.Dir(file(e)))
		// Main and sibling configs belong to the user. A same-name server there
		// takes ownership, so setup does not create a duplicate in blaxel.json.
		files := []string{filepath.Join(root, "config.yaml"), filepath.Join(root, "config.json")}
		if err := filepath.WalkDir(filepath.Dir(file(e)), func(path string, d os.DirEntry, err error) error {
			if errors.Is(err, os.ErrNotExist) {
				return nil
			}
			if err != nil {
				return err
			}
			if !d.IsDir() && path != file(e) {
				files = append(files, path)
			}
			return nil
		}); err != nil {
			return false, err
		}
		for _, path := range files {
			switch filepath.Ext(path) {
			case ".json", ".yaml", ".yml":
			default:
				continue
			}
			data, err := os.ReadFile(path)
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			if err != nil {
				return false, err
			}
			if filepath.Ext(path) == ".json" && !json.Valid(data) {
				return false, fmt.Errorf("cannot safely inspect %s: not plain JSON", path)
			}
			if filepath.Ext(path) == ".json" && filepath.Dir(path) != root && singleServerJSON(data) {
				if strings.TrimSuffix(filepath.Base(path), ".json") == name {
					return true, nil
				}
				continue
			}
			var config struct {
				Servers any `yaml:"mcpServers"`
			}
			if err := yaml.Unmarshal(data, &config); err != nil {
				return false, fmt.Errorf("cannot safely inspect %s: %w", path, err)
			}
			switch servers := config.Servers.(type) {
			case map[string]any:
				if _, ok := servers[name]; ok {
					return true, nil
				}
			case []any:
				for _, server := range servers {
					if entry, ok := server.(map[string]any); ok && entry["name"] == name {
						return true, nil
					}
				}
			}
		}
		return false, nil
	}
	target.Entry = func(e MCPEnv, name string) map[string]any {
		if exists, err := owns(e, name); err == nil && exists {
			return map[string]any{}
		}
		return jsonConfigEntry(file(e), "mcpServers", name)
	}
	write := target.write
	target.write = func(ctx context.Context, e MCPEnv, s MCPServer, replace bool) error {
		if exists, err := owns(e, s.Name); err != nil {
			return err
		} else if exists {
			return errMCPServerExists
		}
		return write(ctx, e, s, replace)
	}
	return target
}

// singleServerJSON reports a server written as the whole file, without an
// mcpServers or projects member. Continue loads it as the server named after
// the file.
func singleServerJSON(data []byte) bool {
	var config map[string]any
	if json.Unmarshal(data, &config) != nil {
		return false
	}
	if _, ok := config["mcpServers"]; ok {
		return false
	}
	if _, ok := config["projects"]; ok {
		return false
	}
	_, command := config["command"].(string)
	_, url := config["url"].(string)
	return command || url
}

// openclawLegacyOnly reports an install still in a legacy ~/.clawdbot or
// ~/.moltbot folder. OpenClaw reads only ~/.openclaw, and openclaw doctor moves
// a legacy folder there only while ~/.openclaw does not exist, so setup must not
// create it first.
func openclawLegacyOnly(e MCPEnv) bool {
	if strings.TrimSpace(e.Env("OPENCLAW_CONFIG_PATH")) != "" || strings.TrimSpace(e.Env("OPENCLAW_STATE_DIR")) != "" {
		return false
	}
	if _, err := os.Stat(filepath.Join(e.Home, ".openclaw")); !errors.Is(err, os.ErrNotExist) {
		return false
	}
	for _, legacy := range []string{".clawdbot", ".moltbot"} {
		if _, err := os.Stat(filepath.Join(e.Home, legacy)); err == nil {
			return true
		}
	}
	return false
}

func openclawConfigFile(e MCPEnv) string {
	path := e.envOr("OPENCLAW_CONFIG_PATH", filepath.Join(e.envOr("OPENCLAW_STATE_DIR", filepath.Join(e.Home, ".openclaw")), "openclaw.json"))
	if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
		path = filepath.Join(e.Home, path[2:])
	}
	return path
}

// OpenClaw's native registry is nested. Leave JSON5 and included settings to
// the client, and preserve every unrelated JSON member while merging servers.
func openclawMCPTarget() MCPTarget {
	read := func(e MCPEnv) ([]jsonMember, []jsonMember, error) {
		_, root, err := readJSONConfig(openclawConfigFile(e))
		if err != nil {
			return nil, nil, err
		}
		if findJSONMember(root, "$include") >= 0 {
			return nil, nil, errors.New("cannot safely edit included MCP settings")
		}
		var mcp []jsonMember
		if i := findJSONMember(root, "mcp"); i >= 0 {
			mcp, err = parseJSONObject(root[i].value)
		}
		if err != nil || findJSONMember(mcp, "$include") >= 0 {
			return nil, nil, errors.New("cannot safely edit the MCP settings object")
		}
		if i := findJSONMember(mcp, "servers"); i >= 0 {
			servers, err := parseJSONObject(mcp[i].value)
			if err != nil || findJSONMember(servers, "$include") >= 0 {
				return nil, nil, errors.New("cannot safely edit the MCP servers object")
			}
		}
		return root, mcp, nil
	}
	return MCPTarget{
		File: openclawConfigFile,
		Entry: func(e MCPEnv, name string) map[string]any {
			_, mcp, err := read(e)
			if err != nil {
				return nil
			}
			index := findJSONMember(mcp, "servers")
			if index < 0 {
				return nil
			}
			servers, err := parseJSONObject(mcp[index].value)
			if err != nil {
				return nil
			}
			index = findJSONMember(servers, name)
			if index < 0 {
				return nil
			}
			var entry map[string]any
			if json.Unmarshal(servers[index].value, &entry) != nil || entry == nil {
				return map[string]any{}
			}
			if transport, ok := entry["transport"]; ok {
				if _, hasType := entry["type"]; !hasType {
					entry["type"] = transport
					delete(entry, "transport")
				}
			}
			return entry
		},
		write: func(_ context.Context, e MCPEnv, s MCPServer, replace bool) error {
			if openclawLegacyOnly(e) {
				return errors.New("run openclaw doctor to move the legacy OpenClaw folder to ~/.openclaw; setup does not create ~/.openclaw beside it")
			}
			root, mcp, err := read(e)
			if err != nil {
				return err
			}
			entry := map[string]any{"transport": "streamable-http", "url": s.URL}
			if s.local() {
				entry = map[string]any{"transport": "stdio", "command": s.Command[0], "args": s.Command[1:]}
			}
			updated, changed, err := mergeJSONServer(mcp, "servers", s.Name, entry, replace)
			if err != nil || !changed {
				return err
			}
			if i := findJSONMember(root, "mcp"); i >= 0 {
				root[i].value = updated
			} else {
				root = append(root, jsonMember{"mcp", updated})
			}
			data, err := encodeJSONObject(root)
			if err != nil {
				return err
			}
			return writeConfigFile(openclawConfigFile(e), data)
		},
	}
}

func copilotConfigFile(e MCPEnv) string {
	return filepath.Join(e.envOr("COPILOT_HOME", filepath.Join(e.Home, ".copilot")), "mcp-config.json")
}

func copilotMCPTarget() MCPTarget {
	target := jsonServerTarget(copilotConfigFile, "mcpServers", func(s MCPServer) any {
		entry := map[string]any{"type": "http", "url": s.URL, "tools": []string{"*"}}
		if s.local() {
			entry = map[string]any{"type": "local", "command": s.Command[0], "args": s.Command[1:], "tools": []string{"*"}}
		}
		return entry
	})
	target.Entry = func(e MCPEnv, name string) map[string]any {
		entry := jsonConfigEntry(copilotConfigFile(e), "mcpServers", name)
		// Copilot's default tool selection is part of the minimal server shape.
		// A user's narrower tool selection remains a customization.
		if tools, ok := entry["tools"].([]any); ok && len(tools) == 1 && tools[0] == "*" {
			delete(entry, "tools")
		}
		return entry
	}
	// Ask Copilot for its resolved registry so disabled and skills-only plugins
	// are not mistaken for plugins that provide the resource server.
	target.hasPlugin = func(e MCPEnv) bool {
		if _, err := e.LookPath("copilot"); err != nil {
			return false
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		data, err := e.Run(ctx, "copilot", "mcp", "list", "--json")
		if err != nil {
			return false
		}
		var config struct {
			Servers map[string]struct {
				SourcePlugin string `json:"sourcePlugin"`
				Source       string `json:"source"`
				Enabled      bool   `json:"enabled"`
			} `json:"mcpServers"`
		}
		if json.Unmarshal(data, &config) != nil {
			return false
		}
		server := config.Servers["blaxel"]
		return server.SourcePlugin == "blaxel" && server.Source == "plugin" && server.Enabled
	}
	target.pluginDirs = func(MCPEnv) []string { return nil }
	return target
}

func ampConfigFile(e MCPEnv) string {
	file := filepath.Join(e.Config, "amp", "settings.json")
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		jsonc := file + "c"
		if _, err := os.Stat(jsonc); !errors.Is(err, os.ErrNotExist) {
			return jsonc
		}
	}
	return file
}

func gooseMCPTarget() MCPTarget {
	file := func(e MCPEnv) string { return filepath.Join(gooseConfigDir(e.paths()), "config.yaml") }
	return MCPTarget{
		File: file,
		Entry: func(e MCPEnv, name string) map[string]any {
			_, extensions, err := readGooseConfig(file(e))
			if err != nil {
				return nil
			}
			index := yamlMember(extensions, name)
			if index < 0 {
				return nil
			}
			if node := extensions.Content[index+1]; node.Kind == yaml.AliasNode || node.Anchor != "" {
				return map[string]any{} // a shared entry is a customization
			}
			var entry map[string]any
			if extensions.Content[index+1].Decode(&entry) != nil || entry == nil {
				return map[string]any{}
			}
			// Translate Goose's native fields for the shared migration classifier.
			for from, to := range map[string]string{"cmd": "command", "uri": "url", "envs": "env"} {
				if value, ok := entry[from]; ok {
					if _, exists := entry[to]; !exists {
						entry[to] = value
						delete(entry, from)
					}
				}
			}
			if entry["name"] == name {
				delete(entry, "name")
			}
			if entry["type"] == "streamable_http" {
				entry["type"] = "http"
			}
			return entry
		},
		write: func(_ context.Context, e MCPEnv, s MCPServer, replace bool) error {
			root, extensions, err := readGooseConfig(file(e))
			if err != nil {
				return err
			}
			index := yamlMember(extensions, s.Name)
			if index >= 0 && !replace {
				return nil
			}
			entry := map[string]any{"name": s.Name, "type": "streamable_http", "uri": s.URL, "enabled": true}
			if s.local() {
				entry = map[string]any{"name": s.Name, "type": "stdio", "cmd": s.Command[0], "args": s.Command[1:], "enabled": true}
			}
			var node yaml.Node
			if err := node.Encode(entry); err != nil {
				return err
			}
			if index >= 0 {
				extensions.Content[index+1] = &node
			} else {
				extensions.Content = append(extensions.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s.Name}, &node)
			}
			var data bytes.Buffer
			encoder := yaml.NewEncoder(&data)
			encoder.SetIndent(2)
			if err := encoder.Encode(root); err != nil {
				return err
			}
			if err := encoder.Close(); err != nil {
				return err
			}
			return writeConfigFile(file(e), data.Bytes())
		},
	}
}

// readGooseConfig keeps YAML nodes so unrelated entries and comments survive.
// Unsupported shapes are left alone, rather than rewriting ambiguous YAML.
func readGooseConfig(file string) (*yaml.Node, *yaml.Node, error) {
	data, err := os.ReadFile(file)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, err
	}
	root := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	if len(bytes.TrimSpace(data)) > 0 {
		decoder := yaml.NewDecoder(bytes.NewReader(data))
		var document yaml.Node
		if err := decoder.Decode(&document); errors.Is(err, io.EOF) {
			doc.HeadComment = string(bytes.TrimSpace(data))
		} else if err != nil {
			return nil, nil, fmt.Errorf("not valid YAML: %w", err)
		} else {
			if err := decoder.Decode(&yaml.Node{}); err != io.EOF {
				return nil, nil, errors.New("expected one YAML document")
			}
			doc, root = &document, document.Content[0]
			var config map[string]any
			if root.Kind != yaml.MappingNode {
				return nil, nil, errors.New("expected a YAML mapping")
			}
			if err := root.Decode(&config); err != nil {
				return nil, nil, fmt.Errorf("not valid YAML: %w", err)
			}
		}
	}
	if yamlMember(root, "<<") >= 0 {
		return nil, nil, errors.New("cannot safely edit inherited YAML settings")
	}
	if index := yamlMember(root, "extensions"); index >= 0 {
		extensions := root.Content[index+1]
		if extensions.Kind == yaml.ScalarNode && extensions.Tag == "!!null" {
			extensions.Kind, extensions.Tag, extensions.Value = yaml.MappingNode, "!!map", ""
		}
		if extensions.Kind != yaml.MappingNode {
			return nil, nil, errors.New("extensions is not a YAML mapping")
		}
		if yamlMember(extensions, "<<") >= 0 {
			return nil, nil, errors.New("cannot safely edit inherited YAML extensions")
		}
		return doc, extensions, nil
	}
	extensions := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	root.Content = append(root.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "extensions"}, extensions)
	return doc, extensions, nil
}

func yamlMember(node *yaml.Node, key string) int {
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value == key {
			return i
		}
	}
	return -1
}

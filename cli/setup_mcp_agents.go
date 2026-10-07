package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func copilotConfigFile(e mcpEnv) string {
	return filepath.Join(e.envOr("COPILOT_HOME", filepath.Join(e.home, ".copilot")), "mcp-config.json")
}

func copilotMCPTarget() mcpTarget {
	target := jsonServerTarget(copilotConfigFile, "mcpServers", func(s mcpServer) any {
		entry := map[string]any{"type": "http", "url": s.url, "tools": []string{"*"}}
		if s.local() {
			entry = map[string]any{"type": "local", "command": s.command[0], "args": s.command[1:], "tools": []string{"*"}}
		}
		return entry
	})
	target.entry = func(e mcpEnv, name string) map[string]any {
		entry := jsonConfigEntry(copilotConfigFile(e), "mcpServers", name)
		// Copilot's default tool selection is part of the minimal server shape.
		// A user's narrower tool selection remains a customization.
		if tools, ok := entry["tools"].([]any); ok && len(tools) == 1 && tools[0] == "*" {
			delete(entry, "tools")
		}
		return entry
	}
	return target
}

func ampConfigFile(e mcpEnv) string {
	file := filepath.Join(e.config, "amp", "settings.json")
	if _, err := os.Stat(file); errors.Is(err, os.ErrNotExist) {
		jsonc := file + "c"
		if _, err := os.Stat(jsonc); !errors.Is(err, os.ErrNotExist) {
			return jsonc
		}
	}
	return file
}

func gooseMCPTarget() mcpTarget {
	file := func(e mcpEnv) string { return filepath.Join(gooseConfigDir(e.paths()), "config.yaml") }
	return mcpTarget{
		file: file,
		entry: func(e mcpEnv, name string) map[string]any {
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
		write: func(_ context.Context, e mcpEnv, s mcpServer, replace bool) error {
			root, extensions, err := readGooseConfig(file(e))
			if err != nil {
				return err
			}
			index := yamlMember(extensions, s.name)
			if index >= 0 && !replace {
				return nil
			}
			entry := map[string]any{"name": s.name, "type": "streamable_http", "uri": s.url, "enabled": true}
			if s.local() {
				entry = map[string]any{"name": s.name, "type": "stdio", "cmd": s.command[0], "args": s.command[1:], "enabled": true}
			}
			var node yaml.Node
			if err := node.Encode(entry); err != nil {
				return err
			}
			if index >= 0 {
				extensions.Content[index+1] = &node
			} else {
				extensions.Content = append(extensions.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: s.name}, &node)
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

package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Plugin installation is not proof of a local MCP transport. Inspect the
// effective payload, without editing plugin caches or disabling their skills.
// Old plugin versions cannot be silently deduplicated by endpoint: agents give
// bundled servers separate names. Report the remaining migration explicitly.
func pluginResourceMCP(e mcpEnv, target mcpTarget) (bool, error) {
	if target.hasPlugin == nil || !target.hasPlugin(e) {
		return false, nil
	}
	var roots []string
	if strings.HasSuffix(target.file(e), ".toml") {
		cache := filepath.Join(filepath.Dir(codexConfigFile(e)), "plugins", "cache")
		matches, _ := filepath.Glob(filepath.Join(cache, "*", "blaxel", "*"))
		if len(matches) > 1 {
			return false, fmt.Errorf("cannot identify the effective Blaxel plugin version in %s; local bl mcp configured, but setup is not complete; reconcile only its bundled MCP server before reconnecting", cache)
		}
		roots = append(roots, matches...)
	} else {
		data, err := os.ReadFile(filepath.Join(claudeConfigDir(e), "plugins", "installed_plugins.json"))
		if err != nil {
			return false, errors.New("cannot inspect the Blaxel plugin's effective MCP configuration; local bl mcp configured, but setup is not complete")
		}
		var installed struct {
			Plugins map[string][]struct {
				InstallPath string `json:"installPath"`
			} `json:"plugins"`
		}
		if json.Unmarshal(data, &installed) == nil {
			for id, entries := range installed.Plugins {
				if strings.HasPrefix(id, "blaxel@") {
					for _, entry := range entries {
						if entry.InstallPath != "" {
							roots = append(roots, entry.InstallPath)
						}
					}
				}
			}
		}
	}
	if len(roots) == 0 {
		return false, errors.New("cannot locate the Blaxel plugin's effective MCP configuration; local bl mcp configured, but setup is not complete; update the plugin to a skills-only version or disable only its bundled Blaxel MCP server, then reconnect")
	}
	local := false
	for _, root := range roots {
		// Claude loads .mcp.json; other plugin hosts can use mcp.json.
		inspected := false
		for _, name := range []string{".mcp.json", "mcp.json"} {
			file := filepath.Join(root, name)
			data, err := os.ReadFile(file)
			if os.IsNotExist(err) {
				continue
			}
			if err != nil {
				return false, fmt.Errorf("cannot inspect plugin MCP configuration %s; setup is not complete", file)
			}
			var config struct {
				Servers map[string]map[string]any `json:"mcpServers"`
			}
			if json.Unmarshal(data, &config) != nil {
				return false, fmt.Errorf("invalid plugin MCP configuration %s; setup is not complete", file)
			}
			inspected = true
			entry := config.Servers["blaxel"]
			if entry == nil {
				continue
			}
			if enabled, ok := entry["enabled"]; ok && enabled == false {
				continue
			}
			command, args := entryCommand(entry)
			if command != "" && len(args) == 1 && args[0] == "mcp" && classifyMCPEntry(e, resourceMCPServer(command), entry) == mcpEntryCurrent {
				local = true
				continue
			}
			return false, fmt.Errorf("local bl mcp configured, but the Blaxel plugin still supplies a separate MCP server in %s; setup is not complete: disable only that bundled server (keep the skills) or update to a skills-only plugin, then reconnect to avoid duplicate tools and a second OAuth prompt", file)
		}
		if !inspected {
			// A verified installed plugin directory without an MCP file supplies
			// skills only. A missing directory is not proof of that migration.
			if info, err := os.Stat(root); err != nil || !info.IsDir() {
				return false, fmt.Errorf("cannot inspect plugin MCP configuration in %s; setup is not complete", root)
			}
		}
	}
	if local && target.entry(e, "blaxel") != nil {
		return false, errors.New("the Blaxel plugin and agent configuration both define MCP servers; setup is not complete: keep only one resource server, without disabling the skills, then reconnect")
	}
	return local, nil
}

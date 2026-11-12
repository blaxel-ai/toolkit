package agentsetup

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"sync"
)

// MCPSetupState is this machine's saved choice about the Blaxel MCP servers.
// bl uninstall turns automatic updates off, so CLI upgrades and the install
// scripts stop adding the servers back; an explicit bl setup turns them on.
type MCPSetupState struct {
	AutoUpdate *bool `json:"autoupdate,omitempty"`
}

func mcpSetupStatePath(home string) string {
	return filepath.Join(home, ".blaxel", "mcp", "state.json")
}

// ReadMCPSetupState reads the saved choice. A missing file means the default.
func ReadMCPSetupState(home string) (MCPSetupState, error) {
	var state MCPSetupState
	data, err := os.ReadFile(mcpSetupStatePath(home))
	if errors.Is(err, os.ErrNotExist) {
		return state, nil
	}
	if err != nil {
		return state, err
	}
	return state, json.Unmarshal(data, &state)
}

// MCPAutoUpdateOff reports whether automatic MCP setup is off. A state that
// cannot be read counts as off, so a damaged file never adds servers back.
func MCPAutoUpdateOff(home string) bool {
	state, err := ReadMCPSetupState(home)
	return err != nil || state.AutoUpdate != nil && !*state.AutoUpdate
}

// mcpAutoUpdateSavedOff reports whether the state file holds exactly the
// choice bl uninstall saves.
func mcpAutoUpdateSavedOff(home string) bool {
	state, err := ReadMCPSetupState(home)
	return err == nil && state.AutoUpdate != nil && !*state.AutoUpdate
}

// SetMCPAutoUpdate saves the choice. Turning it back on removes the saved
// state, which only ever holds this choice.
func SetMCPAutoUpdate(home string, enabled bool) error {
	file := mcpSetupStatePath(home)
	if !enabled {
		data, err := json.MarshalIndent(MCPSetupState{AutoUpdate: &enabled}, "", "  ")
		if err != nil {
			return err
		}
		return writeConfigFile(file, append(data, '\n'))
	}
	if _, err := os.Lstat(file); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return removeConfigFile(file, []byte("{}\n"))
}

// mcpCreatedPath is where setup lists the agent configuration files it
// created, which bl uninstall may delete once they are empty. It is kept apart
// from the saved choice, which bl setup removes.
func mcpCreatedPath(home string) string {
	return filepath.Join(home, ".blaxel", "mcp", "created.json")
}

// mcpCreatedMu serializes edits of the list: setup configures agents in parallel.
var mcpCreatedMu sync.Mutex

// mcpCreatedFiles reads the list. A missing or damaged list is empty.
func mcpCreatedFiles(home string) []string {
	var files []string
	data, err := os.ReadFile(mcpCreatedPath(home))
	if err != nil || json.Unmarshal(data, &files) != nil {
		return nil
	}
	return files
}

// mcpConfigCreated reports whether setup created the file.
func mcpConfigCreated(home, file string) bool {
	return slices.Contains(mcpCreatedFiles(home), file)
}

// setMCPConfigCreated records that setup created the file, or forgets it.
// The list is removed when nothing is left in it.
func setMCPConfigCreated(home, file string, created bool) error {
	mcpCreatedMu.Lock()
	defer mcpCreatedMu.Unlock()
	files := mcpCreatedFiles(home)
	if slices.Contains(files, file) == created {
		return nil
	}
	if created {
		files = append(files, file)
	} else {
		files = slices.DeleteFunc(files, func(path string) bool { return path == file })
	}
	list := mcpCreatedPath(home)
	if len(files) == 0 {
		return removeConfigFile(list, []byte("[]\n"))
	}
	data, err := json.MarshalIndent(files, "", "  ")
	if err != nil {
		return err
	}
	return writeConfigFile(list, append(data, '\n'))
}

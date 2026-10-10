package agentsetup

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkillsInstallCommand(t *testing.T) {
	assert.Equal(t, "bl skills install", skillsInstallCommand())
}

func TestSkillsInstallDisabled(t *testing.T) {
	tests := []struct {
		name     string
		env      map[string]string
		disabled bool
	}{
		{name: "unset", env: map[string]string{}, disabled: false},
		{name: "true", env: map[string]string{SkillsInstallEnv: "true"}, disabled: false},
		{name: "false", env: map[string]string{SkillsInstallEnv: "false"}, disabled: true},
		{name: "false with case and spaces", env: map[string]string{SkillsInstallEnv: " FALSE "}, disabled: true},
		{name: "other value", env: map[string]string{SkillsInstallEnv: "no"}, disabled: false},
		{name: "ci skipped by default", env: map[string]string{"CI": "true"}, disabled: true},
		{name: "github actions skipped by default", env: map[string]string{"GITHUB_ACTIONS": "true"}, disabled: true},
		{name: "ci forced with true", env: map[string]string{"CI": "true", SkillsInstallEnv: "true"}, disabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := func(key string) string { return tt.env[key] }
			assert.Equal(t, tt.disabled, SkillsInstallDisabled(env))
		})
	}
}

func TestSkillsUpdateWorkerSkipsOnlyTheResolvedCommand(t *testing.T) {
	for _, args := range [][]string{{"mcp"}, {"-w", "main", "skills", "status"}, {"setup", "--yes"}, {"upgrade"}, {"docs"}, {"__complete", "get", ""}} {
		assert.True(t, skillsUpdateWorkerSkipped(args), args)
	}
	for _, args := range [][]string{{"get", "skills"}, {"-w", "mcp", "get", "agents"}, {"new", "mcp"}, {"--workspace", "upgrade", "deploy"}} {
		assert.False(t, skillsUpdateWorkerSkipped(args), args)
	}
}

func TestSkillsUpdateWorkerEnvKeepsAgentHomesAndDropsCredentials(t *testing.T) {
	env := map[string]string{"HOME": "/h", "APPDATA": `C:\a`, "CONTINUE_GLOBAL_DIR": "/c", "GOOSE_PATH_ROOT": "/g", "CRUSH_GLOBAL_CONFIG": "/cr",
		"OPENCLAW_CONFIG_PATH": "/o.json", "OPENCLAW_STATE_DIR": "/o", "BL_API_KEY": "secret", "BL_CLIENT_CREDENTIALS": "secret", "BL_WORKSPACE": "w"}
	got := skillsUpdateWorkerEnv(func(name string) (string, bool) { value, ok := env[name]; return value, ok })
	for _, name := range []string{"HOME", "APPDATA", "CONTINUE_GLOBAL_DIR", "GOOSE_PATH_ROOT", "CRUSH_GLOBAL_CONFIG", "OPENCLAW_CONFIG_PATH", "OPENCLAW_STATE_DIR"} {
		assert.Contains(t, got, name+"="+env[name])
	}
	for _, entry := range got {
		assert.False(t, strings.HasPrefix(entry, "BL_API_KEY=") || strings.HasPrefix(entry, "BL_CLIENT_CREDENTIALS=") || strings.HasPrefix(entry, "BL_WORKSPACE="), entry)
	}
	assert.Contains(t, got, "DO_NOT_TRACK=1")
	assert.Contains(t, got, SkillsUpdateWorkerEnv+"=1")
}

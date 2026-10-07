package agentsetup

import (
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

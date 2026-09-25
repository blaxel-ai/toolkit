package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSkillsInstallCommand(t *testing.T) {
	assert.Equal(t, "npx -y skills add blaxel-ai/agent-skills -g --all", skillsInstallCommand())
}

func TestSkillsInstallDisabled(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		disabled bool
	}{
		{name: "unset", value: "", disabled: false},
		{name: "true", value: "true", disabled: false},
		{name: "false", value: "false", disabled: true},
		{name: "false with case and spaces", value: " FALSE ", disabled: true},
		{name: "other value", value: "no", disabled: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := func(key string) string {
				assert.Equal(t, skillsInstallEnv, key)
				return tt.value
			}
			assert.Equal(t, tt.disabled, skillsInstallDisabled(env))
		})
	}
}

func TestBuildCurlUpgradeCommand(t *testing.T) {
	const url = "https://example.com/install.sh"

	tests := []struct {
		name          string
		targetVersion string
		binDir        string
		needsSudo     bool
		expected      string
	}{
		{
			name:     "latest without sudo",
			binDir:   "/home/user/.local/bin",
			expected: "curl -fsSL https://example.com/install.sh | BL_INSTALL_SKILLS=false BINDIR=/home/user/.local/bin sh",
		},
		{
			name:          "specific version without sudo",
			targetVersion: "v1.2.3",
			binDir:        "/home/user/.local/bin",
			expected:      "curl -fsSL https://example.com/install.sh | BL_INSTALL_SKILLS=false VERSION=v1.2.3 BINDIR=/home/user/.local/bin sh",
		},
		{
			name:      "latest with sudo",
			binDir:    "/usr/local/bin",
			needsSudo: true,
			expected:  "curl -fsSL https://example.com/install.sh | BL_INSTALL_SKILLS=false BINDIR=/usr/local/bin sudo -E sh",
		},
		{
			name:          "specific version with sudo",
			targetVersion: "v1.2.3",
			binDir:        "/usr/local/bin",
			needsSudo:     true,
			expected:      "curl -fsSL https://example.com/install.sh | BL_INSTALL_SKILLS=false VERSION=v1.2.3 BINDIR=/usr/local/bin sudo -E sh",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, buildCurlUpgradeCommand(url, tt.targetVersion, tt.binDir, tt.needsSudo))
		})
	}
}

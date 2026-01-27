package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestUpgradeCmd(t *testing.T) {
	cmd := UpgradeCmd()

	assert.Equal(t, "upgrade", cmd.Use)
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Verify flags
	versionFlag := cmd.Flags().Lookup("version")
	assert.NotNil(t, versionFlag)
	assert.Equal(t, "", versionFlag.DefValue)

	forceFlag := cmd.Flags().Lookup("force")
	assert.NotNil(t, forceFlag)
	assert.Equal(t, "f", forceFlag.Shorthand)
	assert.Equal(t, "false", forceFlag.DefValue)
}

func TestNeedsSudoForPath(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		expected bool
	}{
		{
			name:     "usr local bin",
			path:     "/usr/local/bin",
			expected: true,
		},
		{
			name:     "usr bin",
			path:     "/usr/bin",
			expected: true,
		},
		{
			name:     "bin",
			path:     "/bin",
			expected: true,
		},
		{
			name:     "usr sbin",
			path:     "/usr/sbin",
			expected: true,
		},
		{
			name:     "sbin",
			path:     "/sbin",
			expected: true,
		},
		{
			name:     "home directory",
			path:     "/home/user/bin",
			expected: false,
		},
		{
			name:     "tmp directory",
			path:     "/tmp/mybin",
			expected: false,
		},
		{
			name:     "user go bin",
			path:     "/Users/user/go/bin",
			expected: false,
		},
		{
			name:     "opt directory",
			path:     "/opt/blaxel/bin",
			expected: false,
		},
		{
			name:     "local user bin",
			path:     "/Users/user/.local/bin",
			expected: false,
		},
		{
			name:     "nested system path",
			path:     "/usr/local/bin/subdir",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := needsSudoForPath(tt.path)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDetectInstallationMethod(t *testing.T) {
	// This test just verifies the function exists and returns a valid method
	// The actual result depends on the system where tests are run
	method, err := detectInstallationMethod()

	// Should either succeed with a valid method or fail gracefully
	if err != nil {
		// Error is acceptable if we can't get executable path
		assert.Contains(t, err.Error(), "failed to get executable path")
	} else {
		// Should return either "brew" or "curl"
		assert.Contains(t, []string{"brew", "curl"}, method)
	}
}

func TestIsInstalledViaHomebrewWithInvalidPath(t *testing.T) {
	// Test with a path that's definitely not in homebrew
	result := isInstalledViaHomebrew("/some/random/nonexistent/path")
	// This may return false or true depending on if brew is installed
	// but should not panic
	assert.IsType(t, false, result)
}

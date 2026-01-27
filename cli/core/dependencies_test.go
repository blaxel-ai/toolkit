package core

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsCommandAvailableInDependencies(t *testing.T) {
	t.Run("common commands should be available", func(t *testing.T) {
		// 'echo' is universally available on Unix-like systems
		assert.True(t, isCommandAvailable("echo"))
		assert.True(t, isCommandAvailable("sh"))
	})

	t.Run("non-existent command should not be available", func(t *testing.T) {
		assert.False(t, isCommandAvailable("definitely_not_a_real_command_xyz"))
	})
}

// Note: installPythonDependencies and installTypescriptDependencies
// are integration-level tests that would require actual package managers
// to be installed and would modify the file system.
// These are better suited for integration tests.
